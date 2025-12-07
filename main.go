package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"tg-rss/bot"
	"tg-rss/config"
	"tg-rss/db"
	"tg-rss/kafka"
	"time"
)

func main() {
	// Настройки
	cfgDB := config.LoadDBConfig()
	cfgTgBot := config.LoadTgBotConfig()
	cfgKafka := config.LoadKafkaConfig()

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

	// Инициализация Kafka producer с retry
	var kafkaProducer *kafka.Producer
	maxRetries := 15
	for i := 0; i < maxRetries; i++ {
		kafkaProducer, err = kafka.NewProducer(cfgKafka)
		if err != nil {
			log.Printf("Ошибка создания Kafka producer (попытка %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(15 * time.Second)
				continue
			}
			log.Fatalf("Не удалось создать Kafka producer после %d попыток", maxRetries)
		}
		break
	}
	defer kafkaProducer.Close()

	// Инициализация Kafka consumer с retry
	var kafkaConsumer *kafka.Consumer
	for i := 0; i < maxRetries; i++ {
		kafkaConsumer, err = kafka.NewConsumer(cfgKafka)
		if err != nil {
			log.Printf("Ошибка создания Kafka consumer (попытка %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(15 * time.Second)
				continue
			}
			log.Fatalf("Не удалось создать Kafka consumer после %d попыток", maxRetries)
		}
		break
	}
	defer kafkaConsumer.Close()

	// Запуск бота с Kafka
	bot.StartBotWithKafka(cfgTgBot, dbConn, kafkaProducer, kafkaConsumer)

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Получен сигнал завершения, закрываем приложение...")
}
