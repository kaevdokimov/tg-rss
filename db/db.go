package db

import (
	"database/sql"
	"fmt"
	"log"
	"tg-rss/config"
	"time"

	_ "github.com/lib/pq"
)

type Status string

const (
	Active   Status = "active"
	Inactive Status = "inactive"
	Archived Status = "archived"
)

type Source struct {
	Id     int64
	Name   string
	Url    string
	Status Status
}

type User struct {
	Id       int64
	Username string
	ChatId   int64
}

type News struct {
	Id          int64
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
}

type Subscription struct {
	ChatId    int64
	SourceId  int64
	CreatedAt time.Time
}

type Message struct {
	ChatId int64
	NewsId int64
}

func Connect(config *config.DBConfig) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPass, config.DBName,
	)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("Ошибка подключения к БД: %v", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("Ошибка соединения с БД: %v", err)
	}
	log.Println("Подключение к БД установлено")
	return db, nil
}

func InitSchema(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS sources (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		url VARCHAR(1024) NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(100) NOT NULL DEFAULT 'active'
	);
	CREATE TABLE IF NOT EXISTS news (
		id SERIAL PRIMARY KEY,
		source_id BIGINT NOT NULL REFERENCES sources(id),
		title VARCHAR(1024) NOT NULL,
		description TEXT NOT NULL,
		link VARCHAR(1024) NOT NULL UNIQUE,
		published_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		tvs tsvector NULL GENERATED ALWAYS AS (to_tsvector('russian', title || ' ' || description  || ' ' || link)) STORED
	);
	CREATE TABLE IF NOT EXISTS users (
		chat_id BIGINT PRIMARY KEY,
		username VARCHAR(100) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS subscriptions (
		chat_id BIGINT NOT NULL REFERENCES users(chat_id),
		source_id BIGINT NOT NULL REFERENCES sources(id),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (chat_id, source_id)
	);
	CREATE TABLE IF NOT EXISTS messages (
		chat_id BIGINT NOT NULL REFERENCES users(chat_id),
		news_id BIGINT NOT NULL REFERENCES news(id),
		sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (chat_id, news_id)
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка при создании схемы БД: %v", err)
	}
	log.Println("Схема базы данных инициализирована")
}

// SaveUser сохраняет нового пользователя в БД
func SaveUser(db *sql.DB, user User) (int64, error) {
	query := `INSERT INTO users (chat_id, username) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	result, err := db.Exec(query, user.ChatId, user.Username)
	if err != nil {
		log.Printf("SaveUser сохраняет нового пользователя в БД: %d, %s\n%v", user.ChatId, user.Username, err)
	}
	insertId, _ := result.LastInsertId()
	log.Printf("Добавлен новый пользователь с ID %d", insertId)
	return insertId, err
}

// SaveSource сохраняет новый источник в БД
func SaveSource(db *sql.DB, source Source) error {
	query := `INSERT INTO sources (name, url) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := db.Exec(query, source.Name, source.Url)
	if err != nil {
		log.Printf("SaveSource сохраняет новый источник в БД: %s, %s\n%v", source.Name, source.Url, err)
	}
	return err
}

// SaveSubscription сохраняет новую подписку в БД
func SaveSubscription(db *sql.DB, subscription Subscription) error {
	query := `INSERT INTO subscriptions (chat_id, source_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := db.Exec(query, subscription.ChatId, subscription.SourceId)
	if err != nil {
		log.Printf("SaveSubscription сохраняет новую подписку в БД: %d, %d\n%v", subscription.ChatId, subscription.SourceId, err)
	}
	return err
}

// DeleteSubscription удаляет подписку из БД
func DeleteSubscription(db *sql.DB, subscription Subscription) error {
	query := `DELETE FROM subscriptions WHERE chat_id = $1 AND source_id = $2`
	_, err := db.Exec(query, subscription.ChatId, subscription.SourceId)
	if err != nil {
		log.Printf("DeleteSubscription удаляет подписку из БД: %d, %d\n%v", subscription.ChatId, subscription.SourceId, err)
	}
	return err
}

func GetActiveSources(db *sql.DB) ([]Source, error) {
	query := `SELECT id, name, url, status FROM sources WHERE status = '$1' order by id`
	rows, err := db.Query(query, Active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var item Source
		if err := rows.Scan(&item.Id, &item.Name, &item.Url, &item.Status); err != nil {
			return nil, err
		}
		sources = append(sources, item)
	}

	return sources, nil
}

func FindActiveSourceById(db *sql.DB, id int64) (Source, error) {
	query := `SELECT id, name, url, status FROM sources WHERE status = '$1' and id = $2`
	var source Source
	err := db.QueryRow(query, Active, id).Scan(&source.Id, &source.Name, &source.Url, &source.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			// Если источник не найден
			return Source{}, fmt.Errorf("Источник с ID %d не найден", id)
		}
		// Другие ошибки
		return Source{}, err
	}
	return source, nil
}

func FindSourceActiveByUrl(db *sql.DB, url string) (Source, error) {
	query := `SELECT id, name, url, status FROM sources WHERE status = '$1' and url = $1`
	var source Source
	err := db.QueryRow(query, Active, url).Scan(&source.Id, &source.Name, &source.Url, &source.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			// Если источник не найден
			return Source{}, fmt.Errorf("Источник с URL %s не найден", url)
		}
		// Другие ошибки
		return Source{}, err
	}
	return source, nil
}

// GetLatestNews возвращает последние новости
func GetLatestNewsByUser(db *sql.DB, chatId int64, count int) ([]News, error) {
	query := `SELECT id, title, description, link, published_at FROM news n WHERE source_id IN (SELECT source_id FROM subscriptions WHERE chat_id = $1) ORDER BY n.published_at DESC LIMIT $1`
	rows, err := db.Query(query, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var news []News
	for rows.Next() {
		var item News
		if err := rows.Scan(&item.Id, &item.Title, &item.Description, &item.Link, &item.PublishedAt); err != nil {
			return nil, err
		}
		news = append(news, item)
	}

	return news, nil
}

func GetUserSubscriptions(db *sql.DB, chatId int64) ([]Source, error) {
	query := `SELECT id, name, url FROM sources WHERE id IN (SELECT source_id FROM subscriptions WHERE chat_id = $1)`
	rows, err := db.Query(query, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var item Source
		if err := rows.Scan(&item.Id, &item.Name, &item.Url); err != nil {
			return nil, err
		}
		sources = append(sources, item)
	}

	return sources, nil
}
