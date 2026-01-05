package bot

import (
	"context"
	"database/sql"
	"log"
	"tg-rss/config"
	"tg-rss/redis"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartBotWithRedis запускает бота с использованием Redis для очередей сообщений
func StartBotWithRedis(ctx context.Context, cfgTgBot *config.TgBotConfig, dbConn *sql.DB, redisProducer *redis.Producer, redisConsumer *redis.Consumer) {
	interval := time.Duration(cfgTgBot.Timeout) * time.Second

	// Инициализация Telegram-бота
	var bot *tgbotapi.BotAPI
	log.Printf("🔍 Проверяем TELEGRAM_API_KEY: '%s'", cfgTgBot.ApiKey)

	if cfgTgBot.ApiKey == "" || cfgTgBot.ApiKey == "YOUR_TELEGRAM_BOT_TOKEN_HERE" {
		log.Printf("⚠️  TELEGRAM_API_KEY не задан или содержит placeholder - бот будет работать без Telegram функционала")
		// Создаем заглушку для бота
		bot = &tgbotapi.BotAPI{}
		bot.Self = tgbotapi.User{UserName: "MockBot"}
	} else {
		var err error
		bot, err = tgbotapi.NewBotAPI(cfgTgBot.ApiKey)
		if err != nil {
			log.Printf("⚠️  Ошибка инициализации Telegram бота: %v", err)
			log.Printf("🔄 Продолжаем работу без Telegram функционала")
			// Создаем заглушку для бота
			bot = &tgbotapi.BotAPI{}
			bot.Self = tgbotapi.User{UserName: "MockBot"}
		} else {
			log.Printf("Бот авторизован как %s", bot.Self.UserName)
		}
	}

	// Создание обработчиков
	newsProcessor := NewNewsProcessor(dbConn, bot)
	messageProcessor := NewMessageProcessor(bot, dbConn)

	// Запуск обработки команд
	go StartCommandHandler(bot, dbConn, cfgTgBot.Timeout)

	// Запуск опроса RSS-источников (отправка в Redis)
	log.Printf("Запуск RSS парсера с интервалом %v", interval)
	go StartRSSPolling(dbConn, interval, time.Local, redisProducer)

	// Запуск фонового парсера контента новостей
	// Парсит по батчу новостей с заданным интервалом
	scraperInterval := time.Duration(cfgTgBot.ContentScraperInterval) * time.Minute
	contentScraper := NewContentScraper(dbConn, scraperInterval, cfgTgBot.ContentScraperBatch, cfgTgBot.ContentScraperConcurrent)
	go contentScraper.Start()
	log.Printf("Запуск фонового парсера контента: интервал=%v, батч=%d, параллельно=%d", scraperInterval, cfgTgBot.ContentScraperBatch, cfgTgBot.ContentScraperConcurrent)

	// Запуск обработчика новостей из Redis с retry логикой
	go func() {
		// Ждем немного, чтобы Redis полностью запустился
		time.Sleep(5 * time.Second)

		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			if err := redisConsumer.SubscribeNews(func(newsItem redis.NewsItem) error {
				log.Printf("[Redis] Получена новость из Redis: %s (источник: %s)", newsItem.Title, newsItem.SourceName)
				if err := newsProcessor.ProcessNewsItem(newsItem); err != nil {
					log.Printf("[Redis] Ошибка обработки новости: %v", err)
					return err
				}
				return nil
			}); err != nil {
				log.Printf("Ошибка в обработчике Redis новостей (попытка %d/%d): %v", i+1, maxRetries, err)
				if i < maxRetries-1 {
					time.Sleep(5 * time.Second)
					continue
				}
			} else {
				log.Printf("Redis consumer успешно запущен")
				break
			}
		}
	}()

	// Запуск обработчика уведомлений из Redis
	go func() {
		time.Sleep(5 * time.Second) // Небольшая задержка

		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			if err := redisConsumer.SubscribeNotifications(func(notification redis.NewsNotification) error {
				log.Printf("[Redis] Получено уведомление из Redis для пользователя %d", notification.ChatID)
				return messageProcessor.ProcessNewsNotification(notification)
			}); err != nil {
				log.Printf("Ошибка в обработчике Redis уведомлений (попытка %d/%d): %v", i+1, maxRetries, err)
				if i < maxRetries-1 {
					time.Sleep(5 * time.Second)
					continue
				}
				break
			}
		}
	}()

	// Ожидание завершения контекста
	<-ctx.Done()
	log.Println("Бот завершает работу...")
}

