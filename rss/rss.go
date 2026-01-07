package rss

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type News struct {
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
}

// ParseRSSWithClient парсит RSS с использованием указанного HTTP клиента
func ParseRSSWithClient(url string, tz *time.Location, client *http.Client) ([]News, error) {
	fp := gofeed.NewParser()
	fp.Client = client

	// Создаем контекст с таймаутом для RSS парсинга
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		log.Printf("Ошибка при парсинге RSS-ленты %s: %v", url, err)
		return nil, err
	}

	var newsList []News
	for _, item := range feed.Items {
		publishedAt := time.Now().In(tz)
		if item.PublishedParsed != nil {
			publishedAt = item.PublishedParsed.In(tz)
		}
		newsList = append(newsList, News{
			Title:       item.Title,
			Description: item.Description,
			Link:        item.Link,
			PublishedAt: publishedAt,
		})
	}
	return newsList, nil
}

func ParseRSS(url string, tz *time.Location) ([]News, error) {
	// Используем стандартный HTTP клиент для обратной совместимости
	return ParseRSSWithClient(url, tz, http.DefaultClient)
}
