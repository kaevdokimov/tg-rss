package bot

import (
	"database/sql"
	"tg-rss/db"
	"tg-rss/kafka"
	"tg-rss/monitoring"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var newsLogger = monitoring.NewLogger("NewsProcessor")

// NewsProcessor обрабатывает новости из Kafka и записывает в БД
type NewsProcessor struct {
	db              *sql.DB
	bot              *tgbotapi.BotAPI
	globalRateLimiter *GlobalRateLimiter
}

// NewNewsProcessor создает новый обработчик новостей
func NewNewsProcessor(db *sql.DB, bot *tgbotapi.BotAPI) *NewsProcessor {
	// Глобальный rate limiter: минимум 50ms между сообщениями (20 сообщений/секунду)
	// Это дает запас от лимита Telegram в 30 сообщений/секунду
	globalLimiter := NewGlobalRateLimiter(50 * time.Millisecond)
	
	return &NewsProcessor{
		db:               db,
		bot:              bot,
		globalRateLimiter: globalLimiter,
	}
}

// ProcessNewsItem обрабатывает новость из Kafka
func (np *NewsProcessor) ProcessNewsItem(newsItem kafka.NewsItem) error {
	// Парсим время публикации
	publishedAt, err := time.Parse("2006-01-02 15:04:05", newsItem.PublishedAt)
	if err != nil {
		newsLogger.Warn("Ошибка парсинга времени: %v", err)
		publishedAt = time.Now()
	}

	// Сохранение новости в БД
	query := `INSERT INTO news (title, description, link, published_at, source_id) 
			  VALUES ($1, $2, $3, $4, $5) 
			  ON CONFLICT (link) DO UPDATE SET title = $1, description = $2, published_at = $4
			  RETURNING id`

	monitoring.IncrementDBQueries()
	var newsID int64
	err = np.db.QueryRow(query, newsItem.Title, newsItem.Description, newsItem.Link, publishedAt, newsItem.SourceID).Scan(&newsID)
	if err != nil {
		monitoring.IncrementDBQueriesErrors()
		return err
	}

	newsLogger.Debug("Новость сохранена в БД: ID=%d, Title=%s", newsID, newsItem.Title)

	// Проверяем, является ли новость новой (не старше 24 часов)
	// Это предотвращает отправку старых новостей при первом запуске или перезапуске
	if time.Since(publishedAt) > 24*time.Hour {
		newsLogger.Debug("Пропускаем старую новость (старше 24ч): %s от %v", newsItem.Title, publishedAt)
		return nil
	}

	// Получение списка пользователей, подписанных на источник
	monitoring.IncrementDBQueries()
	subscriptions, err := db.GetSubscriptions(np.db, newsItem.SourceID)
	if err != nil {
		monitoring.IncrementDBQueriesErrors()
		newsLogger.Error("Ошибка при получении подписок: %v", err)
		return err
	}

		// Отправка новости подписанным пользователям с проверкой на дубликаты
	for _, subscription := range subscriptions {
		// Проверяем, не отправляли ли уже эту новость пользователю
		sent, err := db.IsNewsSentToUser(np.db, subscription.ChatId, newsItem.SourceID, newsItem.Link)
		if err != nil {
			newsLogger.Error("Ошибка при проверке отправленной новости для пользователя %d: %v", subscription.ChatId, err)
			continue
		}
		if sent {
			newsLogger.Debug("Новость уже была отправлена пользователю %d: %s", subscription.ChatId, newsItem.Title)
			continue
		}

		// Проверяем глобальный rate limit перед отправкой
		if !np.globalRateLimiter.AllowGlobal() {
			// Если глобальный лимит превышен, пропускаем эту новость
			// Она будет обработана при следующем опросе RSS
			newsLogger.Debug("Глобальный rate limit, пропускаем отправку новости пользователю %d: %s", subscription.ChatId, newsItem.Title)
			continue
		}
		
		// Небольшая задержка между отправками для предотвращения превышения лимитов
		// Используем текущий интервал глобального rate limiter
		currentInterval := np.globalRateLimiter.GetMinInterval()
		if currentInterval > 0 {
			time.Sleep(currentInterval)
		}

		// Отправляем сообщение
		msg := tgbotapi.NewMessage(subscription.ChatId, formatNewsMessage(newsItem.Title, newsItem.Description, publishedAt, newsItem.SourceName))
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.DisableWebPagePreview = true
		msg.ReplyMarkup = createNewsKeyboard(newsItem.Link, newsID)

		if _, err := np.bot.Send(msg); err != nil {
			monitoring.IncrementTelegramMessagesErrors()
			
			// Улучшенная обработка ошибок
			if isRateLimitError(err) {
				retryAfter := extractRetryAfter(err)
				if retryAfter > 0 {
					// Если указано время ожидания, увеличиваем глобальный интервал
					// Для больших значений (более часа) используем более консервативный подход
					var newInterval time.Duration
					if retryAfter > 3600 {
						// Если более часа, устанавливаем интервал в 1 минуту
						// Это позволит постепенно отправлять новости, не блокируя полностью
						newInterval = 60 * time.Second
						newsLogger.Warn("Критический rate limit для пользователя %d (retry after %d сек = %.1f часов). Устанавливаем интервал 1 минута между сообщениями. Новость будет отправлена позже: %s", 
							subscription.ChatId, retryAfter, float64(retryAfter)/3600, newsItem.Title)
					} else if retryAfter > 300 {
						// Если более 5 минут, устанавливаем интервал в 30 секунд
						newInterval = 30 * time.Second
						newsLogger.Warn("Высокий rate limit для пользователя %d (retry after %d сек = %.1f минут). Устанавливаем интервал 30 секунд. Новость будет отправлена позже: %s", 
							subscription.ChatId, retryAfter, float64(retryAfter)/60, newsItem.Title)
					} else {
						// Для меньших значений используем указанное время + запас
						newInterval = time.Duration(retryAfter+5) * time.Second
						if newInterval > 60*time.Second {
							newInterval = 60 * time.Second
						}
						newsLogger.Warn("Rate limit для пользователя %d (retry after %d сек), увеличиваем глобальный интервал до %v. Новость будет отправлена позже: %s", 
							subscription.ChatId, retryAfter, newInterval, newsItem.Title)
					}
					
					np.globalRateLimiter.SetMinInterval(newInterval)
				} else {
					// Если время не указано, увеличиваем до 5 секунд
					np.globalRateLimiter.SetMinInterval(5 * time.Second)
					newsLogger.Warn("Rate limit для пользователя %d (время не указано), увеличиваем глобальный интервал до 5 секунд. Новость будет отправлена позже: %s", 
						subscription.ChatId, newsItem.Title)
				}
				continue
			}

			// Для других ошибок логируем с понятным сообщением
			errorMsg := handleTelegramError(err)
			newsLogger.Error("Ошибка отправки новости пользователю %d: %v (сообщение: %s)", subscription.ChatId, err, errorMsg)
			continue
		}

		// Сохраняем информацию об отправке в таблицу messages
		tx, err := np.db.Begin()
		if err != nil {
			newsLogger.Error("Ошибка начала транзакции для сохранения сообщения: %v", err)
			continue
		}

		if err := db.SaveMessage(tx, subscription.ChatId, newsID); err != nil {
			tx.Rollback()
			newsLogger.Error("Ошибка сохранения сообщения: %v", err)
			continue
		}

		if err := tx.Commit(); err != nil {
			newsLogger.Error("Ошибка коммита транзакции: %v", err)
			continue
		}

		monitoring.IncrementTelegramMessagesSent()
		newsLogger.Debug("Новость отправлена пользователю %d: %s", subscription.ChatId, newsItem.Title)
		
		// После успешной отправки постепенно уменьшаем интервал, если он был увеличен
		// Это позволяет вернуться к нормальной скорости после снятия ограничения
		currentInterval = np.globalRateLimiter.GetMinInterval()
		if currentInterval > 50*time.Millisecond {
			// Уменьшаем на 10%, но не меньше базового значения (50ms)
			newInterval := currentInterval * 90 / 100
			if newInterval < 50*time.Millisecond {
				newInterval = 50 * time.Millisecond
			}
			np.globalRateLimiter.SetMinInterval(newInterval)
		}
	}

	return nil
}
