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
	ApiKey  string
	TZ      string
	Timeout int
}

type RedpandaConfig struct {
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
		DBHost: getEnv("POSTGRES_HOST", ""),
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
	return &TgBotConfig{
		ApiKey:  TelegramApiKey,
		TZ:      getEnv("TZ", "Europe/Moscow"),
		Timeout: Timeout,
	}
}

func LoadRedpandaConfig() *RedpandaConfig {
	brokers := getEnv("REDPANDA_BROKERS", "localhost:9092")
	log.Printf("Redpanda brokers: %s", brokers)
	return &RedpandaConfig{
		Brokers:     []string{brokers},
		NewsTopic:   getEnv("REDPANDA_NEWS_TOPIC", "news-items"),
		NotifyTopic: getEnv("REDPANDA_NOTIFY_TOPIC", "news-notifications"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
