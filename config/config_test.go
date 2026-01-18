package config

import (
	"os"
	"testing"
)

func TestLoadDBConfig(t *testing.T) {
	// Сохраняем оригинальные значения
	originalHost := os.Getenv("POSTGRES_HOST")
	originalPort := os.Getenv("POSTGRES_PORT")
	originalUser := os.Getenv("POSTGRES_USER")
	originalPass := os.Getenv("POSTGRES_PASSWORD")
	originalDB := os.Getenv("POSTGRES_DB")

	// Восстанавливаем после теста
	defer func() {
		if originalHost != "" {
			_ = os.Setenv("POSTGRES_HOST", originalHost)
		} else {
			_ = os.Unsetenv("POSTGRES_HOST")
		}
		if originalPort != "" {
			_ = os.Setenv("POSTGRES_PORT", originalPort)
		} else {
			_ = os.Unsetenv("POSTGRES_PORT")
		}
		if originalUser != "" {
			_ = os.Setenv("POSTGRES_USER", originalUser)
		} else {
			_ = os.Unsetenv("POSTGRES_USER")
		}
		if originalPass != "" {
			_ = os.Setenv("POSTGRES_PASSWORD", originalPass)
		} else {
			_ = os.Unsetenv("POSTGRES_PASSWORD")
		}
		if originalDB != "" {
			_ = os.Setenv("POSTGRES_DB", originalDB)
		} else {
			_ = os.Unsetenv("POSTGRES_DB")
		}
	}()

	// Тест с дефолтными значениями
	_ = os.Unsetenv("POSTGRES_HOST")
	_ = os.Unsetenv("POSTGRES_PORT")
	_ = os.Unsetenv("POSTGRES_USER")
	_ = os.Unsetenv("POSTGRES_PASSWORD")
	_ = os.Unsetenv("POSTGRES_DB")

	cfg := LoadDBConfig()
	if cfg.DBHost != "db" {
		t.Errorf("Ожидался DBHost 'db', получено '%s'", cfg.DBHost)
	}
	if cfg.DBPort != 5432 {
		t.Errorf("Ожидался DBPort 5432, получено %d", cfg.DBPort)
	}
	if cfg.DBUser != "postgres" {
		t.Errorf("Ожидался DBUser 'postgres', получено '%s'", cfg.DBUser)
	}
	if cfg.DBName != "news_bot" {
		t.Errorf("Ожидался DBName 'news_bot', получено '%s'", cfg.DBName)
	}

	// Тест с кастомными значениями
	_ = os.Setenv("POSTGRES_HOST", "test-host")
	_ = os.Setenv("POSTGRES_PORT", "3306")
	_ = os.Setenv("POSTGRES_USER", "test-user")
	_ = os.Setenv("POSTGRES_PASSWORD", "test-pass")
	os.Setenv("POSTGRES_DB", "test-db")

	cfg = LoadDBConfig()
	if cfg.DBHost != "test-host" {
		t.Errorf("Ожидался DBHost 'test-host', получено '%s'", cfg.DBHost)
	}
	if cfg.DBPort != 3306 {
		t.Errorf("Ожидался DBPort 3306, получено %d", cfg.DBPort)
	}
	if cfg.DBUser != "test-user" {
		t.Errorf("Ожидался DBUser 'test-user', получено '%s'", cfg.DBUser)
	}
	if cfg.DBPass != "test-pass" {
		t.Errorf("Ожидался DBPass 'test-pass', получено '%s'", cfg.DBPass)
	}
	if cfg.DBName != "test-db" {
		t.Errorf("Ожидался DBName 'test-db', получено '%s'", cfg.DBName)
	}
}

func TestLoadRedisConfig(t *testing.T) {
	originalAddr := os.Getenv("REDIS_ADDR")
	originalPassword := os.Getenv("REDIS_PASSWORD")
	originalDB := os.Getenv("REDIS_DB")
	originalNewsChannel := os.Getenv("REDIS_NEWS_CHANNEL")
	originalNotifyChannel := os.Getenv("REDIS_NOTIFY_CHANNEL")

	defer func() {
		if originalAddr != "" {
			os.Setenv("REDIS_ADDR", originalAddr)
		} else {
			os.Unsetenv("REDIS_ADDR")
		}
		if originalPassword != "" {
			os.Setenv("REDIS_PASSWORD", originalPassword)
		} else {
			os.Unsetenv("REDIS_PASSWORD")
		}
		if originalDB != "" {
			os.Setenv("REDIS_DB", originalDB)
		} else {
			os.Unsetenv("REDIS_DB")
		}
		if originalNewsChannel != "" {
			os.Setenv("REDIS_NEWS_CHANNEL", originalNewsChannel)
		} else {
			os.Unsetenv("REDIS_NEWS_CHANNEL")
		}
		if originalNotifyChannel != "" {
			os.Setenv("REDIS_NOTIFY_CHANNEL", originalNotifyChannel)
		} else {
			os.Unsetenv("REDIS_NOTIFY_CHANNEL")
		}
	}()

	// Тест с дефолтными значениями
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("REDIS_PASSWORD")
	os.Unsetenv("REDIS_DB")
	os.Unsetenv("REDIS_NEWS_CHANNEL")
	os.Unsetenv("REDIS_NOTIFY_CHANNEL")

	cfg := LoadRedisConfig()
	if cfg.Addr != "redis:6379" {
		t.Errorf("Ожидался адрес по умолчанию 'redis:6379', получено '%s'", cfg.Addr)
	}
	if cfg.DB != 0 {
		t.Errorf("Ожидался DB по умолчанию 0, получено %d", cfg.DB)
	}
	if cfg.NewsChannel != "news-items" {
		t.Errorf("Ожидался NewsChannel 'news-items', получено '%s'", cfg.NewsChannel)
	}
	if cfg.NotifyChannel != "news-notifications" {
		t.Errorf("Ожидался NotifyChannel 'news-notifications', получено '%s'", cfg.NotifyChannel)
	}
}

