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
	ApiKey                  string
	TZ                      string
	Timeout                 int
	ContentScraperInterval  int // интервал в минутах
	ContentScraperBatch     int // размер батча
	ContentScraperConcurrent int // количество параллельных запросов
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	NewsChannel  string
	NotifyChannel string
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
	ContentScraperInterval, err := strconv.Atoi(getEnv("CONTENT_SCRAPER_INTERVAL", "1"))
	if err != nil {
		log.Fatalf("Некорректное значение интервала парсера контента: %v", err)
	}
	ContentScraperBatch, err := strconv.Atoi(getEnv("CONTENT_SCRAPER_BATCH", "50"))
	if err != nil {
		log.Fatalf("Некорректное значение размера батча парсера контента: %v", err)
	}
	ContentScraperConcurrent, err := strconv.Atoi(getEnv("CONTENT_SCRAPER_CONCURRENT", "3"))
	if err != nil {
		log.Fatalf("Некорректное значение количества параллельных запросов парсера контента: %v", err)
	}
	return &TgBotConfig{
		ApiKey:                  TelegramApiKey,
		TZ:                      getEnv("TZ", "Europe/Moscow"),
		Timeout:                 Timeout,
		ContentScraperInterval:  ContentScraperInterval,
		ContentScraperBatch:     ContentScraperBatch,
		ContentScraperConcurrent: ContentScraperConcurrent,
	}
}

func LoadRedisConfig() *RedisConfig {
	addr := getEnv("REDIS_ADDR", "redis:6379")
	db, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	log.Printf("Redis addr: %s", addr)
	return &RedisConfig{
		Addr:          addr,
		Password:      getEnv("REDIS_PASSWORD", ""),
		DB:            db,
		NewsChannel:   getEnv("REDIS_NEWS_CHANNEL", "news-items"),
		NotifyChannel: getEnv("REDIS_NOTIFY_CHANNEL", "news-notifications"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
