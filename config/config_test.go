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
			os.Setenv("POSTGRES_HOST", originalHost)
		} else {
			os.Unsetenv("POSTGRES_HOST")
		}
		if originalPort != "" {
			os.Setenv("POSTGRES_PORT", originalPort)
		} else {
			os.Unsetenv("POSTGRES_PORT")
		}
		if originalUser != "" {
			os.Setenv("POSTGRES_USER", originalUser)
		} else {
			os.Unsetenv("POSTGRES_USER")
		}
		if originalPass != "" {
			os.Setenv("POSTGRES_PASSWORD", originalPass)
		} else {
			os.Unsetenv("POSTGRES_PASSWORD")
		}
		if originalDB != "" {
			os.Setenv("POSTGRES_DB", originalDB)
		} else {
			os.Unsetenv("POSTGRES_DB")
		}
	}()

	// Тест с дефолтными значениями
	os.Unsetenv("POSTGRES_HOST")
	os.Unsetenv("POSTGRES_PORT")
	os.Unsetenv("POSTGRES_USER")
	os.Unsetenv("POSTGRES_PASSWORD")
	os.Unsetenv("POSTGRES_DB")

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
	os.Setenv("POSTGRES_HOST", "test-host")
	os.Setenv("POSTGRES_PORT", "3306")
	os.Setenv("POSTGRES_USER", "test-user")
	os.Setenv("POSTGRES_PASSWORD", "test-pass")
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

func TestLoadKafkaConfig(t *testing.T) {
	originalBrokers := os.Getenv("KAFKA_BROKERS")
	originalNewsTopic := os.Getenv("KAFKA_NEWS_TOPIC")
	originalNotifyTopic := os.Getenv("KAFKA_NOTIFY_TOPIC")

	defer func() {
		if originalBrokers != "" {
			os.Setenv("KAFKA_BROKERS", originalBrokers)
		} else {
			os.Unsetenv("KAFKA_BROKERS")
		}
		if originalNewsTopic != "" {
			os.Setenv("KAFKA_NEWS_TOPIC", originalNewsTopic)
		} else {
			os.Unsetenv("KAFKA_NEWS_TOPIC")
		}
		if originalNotifyTopic != "" {
			os.Setenv("KAFKA_NOTIFY_TOPIC", originalNotifyTopic)
		} else {
			os.Unsetenv("KAFKA_NOTIFY_TOPIC")
		}
	}()

	// Тест с дефолтными значениями
	os.Unsetenv("KAFKA_BROKERS")
	os.Unsetenv("KAFKA_NEWS_TOPIC")
	os.Unsetenv("KAFKA_NOTIFY_TOPIC")

	cfg := LoadKafkaConfig()
	if len(cfg.Brokers) == 0 || cfg.Brokers[0] != "kafka:29092" {
		t.Errorf("Ожидался брокер по умолчанию 'kafka:29092', получено '%v'", cfg.Brokers)
	}
	if cfg.NewsTopic != "news-items" {
		t.Errorf("Ожидался NewsTopic 'news-items', получено '%s'", cfg.NewsTopic)
	}
	if cfg.NotifyTopic != "news-notifications" {
		t.Errorf("Ожидался NotifyTopic 'news-notifications', получено '%s'", cfg.NotifyTopic)
	}
}
