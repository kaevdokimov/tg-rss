package bot

import (
	"database/sql"
	"log"
	"tg-rss/db"
	"tg-rss/kafka"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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
		log.Printf("Ошибка парсинга времени: %v", err)
		publishedAt = time.Now()
	}

	// Сохранение новости в БД
	query := `INSERT INTO news (title, description, link, published_at, source_id) 
			  VALUES ($1, $2, $3, $4, $5) 
			  ON CONFLICT (link) DO UPDATE SET title = $1, description = $2, published_at = $4
			  RETURNING id`

	var newsID int64
	err = np.db.QueryRow(query, newsItem.Title, newsItem.Description, newsItem.Link, publishedAt, newsItem.SourceID).Scan(&newsID)
	if err != nil {
		return err
	}

	log.Printf("Новость сохранена в БД: ID=%d, Title=%s", newsID, newsItem.Title)

	// Получение списка пользователей, подписанных на источник
	subscriptions, err := db.GetSubscriptions(np.db, newsItem.SourceID)
	if err != nil {
		log.Printf("Ошибка при получении подписок: %v", err)
		return err
	}

	// Отправка новости подписанным пользователям
	for _, subscription := range subscriptions {
		msg := tgbotapi.NewMessage(subscription.ChatId, formatNewsMessage(newsItem.Title, newsItem.Description, publishedAt, newsItem.SourceName))
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.DisableWebPagePreview = true
		msg.ReplyMarkup = createNewsKeyboard(newsItem.Link, newsID)

		if _, err := np.bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки новости пользователю %d: %v", subscription.ChatId, err)
			continue
		}

		log.Printf("Новость отправлена пользователю %d: %s", subscription.ChatId, newsItem.Title)
	}

	return nil
}
