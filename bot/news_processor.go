package bot

import (
	"database/sql"
	"strings"
	"sync"
	"tg-rss/db"
	"tg-rss/kafka"
	"tg-rss/monitoring"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var newsLogger = monitoring.NewLogger("NewsProcessor")

// PendingNews представляет новость, ожидающую отправки
type PendingNews struct {
	NewsID      int64
	SourceID    int64
	SourceName  string
	SourceUrl   string
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
}

// NewsProcessor обрабатывает новости из Kafka и записывает в БД
type NewsProcessor struct {
	db                *sql.DB
	bot               *tgbotapi.BotAPI
	globalRateLimiter *GlobalRateLimiter
	pendingNews       map[int64][]PendingNews // очередь новостей по пользователям
	pendingMutex      sync.Mutex
	sendInterval      time.Duration
}

// NewNewsProcessor создает новый обработчик новостей
func NewNewsProcessor(db *sql.DB, bot *tgbotapi.BotAPI) *NewsProcessor {
	// Глобальный rate limiter: минимум 50ms между сообщениями (20 сообщений/секунду)
	// Это дает запас от лимита Telegram в 30 сообщений/секунду
	globalLimiter := NewGlobalRateLimiter(50 * time.Millisecond)
	
	np := &NewsProcessor{
		db:                db,
		bot:               bot,
		globalRateLimiter: globalLimiter,
		pendingNews:       make(map[int64][]PendingNews),
		sendInterval:      15 * time.Minute, // отправка раз в 15 минут
	}
	
	// Запускаем периодическую отправку накопленных новостей
	go np.startPeriodicSending()
	
	return np
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

	// Получаем URL источника для форматирования
	var sourceUrl string
	err = np.db.QueryRow("SELECT url FROM sources WHERE id = $1", newsItem.SourceID).Scan(&sourceUrl)
	if err != nil {
		newsLogger.Warn("Не удалось получить URL источника %d: %v", newsItem.SourceID, err)
		sourceUrl = ""
	}

	// Добавляем новость в очередь для каждого подписанного пользователя
	np.pendingMutex.Lock()
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

		// Добавляем новость в очередь пользователя
		pending := PendingNews{
			NewsID:      newsID,
			SourceID:    newsItem.SourceID,
			SourceName:  newsItem.SourceName,
			SourceUrl:   sourceUrl,
			Title:       newsItem.Title,
			Description: newsItem.Description,
			Link:        newsItem.Link,
			PublishedAt: publishedAt,
		}
		np.pendingNews[subscription.ChatId] = append(np.pendingNews[subscription.ChatId], pending)
		newsLogger.Debug("Новость добавлена в очередь для пользователя %d: %s (всего в очереди: %d)", 
			subscription.ChatId, newsItem.Title, len(np.pendingNews[subscription.ChatId]))
	}
	np.pendingMutex.Unlock()

	return nil
}

// startPeriodicSending запускает периодическую отправку накопленных новостей
func (np *NewsProcessor) startPeriodicSending() {
	newsLogger.Info("Запуск периодической отправки новостей с интервалом %v", np.sendInterval)
	
	// Первая отправка через 15 минут после старта
	// Это позволяет накопить новости за период
	time.Sleep(np.sendInterval)
	np.sendPendingNews()
	
	// Затем отправляем по расписанию каждые 15 минут
	ticker := time.NewTicker(np.sendInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		np.sendPendingNews()
	}
}

