package bot

import (
	"database/sql"
	"fmt"
	"log"
	"tg-rss/db"
	"tg-rss/kafka"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MessageProcessor обрабатывает сообщения из Kafka
type MessageProcessor struct {
	bot         *tgbotapi.BotAPI
	db          *sql.DB
	rateLimiter *RateLimiter
}

// NewMessageProcessor создает новый обработчик сообщений
func NewMessageProcessor(bot *tgbotapi.BotAPI, db *sql.DB) *MessageProcessor {
	// Ограничиваем отправку до 1 сообщения в 3 секунды для одного чата
	// Telegram имеет ограничение примерно 30 сообщений в секунду для ботов
	// Ограничение в 3 секунды даёт нам запас на случай пиковой нагрузки
	return &MessageProcessor{
		bot:         bot,
		db:          db,
		rateLimiter: NewRateLimiter(3 * time.Second),
	}
}

// ProcessNewsNotification обрабатывает уведомление о новости
func (mp *MessageProcessor) ProcessNewsNotification(notification kafka.NewsNotification) error {
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

	// Проверяем, что новость не старше 1 часа
	if time.Since(publishedAt) > time.Hour {
		log.Printf("Пропускаем устаревшую новость (старше 1ч): %s от %v", notification.Title, publishedAt)
		return nil
	}

	// Проверяем rate limit перед отправкой
	if !mp.rateLimiter.Allow(notification.ChatID) {
		log.Printf("Превышен лимит запросов для чата %d, пропускаем отправку: %s", notification.ChatID, notification.Title)
		return nil
	}

	// Формируем сообщение
	msg := tgbotapi.NewMessage(notification.ChatID, formatNewsMessage(notification.Title, notification.Description, publishedAt, notification.SourceName))
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = createNewsKeyboard(notification.Link, notification.NewsID)

	// Отправляем сообщение
	if _, err := mp.bot.Send(msg); err != nil {
		// Улучшенная обработка rate limiting
		if isRateLimitError(err) {
			retryAfter := extractRetryAfter(err)
			if retryAfter > 0 {
				// Увеличиваем период rate limiter на основе времени ожидания
				newPeriod := time.Duration(retryAfter+2) * time.Second
				if newPeriod > mp.rateLimiter.period {
					mp.rateLimiter.period = newPeriod
					log.Printf("Обнаружено ограничение скорости для чата %d, увеличиваем задержку до %v", notification.ChatID, newPeriod)
				}
			} else {
				// Если время не указано, увеличиваем до 5 секунд
				mp.rateLimiter.period = 5 * time.Second
				log.Printf("Обнаружено ограничение скорости для чата %d, увеличиваем задержку до 5 секунд", notification.ChatID)
			}
			// Не отправляем сообщение об ошибке пользователю при rate limiting
			// Новости будут отправлены позже автоматически
			return nil
		}

		// Для других ошибок логируем и возвращаем ошибку
		errorMsg := handleTelegramError(err)
		log.Printf("Ошибка отправки новости пользователю %d: %v (сообщение: %s)", notification.ChatID, err, errorMsg)
		return fmt.Errorf("ошибка отправки сообщения: %v", err)
	}

	log.Printf("Новость отправлена пользователю %d: %s", notification.ChatID, notification.Title)
	return nil
}
