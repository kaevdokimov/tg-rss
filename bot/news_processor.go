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
	db  *sql.DB
	bot *tgbotapi.BotAPI
}

// NewNewsProcessor создает новый обработчик новостей
func NewNewsProcessor(db *sql.DB, bot *tgbotapi.BotAPI) *NewsProcessor {
	return &NewsProcessor{
		db:  db,
		bot: bot,
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

		// Отправляем сообщение
		msg := tgbotapi.NewMessage(subscription.ChatId, formatNewsMessage(newsItem.Title, newsItem.Description, publishedAt, newsItem.SourceName))
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.DisableWebPagePreview = true
		msg.ReplyMarkup = createNewsKeyboard(newsItem.Link, newsID)

		if _, err := np.bot.Send(msg); err != nil {
			monitoring.IncrementTelegramMessagesErrors()
			
			// Улучшенная обработка ошибок
			if isRateLimitError(err) {
				// При rate limiting не логируем как ошибку, просто пропускаем
				newsLogger.Warn("Rate limit для пользователя %d, новость будет отправлена позже: %s", subscription.ChatId, newsItem.Title)
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
	}

	return nil
}
