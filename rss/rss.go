package rss

import (
	"log"
	"time"

	"github.com/mmcdole/gofeed"
)

type News struct {
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
}

func ParseRSS(url string, tz *time.Location) ([]News, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
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
