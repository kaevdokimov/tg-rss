package bot

import (
	"database/sql"
	"log"
	"time"

	"tg-rss/db"
	"tg-rss/rss"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartRSSPolling запускает регулярный опрос RSS-источников
func StartRSSPolling(dbConn *sql.DB, bot *tgbotapi.BotAPI, interval time.Duration, tz *time.Location) {
	for {
		sources, err := fetchSources(dbConn)
		if err != nil {
			log.Printf("Ошибка при получении источников: %v", err)
			time.Sleep(interval)
			continue
		}

		users, err := fetchUsers(dbConn)
		if err != nil {
			log.Printf("Ошибка при получении пользователей: %v", err)
			time.Sleep(interval)
			continue
		}

		for _, source := range sources {
			sourceNewsList, _ := rss.ParseRSS(source.Url, tz)

			for _, item := range sourceNewsList {

				// Сохранение новости в БД
				query := `INSERT INTO news (title, description, link, published_at) 
						  VALUES ($1, $2, $3, $4) 
						  ON CONFLICT DO NOTHING`
				_, err := dbConn.Exec(query, item.Title, item.Description, item.Link, item.PublishedAt)
				if err != nil {
					log.Printf("Ошибка при сохранении новости: %v", err)
					continue
				}

				// Отправка новости всем пользователям
				for _, userID := range users {
					msg := tgbotapi.NewMessage(userID, formatNewsMessage(item.Title, item.Link, item.Description, item.PublishedAt))
					msg.ParseMode = "Markdown"
					msg.DisableWebPagePreview = true
					bot.Send(msg)
				}
			}
		}

		time.Sleep(interval)
	}
}

// fetchSources получает список источников из БД
func fetchSources(dbConn *sql.DB) ([]db.Source, error) {
	rows, err := dbConn.Query("SELECT id, url FROM sources WHERE status = '$1'", db.Active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []db.Source
	for rows.Next() {
		var id int
		var url string
		if err := rows.Scan(&id, &url); err != nil {
			return nil, err
		}
		sources = append(sources, db.Source{
			Id:  int64(id),
			Url: url,
		})
	}
	return sources, nil
}

// fetchUsers получает список пользователей из БД
func fetchUsers(dbConn *sql.DB) ([]int64, error) {
	rows, err := dbConn.Query("SELECT chat_id FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		users = append(users, chatID)
	}
	return users, nil
}