// sendPendingNews отправляет все накопленные новости пользователям списком
func (np *NewsProcessor) sendPendingNews() {
	np.pendingMutex.Lock()
	
	// Копируем очередь и очищаем оригинал
	pendingCopy := make(map[int64][]PendingNews)
	for chatId, news := range np.pendingNews {
		if len(news) > 0 {
			pendingCopy[chatId] = make([]PendingNews, len(news))
			copy(pendingCopy[chatId], news)
		}
	}
	np.pendingNews = make(map[int64][]PendingNews)
	
	np.pendingMutex.Unlock()

	if len(pendingCopy) == 0 {
		newsLogger.Debug("Нет накопленных новостей для отправки")
		return
	}

	newsLogger.Info("Начинаем отправку накопленных новостей для %d пользователей", len(pendingCopy))

	// Отправляем новости каждому пользователю
	for chatId, newsList := range pendingCopy {
		if len(newsList) == 0 {
			continue
		}

		// Формируем сообщение со списком новостей
		message := ""
		for i, news := range newsList {
			message += formatMessage(i+1, news.Title, news.Description, news.PublishedAt, news.SourceName, news.Link, news.SourceUrl)
		}
		// Убираем лишний перенос в конце
		message = strings.TrimRight(message, "\n")

		// Проверяем глобальный rate limit перед отправкой
		if !np.globalRateLimiter.AllowGlobal() {
			newsLogger.Debug("Глобальный rate limit, пропускаем отправку списка новостей пользователю %d", chatId)
			// Возвращаем новости обратно в очередь
			np.pendingMutex.Lock()
			np.pendingNews[chatId] = append(np.pendingNews[chatId], newsList...)
			np.pendingMutex.Unlock()
			continue
		}

		// Небольшая задержка между отправками
		currentInterval := np.globalRateLimiter.GetMinInterval()
		if currentInterval > 0 {
			time.Sleep(currentInterval)
		}

		// Отправляем сообщение
		msg := tgbotapi.NewMessage(chatId, message)
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.DisableWebPagePreview = true

		if _, err := np.bot.Send(msg); err != nil {
			monitoring.IncrementTelegramMessagesErrors()
			
			// Улучшенная обработка ошибок
			if isRateLimitError(err) {
				retryAfter := extractRetryAfter(err)
				if retryAfter > 0 {
					var newInterval time.Duration
					if retryAfter > 3600 {
						newInterval = 60 * time.Second
						newsLogger.Warn("Критический rate limit для пользователя %d (retry after %d сек). Устанавливаем интервал 1 минута", 
							chatId, retryAfter)
					} else if retryAfter > 300 {
						newInterval = 30 * time.Second
						newsLogger.Warn("Высокий rate limit для пользователя %d (retry after %d сек). Устанавливаем интервал 30 секунд", 
							chatId, retryAfter)
					} else {
						newInterval = time.Duration(retryAfter+5) * time.Second
						if newInterval > 60*time.Second {
							newInterval = 60 * time.Second
						}
						newsLogger.Warn("Rate limit для пользователя %d (retry after %d сек), увеличиваем глобальный интервал до %v", 
							chatId, retryAfter, newInterval)
					}
					np.globalRateLimiter.SetMinInterval(newInterval)
				} else {
					np.globalRateLimiter.SetMinInterval(5 * time.Second)
					newsLogger.Warn("Rate limit для пользователя %d (время не указано), увеличиваем глобальный интервал до 5 секунд", chatId)
				}
				// Возвращаем новости обратно в очередь
				np.pendingMutex.Lock()
				np.pendingNews[chatId] = append(np.pendingNews[chatId], newsList...)
				np.pendingMutex.Unlock()
				continue
			}

			errorMsg := handleTelegramError(err)
			newsLogger.Error("Ошибка отправки списка новостей пользователю %d: %v (сообщение: %s)", chatId, err, errorMsg)
			// Возвращаем новости обратно в очередь
			np.pendingMutex.Lock()
			np.pendingNews[chatId] = append(np.pendingNews[chatId], newsList...)
			np.pendingMutex.Unlock()
			continue
		}

		// Сохраняем информацию об отправке в таблицу messages
		tx, err := np.db.Begin()
		if err != nil {
			newsLogger.Error("Ошибка начала транзакции для сохранения сообщений: %v", err)
			continue
		}

		// Сохраняем все отправленные новости
		for _, news := range newsList {
			if err := db.SaveMessage(tx, chatId, news.NewsID); err != nil {
				newsLogger.Error("Ошибка сохранения сообщения для новости %d: %v", news.NewsID, err)
				// Продолжаем сохранять остальные
			}
		}

		if err := tx.Commit(); err != nil {
			newsLogger.Error("Ошибка коммита транзакции: %v", err)
			continue
		}

		monitoring.IncrementTelegramMessagesSent()
		newsLogger.Info("Список из %d новостей отправлен пользователю %d", len(newsList), chatId)
		
		// После успешной отправки постепенно уменьшаем интервал, если он был увеличен
		currentInterval = np.globalRateLimiter.GetMinInterval()
		if currentInterval > 50*time.Millisecond {
			newInterval := currentInterval * 90 / 100
			if newInterval < 50*time.Millisecond {
				newInterval = 50 * time.Millisecond
			}
			np.globalRateLimiter.SetMinInterval(newInterval)
		}
	}
}
