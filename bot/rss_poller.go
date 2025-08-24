package bot

import (
	"database/sql"
	"log"
	"time"

	"tg-rss/db"
	"tg-rss/kafka"
	"tg-rss/rss"
)

// StartRSSPolling запускает регулярный опрос RSS-источников
func StartRSSPolling(dbConn *sql.DB, interval time.Duration, tz *time.Location, kafkaProducer *kafka.Producer) {
	for {
		sources, err := fetchSources(dbConn)
		if err != nil {
			log.Printf("Ошибка при получении источников: %v", err)
			time.Sleep(interval)
			continue
		}

		for _, source := range sources {
			sourceNewsList, _ := rss.ParseRSS(source.Url, tz)
			for _, item := range sourceNewsList {
				// Создаем объект новости для отправки в Kafka
				newsItem := kafka.NewsItem{
					SourceID:    source.Id,
					SourceName:  source.Name,
					Title:       item.Title,
					Description: item.Description,
					Link:        item.Link,
					PublishedAt: item.PublishedAt.Format("2006-01-02 15:04:05"),
				}

				// Отправляем новость в Kafka для обработки
				if err := kafkaProducer.SendNewsItem(newsItem); err != nil {
					log.Printf("Ошибка отправки новости в Kafka: %v", err)
					continue
				}

				log.Printf("Новость отправлена в очередь: %s", item.Title)
			}
		}

		time.Sleep(interval)
	}
}

// fetchSources получает список источников из БД
func fetchSources(dbConn *sql.DB) ([]db.Source, error) {
	rows, err := dbConn.Query("SELECT id, name, url FROM sources WHERE status = $1", db.Active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []db.Source
	for rows.Next() {
		var id int
		var name, url string
		if err := rows.Scan(&id, &name, &url); err != nil {
			return nil, err
		}
		sources = append(sources, db.Source{
			Id:   int64(id),
			Name: name,
			Url:  url,
		})
	}
	return sources, nil
}

// fetchUsers получает список пользователей из БД
// func fetchUsers(dbConn *sql.DB) ([]int64, error) {
// 	rows, err := dbConn.Query("SELECT chat_id FROM users")
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var users []int64
// 	for rows.Next() {
// 		var chatID int64
// 		if err := rows.Scan(&chatID); err != nil {
// 			return nil, err
// 		}
// 		users = append(users, chatID)
// 	}
// 	return users, nil
// }
