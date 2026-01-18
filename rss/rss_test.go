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

	url := "https://lenta.ru/rss/google-newsstand/main/"
	newsList, err := ParseRSS(url, tz)

	if err != nil {
		t.Fatalf("Ошибка парсинга RSS: %v", err)
	}

	if len(newsList) == 0 {
		t.Error("Ожидалась хотя бы одна новость")
	}

	// Проверяем структуру первой новости
	if len(newsList) > 0 {
		news := newsList[0]
		if news.Title == "" {
			t.Error("Ожидался непустой заголовок")
		}
		if news.Link == "" {
			t.Error("Ожидалась непустая ссылка")
		}
	}
}

func TestParseRSSInvalidURL(t *testing.T) {
	tz := time.UTC
	invalidURL := "https://invalid-url-that-does-not-exist-12345.com/rss"

	_, err := ParseRSS(invalidURL, tz)

	if err == nil {
		t.Error("Ожидалась ошибка для невалидного URL")
	}
}
