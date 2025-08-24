package bot

import (
	"database/sql"
	"log"
	"tg-rss/config"
	"tg-rss/kafka"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartBotWithKafka запускает бота с использованием Kafka для очередей сообщений
func StartBotWithKafka(cfgTgBot *config.TgBotConfig, dbConn *sql.DB, kafkaProducer *kafka.Producer, kafkaConsumer *kafka.Consumer) {
	interval := time.Duration(cfgTgBot.Timeout) * time.Second

	// Инициализация Telegram-бота
	bot, err := tgbotapi.NewBotAPI(cfgTgBot.ApiKey)
	if err != nil {
		log.Fatalf("Ошибка инициализации бота: %v", err)
	}
	log.Printf("Бот авторизован как %s", bot.Self.UserName)

	// Создание обработчиков
	newsProcessor := NewNewsProcessor(dbConn, bot)
	messageProcessor := NewMessageProcessor(bot, dbConn)

	// Запуск обработки команд
	go StartCommandHandler(bot, dbConn, cfgTgBot.Timeout)

	// Запуск опроса RSS-источников (отправка в Kafka)
	go StartRSSPolling(dbConn, interval, time.Local, kafkaProducer)

	// Запуск обработчика новостей из Kafka с retry логикой
	go func() {
		// Ждем немного, чтобы Kafka полностью запустилась
		time.Sleep(10 * time.Second)

		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			if err := kafkaConsumer.StartConsuming(func(data interface{}) error {
				// Определяем тип сообщения и обрабатываем соответственно
				if newsItem, ok := data.(kafka.NewsItem); ok {
					return newsProcessor.ProcessNewsItem(newsItem)
				}
				if notification, ok := data.(kafka.NewsNotification); ok {
					return messageProcessor.ProcessNewsNotification(notification)
				}
				return nil
			}); err != nil {
				log.Printf("Ошибка в обработчике Kafka (попытка %d/%d): %v", i+1, maxRetries, err)
				if i < maxRetries-1 {
					time.Sleep(10 * time.Second)
					continue
				}
			} else {
				break
			}
		}
	}()

	// Задержка для предотвращения выхода из программы
	select {}
}
