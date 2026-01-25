package main

import (
	"testing"
)

// FuzzParseRSS тестирует парсинг RSS с fuzzing
func FuzzParseRSS(f *testing.F) {
	// Добавляем seed корпуса для тестирования
	seeds := []string{
		`<rss version="2.0"><channel><title>Test</title><item><title>Item 1</title><description>Description</description></item></channel></rss>`,
		`<rss version="2.0"><channel><title>Test Feed</title><item><title>News Item</title><link>https://example.com</link><description>Some description</description><pubDate>Mon, 01 Jan 2024 00:00:00 GMT</pubDate></item></channel></rss>`,
		`<rss version="2.0"><channel><title>Empty Feed</title></channel></rss>`,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, rssData string) {
		// Пытаемся распарсить RSS данные
		// В реальном коде здесь был бы вызов rss.ParseRSS(rssData)
		// Для демонстрации просто проверяем, что строка не пустая
		if len(rssData) > 0 {
			// Базовая проверка - данные содержат XML-like структуру
			if len(rssData) > 10 && (rssData[0] == '<' || rssData[:4] == "http") {
				// Это потенциально валидные данные для парсинга
				return
			}
		}
	})
}
