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
	-- Создание типа ENUM
	DO $$ BEGIN
		CREATE TYPE status_enum AS ENUM ('active', 'inactive', 'archived');
	EXCEPTION
		WHEN duplicate_object THEN NULL;
	END $$;

	-- Таблица источников
	CREATE TABLE IF NOT EXISTS sources (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		url VARCHAR(1024) NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		status status_enum NOT NULL DEFAULT 'active'
	);
	-- Таблица новостей
	CREATE TABLE IF NOT EXISTS news (
		id SERIAL PRIMARY KEY,
		source_id INTEGER NOT NULL REFERENCES sources(id),
		title VARCHAR(1024) NOT NULL,
		description TEXT NOT NULL,
		link VARCHAR(1024) NOT NULL UNIQUE,
		published_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		tvs tsvector NULL GENERATED ALWAYS AS (
			to_tsvector('russian', title || ' ' || description)
		) STORED
	);
	-- Таблица пользователей
	CREATE TABLE IF NOT EXISTS users (
		chat_id BIGINT PRIMARY KEY,
		username VARCHAR(100) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	-- Таблица подписок
	CREATE TABLE IF NOT EXISTS subscriptions (
		chat_id BIGINT NOT NULL REFERENCES users(chat_id) ON DELETE CASCADE,
		source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (chat_id, source_id)
	);
	-- Таблица отправленных сообщений
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		chat_id BIGINT NOT NULL REFERENCES users(chat_id) ON DELETE CASCADE,
		news_id BIGINT NOT NULL REFERENCES news(id) ON DELETE CASCADE,
		sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Индексы
	CREATE UNIQUE INDEX idx_sources_url ON sources (url);
	CREATE INDEX idx_news_published_at ON news (published_at DESC);
	CREATE INDEX idx_news_tsvector ON news USING GIN (tvs);

	CREATE OR REPLACE FUNCTION lowercase_url()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.url = LOWER(NEW.url);
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;

	CREATE TRIGGER trg_lowercase_url
	BEFORE INSERT OR UPDATE ON sources
	FOR EACH ROW EXECUTE FUNCTION lowercase_url();
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка при создании схемы БД: %v", err)
	}
	log.Println("Схема базы данных инициализирована")
}

// SaveUser сохраняет нового пользователя в БД
func SaveUser(db *sql.DB, user User) (int64, error) {
	query := `INSERT INTO users (chat_id, username) VALUES ($1, $2)
	RETURNING chat_id`
	var insertedId int64
	err := db.QueryRow(query, user.ChatId, user.Username).Scan(&insertedId)
	if err != nil {
		// Ошибка может быть вызвана тем, что пользователь уже существует
		if err == sql.ErrNoRows {
			return 0, nil // Конфликт: пользователь не был добавлен, но это не ошибка
		}
		return 0, fmt.Errorf("Ошибка при добавлении пользователя: %w", err)
	}

	return insertedId, nil
}

// SaveSource сохраняет новый источник в БД
func SaveSource(db *sql.DB, source Source) error {
	query := `INSERT INTO sources (name, url) VALUES ($1, $2)`
	_, err := db.Exec(query, source.Name, source.Url)
	if err != nil {
		log.Printf("SaveSource сохраняет новый источник в БД: %s, %s\n%v", source.Name, source.Url, err)
	}
	return err
}

// SaveSubscription сохраняет новую подписку в БД
func SaveSubscription(db *sql.DB, subscription Subscription) error {
	query := `INSERT INTO subscriptions (chat_id, source_id) VALUES ($1, $2)`
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

func FindActiveSources(db *sql.DB) ([]Source, error) {
	query := `SELECT id, name, url, status FROM sources WHERE status = $1`
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
	query := `SELECT id, name, url, status FROM sources WHERE status = $1 and id = $2`
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
	query := `SELECT id, name, url, status FROM sources WHERE status = $1 and url = $2`
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

// GetSubscriptions: получить подписков на источник
func GetSubscriptions(db *sql.DB, sourceId int64) ([]Subscription, error) {
	query := `SELECT chat_id, source_id, created_at FROM subscriptions WHERE source_id = $1`
	rows, err := db.Query(query, sourceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []Subscription
	for rows.Next() {
		var item Subscription
		if err := rows.Scan(&item.ChatId, &item.SourceId, &item.CreatedAt); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, item)
	}

	return subscriptions, nil
}

// IsUserSubscribed: проверить, подписан ли пользователь на источник
func IsUserSubscribed(db *sql.DB, chatId int64, sourceId int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE chat_id = $1 AND source_id = $2)`
	var exists bool
	err := db.QueryRow(query, chatId, sourceId).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// GetUserSubscriptionsWithDetails: получить подписки пользователя с деталями
func GetUserSubscriptionsWithDetails(db *sql.DB, chatId int64) ([]Subscription, error) {
	query := `SELECT chat_id, source_id, created_at FROM subscriptions WHERE chat_id = $1`
	rows, err := db.Query(query, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []Subscription
	for rows.Next() {
		var item Subscription
		if err := rows.Scan(&item.ChatId, &item.SourceId, &item.CreatedAt); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, item)
	}

	return subscriptions, nil
}
