package bot

import (
	"database/sql"
	"log"
	"tg-rss/config"
	"tg-rss/redpanda"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func StartBot(cfgTgBot *config.TgBotConfig, dbConn *sql.DB) {
	// interval := time.Duration(cfgTgBot.Timeout) * time.Second
	// Инициализация Telegram-бота
	bot, err := tgbotapi.NewBotAPI(cfgTgBot.ApiKey)
	if err != nil {
		log.Fatalf("Ошибка инициализации бота: %v", err)
	}
	log.Printf("Бот авторизован как %s", bot.Self.UserName)

	// Запуск обработки команд
	go StartCommandHandler(bot, dbConn, cfgTgBot.Timeout)

	// Запуск опроса RSS-источников (без Redpanda - для обратной совместимости)
	// go StartRSSPolling(dbConn, bot, interval, time.Local)
	log.Println("Внимание: используется старая версия StartBot без Redpanda. Используйте StartBotWithRedpanda для отправки через очередь.")

	// Задержка для предотвращения выхода из программы
	select {}
}

func StartBotWithRedpanda(cfgTgBot *config.TgBotConfig, dbConn *sql.DB, redpandaProducer *redpanda.Producer, redpandaConsumer *redpanda.Consumer) {
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

	// Запуск опроса RSS-источников (только отправка в Redpanda)
	go StartRSSPolling(dbConn, interval, time.Local, redpandaProducer)

	// Запуск обработчика новостей из Redpanda с retry логикой
	go func() {
		// Ждем немного, чтобы Redpanda полностью запустилась
		time.Sleep(5 * time.Second)

		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			if err := redpandaConsumer.StartConsuming(func(data interface{}) error {
				// Определяем тип сообщения и обрабатываем соответственно
				if newsItem, ok := data.(redpanda.NewsItem); ok {
					return newsProcessor.ProcessNewsItem(newsItem)
				}
				if notification, ok := data.(redpanda.NewsNotification); ok {
					return messageProcessor.ProcessNewsNotification(notification)
				}
				return nil
			}); err != nil {
				log.Printf("Ошибка в обработчике Redpanda (попытка %d/%d): %v", i+1, maxRetries, err)
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
