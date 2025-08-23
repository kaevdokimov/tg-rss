package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"tg-rss/bot"
	"tg-rss/config"
	"tg-rss/db"
	"tg-rss/redpanda"
	"time"
)

func main() {
	// Настройки
	cfgDB := config.LoadDBConfig()
	cfgTgBot := config.LoadTgBotConfig()
	cfgRedpanda := config.LoadRedpandaConfig()

	// Инициализация базы данных
	dbConn, err := db.Connect(cfgDB)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer dbConn.Close()

	db.InitSchema(dbConn)

	// Обновляем названия существующих источников
	err = db.UpdateSourceNames(dbConn)
	if err != nil {
		log.Printf("Предупреждение: не удалось обновить названия источников: %v", err)
	}

	// Инициализация Redpanda producer с retry
	var redpandaProducer *redpanda.Producer
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		redpandaProducer, err = redpanda.NewProducer(cfgRedpanda)
		if err != nil {
			log.Printf("Ошибка создания Redpanda producer (попытка %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(10 * time.Second)
				continue
			}
			log.Fatalf("Не удалось создать Redpanda producer после %d попыток", maxRetries)
		}
		break
	}
	defer redpandaProducer.Close()

	// Инициализация Redpanda consumer с retry
	var redpandaConsumer *redpanda.Consumer
	for i := 0; i < maxRetries; i++ {
		redpandaConsumer, err = redpanda.NewConsumer(cfgRedpanda)
		if err != nil {
			log.Printf("Ошибка создания Redpanda consumer (попытка %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(10 * time.Second)
				continue
			}
			log.Fatalf("Не удалось создать Redpanda consumer после %d попыток", maxRetries)
		}
		break
	}
	defer redpandaConsumer.Close()

	// Запуск бота с Redpanda
	bot.StartBotWithRedpanda(cfgTgBot, dbConn, redpandaProducer, redpandaConsumer)

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Получен сигнал завершения, закрываем приложение...")
}
