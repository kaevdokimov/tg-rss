package bot

import (
	"database/sql"
	"log"
	"tg-rss/db"
	"tg-rss/redpanda"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MessageProcessor обрабатывает сообщения из Redpanda
type MessageProcessor struct {
	bot *tgbotapi.BotAPI
	db  *sql.DB
}

// NewMessageProcessor создает новый обработчик сообщений
func NewMessageProcessor(bot *tgbotapi.BotAPI, db *sql.DB) *MessageProcessor {
	return &MessageProcessor{
		bot: bot,
		db:  db,
	}
}

// ProcessNewsNotification обрабатывает уведомление о новости
func (mp *MessageProcessor) ProcessNewsNotification(notification redpanda.NewsNotification) error {
	// Проверяем, подписан ли пользователь на источник
	isSubscribed, err := db.IsUserSubscribed(mp.db, notification.ChatID, notification.SourceID)
	if err != nil {
		return err
	}

	if !isSubscribed {
		log.Printf("Пользователь %d не подписан на источник %d, пропускаем", notification.ChatID, notification.SourceID)
		return nil
	}

	// Парсим время публикации
	publishedAt, err := time.Parse("2006-01-02 15:04:05", notification.PublishedAt)
	if err != nil {
		log.Printf("Ошибка парсинга времени: %v", err)
		publishedAt = time.Now()
	}

	// Формируем сообщение
	msg := tgbotapi.NewMessage(notification.ChatID, formatNewsMessage(notification.Title, notification.Description, publishedAt, notification.SourceName))
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = createNewsKeyboard(notification.Link, notification.NewsID)

	// Отправляем сообщение
	if _, err := mp.bot.Send(msg); err != nil {
		return err
	}

	log.Printf("Новость отправлена пользователю %d: %s", notification.ChatID, notification.Title)
	return nil
}
