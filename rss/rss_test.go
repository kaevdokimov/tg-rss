package rss

import (
	"testing"
	"time"
)

func TestParseRSS(t *testing.T) {
	// Тест с валидным RSS URL (используем публичный тестовый RSS)
	// Примечание: в реальных тестах лучше использовать mock или локальный сервер

	// Пропускаем тест, если нет интернета или RSS недоступен
	// В production лучше использовать mock
	t.Skip("Пропуск интеграционного теста - требуется доступ к сети")
}

func TestParseRSSInvalidURL(t *testing.T) {
	tz := time.UTC
	invalidURL := "https://invalid-url-that-does-not-exist-12345.com/rss"

	_, err := ParseRSS(invalidURL, tz)

	if err == nil {
		t.Error("Ожидалась ошибка для невалидного URL")
	}
}
