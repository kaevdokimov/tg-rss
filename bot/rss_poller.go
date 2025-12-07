package bot

import (
	"database/sql"
	"time"

	"tg-rss/db"
	"tg-rss/kafka"
	"tg-rss/monitoring"
	"tg-rss/rss"
)

var rssLogger = monitoring.NewLogger("RSS")

// StartRSSPolling запускает регулярный опрос RSS-источников
func StartRSSPolling(dbConn *sql.DB, interval time.Duration, tz *time.Location, kafkaProducer *kafka.Producer) {
	for {
		monitoring.IncrementRSSPolls()
		sources, err := fetchSources(dbConn)
		if err != nil {
			monitoring.IncrementRSSPollsErrors()
			rssLogger.Error("Ошибка при получении источников: %v", err)
			time.Sleep(interval)
			continue
		}

		for _, source := range sources {
			sourceNewsList, err := rss.ParseRSS(source.Url, tz)
			if err != nil {
				monitoring.IncrementRSSPollsErrors()
				rssLogger.Warn("Ошибка парсинга RSS для источника %s: %v", source.Name, err)
				continue
			}

			for _, item := range sourceNewsList {
				// Пропускаем старые новости (старше 24 часов)
				// Это предотвращает отправку всех старых новостей при первом запуске
				if time.Since(item.PublishedAt) > 24*time.Hour {
					rssLogger.Debug("Пропускаем старую новость (старше 24ч): %s от %v", item.Title, item.PublishedAt)
					continue
				}

				// Проверяем, есть ли уже такая новость в БД
				// Это предотвращает повторную обработку уже обработанных новостей
				var existingNewsID int64
				err = dbConn.QueryRow(`
					SELECT id FROM news 
					WHERE source_id = $1 AND link = $2
				`, source.Id, item.Link).Scan(&existingNewsID)

				if err == nil {
					// Новость уже есть в БД, пропускаем
					rssLogger.Debug("Новость уже есть в БД, пропускаем: %s", item.Title)
					continue
				} else if err != sql.ErrNoRows {
					// Если это не "нет строк", значит произошла реальная ошибка
					rssLogger.Warn("Ошибка при проверке существования новости: %v", err)
					// Продолжаем обработку, так как это может быть временная ошибка
				}

				monitoring.IncrementRSSItemsProcessed()
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
					monitoring.IncrementKafkaErrors()
					rssLogger.Error("Ошибка отправки новости в Kafka: %v", err)
					continue
				}

				monitoring.IncrementKafkaMessagesProduced()
				rssLogger.Debug("Новость отправлена в очередь: %s", item.Title)
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
