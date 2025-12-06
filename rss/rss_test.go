package rss

import (
	"testing"
	"time"
)

func TestParseRSS(t *testing.T) {
	// Тест с валидным RSS URL (используем публичный тестовый RSS)
	// Примечание: в реальных тестах лучше использовать mock или локальный сервер
	tz := time.UTC
	
	// Пропускаем тест, если нет интернета или RSS недоступен
	// В production лучше использовать mock
	t.Skip("Skipping integration test - requires network access")
	
	url := "https://lenta.ru/rss/google-newsstand/main/"
	newsList, err := ParseRSS(url, tz)
	
	if err != nil {
		t.Fatalf("Failed to parse RSS: %v", err)
	}
	
	if len(newsList) == 0 {
		t.Error("Expected at least one news item")
	}
	
	// Проверяем структуру первой новости
	if len(newsList) > 0 {
		news := newsList[0]
		if news.Title == "" {
			t.Error("Expected non-empty title")
		}
		if news.Link == "" {
			t.Error("Expected non-empty link")
		}
	}
}

func TestParseRSSInvalidURL(t *testing.T) {
	tz := time.UTC
	invalidURL := "https://invalid-url-that-does-not-exist-12345.com/rss"
	
	_, err := ParseRSS(invalidURL, tz)
	
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}