func TestLoadTgBotConfig(t *testing.T) {
	// Сохраняем оригинальные значения
	originalApiKey := os.Getenv("TELEGRAM_API_KEY")
	originalTZ := os.Getenv("TZ")
	originalTimeout := os.Getenv("TIMEOUT")
	originalInterval := os.Getenv("CONTENT_SCRAPER_INTERVAL")
	originalBatch := os.Getenv("CONTENT_SCRAPER_BATCH")
	originalConcurrent := os.Getenv("CONTENT_SCRAPER_CONCURRENT")

	// Восстанавливаем после теста
	defer func() {
		if originalApiKey != "" {
			os.Setenv("TELEGRAM_API_KEY", originalApiKey)
		} else {
			os.Unsetenv("TELEGRAM_API_KEY")
		}
		if originalTZ != "" {
			os.Setenv("TZ", originalTZ)
		} else {
			os.Unsetenv("TZ")
		}
		if originalTimeout != "" {
			os.Setenv("TIMEOUT", originalTimeout)
		} else {
			os.Unsetenv("TIMEOUT")
		}
		if originalInterval != "" {
			os.Setenv("CONTENT_SCRAPER_INTERVAL", originalInterval)
		} else {
			os.Unsetenv("CONTENT_SCRAPER_INTERVAL")
		}
		if originalBatch != "" {
			os.Setenv("CONTENT_SCRAPER_BATCH", originalBatch)
		} else {
			os.Unsetenv("CONTENT_SCRAPER_BATCH")
		}
		if originalConcurrent != "" {
			os.Setenv("CONTENT_SCRAPER_CONCURRENT", originalConcurrent)
		} else {
			os.Unsetenv("CONTENT_SCRAPER_CONCURRENT")
		}
	}()

	// Тест с дефолтными значениями (кроме ApiKey, который обязателен)
	os.Setenv("TELEGRAM_API_KEY", "test-api-key")
	os.Unsetenv("TZ")
	os.Unsetenv("TIMEOUT")
	os.Unsetenv("CONTENT_SCRAPER_INTERVAL")
	os.Unsetenv("CONTENT_SCRAPER_BATCH")
	os.Unsetenv("CONTENT_SCRAPER_CONCURRENT")

	cfg := LoadTgBotConfig()
	if cfg.ApiKey != "test-api-key" {
		t.Errorf("Ожидался ApiKey 'test-api-key', получено '%s'", cfg.ApiKey)
	}
	if cfg.TZ != "Europe/Moscow" {
		t.Errorf("Ожидался TZ 'Europe/Moscow', получено '%s'", cfg.TZ)
	}
	if cfg.Timeout != 60 {
		t.Errorf("Ожидался Timeout 60, получено %d", cfg.Timeout)
	}
	if cfg.ContentScraperInterval != 1 {
		t.Errorf("Ожидался ContentScraperInterval 1, получено %d", cfg.ContentScraperInterval)
	}
	if cfg.ContentScraperBatch != 50 {
		t.Errorf("Ожидался ContentScraperBatch 50, получено %d", cfg.ContentScraperBatch)
	}
	if cfg.ContentScraperConcurrent != 3 {
		t.Errorf("Ожидался ContentScraperConcurrent 3, получено %d", cfg.ContentScraperConcurrent)
	}

	// Тест с кастомными значениями
	os.Setenv("TELEGRAM_API_KEY", "custom-api-key")
	os.Setenv("TZ", "UTC")
	os.Setenv("TIMEOUT", "120")
	os.Setenv("CONTENT_SCRAPER_INTERVAL", "5")
	os.Setenv("CONTENT_SCRAPER_BATCH", "30")
	os.Setenv("CONTENT_SCRAPER_CONCURRENT", "10")

	cfg = LoadTgBotConfig()
	if cfg.ApiKey != "custom-api-key" {
		t.Errorf("Ожидался ApiKey 'custom-api-key', получено '%s'", cfg.ApiKey)
	}
	if cfg.TZ != "UTC" {
		t.Errorf("Ожидался TZ 'UTC', получено '%s'", cfg.TZ)
	}
	if cfg.Timeout != 120 {
		t.Errorf("Ожидался Timeout 120, получено %d", cfg.Timeout)
	}
	if cfg.ContentScraperInterval != 5 {
		t.Errorf("Ожидался ContentScraperInterval 5, получено %d", cfg.ContentScraperInterval)
	}
	if cfg.ContentScraperBatch != 30 {
		t.Errorf("Ожидался ContentScraperBatch 30, получено %d", cfg.ContentScraperBatch)
	}
	if cfg.ContentScraperConcurrent != 10 {
		t.Errorf("Ожидался ContentScraperConcurrent 10, получено %d", cfg.ContentScraperConcurrent)
	}
}
