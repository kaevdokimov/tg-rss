package config

import (
	"log"
	"os"
	"strconv"
)

type DBConfig struct {
	DBHost string
	DBPort int
	DBUser string
	DBPass string
	DBName string
}

type TgBotConfig struct {
	ApiKey                 string
	TZ                     string
	Timeout                int
	ContentScraperInterval int // интервал в минутах
	ContentScraperBatch    int // размер батча
}

type KafkaConfig struct {
	Brokers     []string
	NewsTopic   string
	NotifyTopic string
}

func LoadDBConfig() *DBConfig {
	port, err := strconv.Atoi(getEnv("POSTGRES_PORT", "5432"))
	if err != nil {
		log.Fatalf("Некорректный порт БД: %v", err)
	}
	return &DBConfig{
		DBHost: getEnv("POSTGRES_HOST", "db"),
		DBPort: port,
		DBUser: getEnv("POSTGRES_USER", "postgres"),
		DBPass: getEnv("POSTGRES_PASSWORD", "password"),
		DBName: getEnv("POSTGRES_DB", "news_bot"),
	}
}

func LoadTgBotConfig() *TgBotConfig {
	TelegramApiKey := getEnv("TELEGRAM_API_KEY", "")
	if TelegramApiKey == "" {
		log.Fatalf("Некорректный ключ телеграм бота")
	}
	Timeout, err := strconv.Atoi(getEnv("TIMEOUT", "60"))
	if err != nil {
		log.Fatalf("Некорректное значение таймаута: %v", err)
	}
	ContentScraperInterval, err := strconv.Atoi(getEnv("CONTENT_SCRAPER_INTERVAL", "2"))
	if err != nil {
		log.Fatalf("Некорректное значение интервала парсера контента: %v", err)
	}
	ContentScraperBatch, err := strconv.Atoi(getEnv("CONTENT_SCRAPER_BATCH", "20"))
	if err != nil {
		log.Fatalf("Некорректное значение размера батча парсера контента: %v", err)
	}
	return &TgBotConfig{
		ApiKey:                 TelegramApiKey,
		TZ:                     getEnv("TZ", "Europe/Moscow"),
		Timeout:                Timeout,
		ContentScraperInterval: ContentScraperInterval,
		ContentScraperBatch:    ContentScraperBatch,
	}
}

func LoadKafkaConfig() *KafkaConfig {
	brokers := getEnv("KAFKA_BROKERS", "kafka:29092")
	log.Printf("Kafka brokers: %s", brokers)
	return &KafkaConfig{
		Brokers:     []string{brokers},
		NewsTopic:   getEnv("KAFKA_NEWS_TOPIC", "news-items"),
		NotifyTopic: getEnv("KAFKA_NOTIFY_TOPIC", "news-notifications"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
