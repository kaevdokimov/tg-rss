package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
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

// NewsWithSource содержит новость с информацией об источнике
type NewsWithSource struct {
	News
	SourceName string
	SourceUrl  string
}

type Subscription struct {
	ChatId    int64
	SourceId  int64
	CreatedAt time.Time
}

type Message struct {
	ChatId    int64     `json:"chat_id"`
	NewsId    int64     `json:"news_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveNews сохраняет новость в БД и возвращает её ID
func SaveNews(db *sql.DB, sourceID int64, title, description, link string, publishedAt time.Time) (int64, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO news (source_id, title, description, link, published_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (link) DO UPDATE 
		SET updated_at = NOW()
		RETURNING id
	`, sourceID, title, description, link, publishedAt).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("ошибка при сохранении новости: %v", err)
	}

	return id, nil
}

// SaveMessage сохраняет информацию об отправленном уведомлении
func SaveMessage(tx *sql.Tx, chatID, newsID int64) error {
	_, err := tx.Exec(`
		INSERT INTO messages (chat_id, news_id)
		VALUES ($1, $2)
		ON CONFLICT (chat_id, news_id) DO NOTHING
	`, chatID, newsID)

	if err != nil {
		return fmt.Errorf("ошибка при сохранении сообщения: %v", err)
	}

	return nil
}

// SendNewsToSubscribers отправляет новость подписчикам, если они ещё не получали её
func SendNewsToSubscribers(db *sql.DB, chatIDs []int64, sourceID int64, title, description, link string, publishedAt time.Time) ([]int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("не удалось начать транзакцию: %v", err)
	}
	defer tx.Rollback()

	// Проверяем, есть ли уже такая новость в базе
	var existingNewsID int64
	err = tx.QueryRow(`
		SELECT id FROM news 
		WHERE source_id = $1 AND link = $2
	`, sourceID, link).Scan(&existingNewsID)

	var newsID int64
	if err == sql.ErrNoRows {
		// Новости нет в базе, сохраняем её
		newsID, err = SaveNews(db, sourceID, title, description, link, publishedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сохранении новости: %v", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("ошибка при проверке существующей новости: %v", err)
	} else {
		// Используем существующую новость
		newsID = existingNewsID
	}

	var sentTo []int64

	// Отправляем уведомления
	for _, chatID := range chatIDs {
		// Проверяем, не отправляли ли уже эту новость пользователю
		sent, err := IsNewsSentToUser(db, chatID, sourceID, link)
		if err != nil {
			log.Printf("Ошибка при проверке отправленной новости: %v", err)
			continue
		}
		if sent {
			log.Printf("Новость уже была отправлена пользователю %d", chatID)
			continue
		}

		// Сохраняем сообщение
		if err := SaveMessage(tx, chatID, newsID); err != nil {
			log.Printf("Ошибка при сохранении уведомления: %v", err)
			continue
		}

		sentTo = append(sentTo, chatID)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("ошибка при фиксации транзакции: %v", err)
	}

	return sentTo, nil
}

// IsNewsSentToUser проверяет, была ли уже отправлена новость пользователю
func IsNewsSentToUser(db *sql.DB, chatID, sourceID int64, link string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM messages m
		JOIN news n ON m.news_id = n.id
		WHERE m.chat_id = $1 AND n.source_id = $2 AND n.link = $3
	`, chatID, sourceID, link).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("ошибка при проверке отправленной новости: %v", err)
	}

	return count > 0, nil
}

func Connect(config *config.DBConfig) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPass, config.DBName,
	)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %v", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("ошибка соединения с БД: %v", err)
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
		sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (chat_id, news_id)
	);

	-- Индексы
	CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_url ON sources (url);
	CREATE INDEX IF NOT EXISTS idx_news_published_at ON news (published_at DESC);
	CREATE INDEX IF NOT EXISTS idx_news_tsvector ON news USING GIN (tvs);

	CREATE OR REPLACE FUNCTION lowercase_url()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.url = LOWER(NEW.url);
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;

	DROP TRIGGER IF EXISTS trg_lowercase_url ON sources;
	CREATE TRIGGER trg_lowercase_url
	BEFORE INSERT OR UPDATE ON sources
	FOR EACH ROW EXECUTE FUNCTION lowercase_url();

	INSERT INTO public.sources ("name",url,created_at,status) VALUES
	 ('Lenta.ru','https://lenta.ru/rss/google-newsstand/main/','2025-08-24 02:26:05.39313','active'::public."status_enum"),
	 ('Ria.ru','https://ria.ru/export/rss2/index.xml?page_type=google_newsstand','2025-08-24 02:26:07.792288','active'::public."status_enum"),
	 ('Rssexport.rbc.ru','https://rssexport.rbc.ru/rbcnews/news/30/full.rss','2025-08-24 02:26:09.780376','active'::public."status_enum"),
	 ('Tass.ru','https://tass.ru/rss/v2.xml','2025-08-24 02:26:11.897401','active'::public."status_enum"),
	 ('Government.ru','http://government.ru/all/rss/','2025-08-24 02:26:14.245899','active'::public."status_enum")
	ON CONFLICT (url) DO NOTHING;
	`
	_, err := db.Exec(query)
	if err != nil {
		// Игнорируем ошибки дублирования ключей при инициализации
		errMsg := err.Error()
		if !strings.Contains(errMsg, "duplicate key") && !strings.Contains(errMsg, "unique constraint") {
			log.Fatalf("ошибка при создании схемы БД: %v", err)
		}
		// Если это ошибка дублирования, просто логируем и продолжаем
		log.Printf("Предупреждение при инициализации схемы (возможно, данные уже существуют): %v", err)
	}
	log.Println("Схема базы данных инициализирована")
}

// SaveUser сохраняет нового пользователя в БД
func SaveUser(db *sql.DB, user User) (int64, error) {
	query := `INSERT INTO users (chat_id, username) VALUES ($1, $2)
	ON CONFLICT (chat_id) DO UPDATE SET username = $2
	RETURNING chat_id`
	var insertedId int64
	err := db.QueryRow(query, user.ChatId, user.Username).Scan(&insertedId)
	if err != nil {
		return 0, fmt.Errorf("ошибка при добавлении пользователя: %w", err)
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
	// Сначала проверяем, существует ли пользователь
	exists, err := UserExists(db, subscription.ChatId)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования пользователя: %w", err)
	}
	if !exists {
		return fmt.Errorf("пользователь с chat_id %d не существует", subscription.ChatId)
	}

	query := `INSERT INTO subscriptions (chat_id, source_id) VALUES ($1, $2)`
	_, err = db.Exec(query, subscription.ChatId, subscription.SourceId)
	if err != nil {
		log.Printf("SaveSubscription сохраняет новую подписку в БД: %d, %d\n%v", subscription.ChatId, subscription.SourceId, err)
	}
	return err
}

// UserExists проверяет, существует ли пользователь в БД
func UserExists(db *sql.DB, chatId int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE chat_id = $1)`
	var exists bool
	err := db.QueryRow(query, chatId).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
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
			return Source{}, fmt.Errorf("источник с ID %d не найден", id)
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
			return Source{}, fmt.Errorf("источник с URL %s не найден", url)
		}
		// Другие ошибки
		return Source{}, err
	}
	return source, nil
}

// GetLatestNews возвращает последние новости с информацией об источнике
func GetLatestNewsByUser(db *sql.DB, chatId int64, count int) ([]NewsWithSource, error) {
	query := `SELECT n.id, n.title, n.description, n.link, n.published_at, s.name, s.url 
			  FROM news n 
			  JOIN sources s ON n.source_id = s.id 
			  WHERE n.source_id IN (SELECT source_id FROM subscriptions WHERE chat_id = $1) 
			  ORDER BY n.published_at DESC LIMIT $2`
	rows, err := db.Query(query, chatId, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var news []NewsWithSource
	for rows.Next() {
		var item NewsWithSource
		if err := rows.Scan(&item.Id, &item.Title, &item.Description, &item.Link, &item.PublishedAt, &item.SourceName, &item.SourceUrl); err != nil {
			return nil, err
		}
		news = append(news, item)
	}

	return news, nil
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

// UpdateSourceNames обновляет названия источников, которые пустые или содержат только хост
func UpdateSourceNames(db *sql.DB) error {
	query := `
	UPDATE sources 
	SET name = CASE 
		WHEN name = '' OR name IS NULL THEN 
			CASE 
				WHEN url LIKE '%www.%' THEN 
					UPPER(SUBSTRING(REPLACE(url, 'https://', ''), 5, 1)) || 
					LOWER(SUBSTRING(REPLACE(url, 'https://', ''), 6))
				WHEN url LIKE '%http://%' THEN 
					UPPER(SUBSTRING(REPLACE(url, 'http://', ''), 1, 1)) || 
					LOWER(SUBSTRING(REPLACE(url, 'http://', ''), 2))
				ELSE 
					UPPER(SUBSTRING(REPLACE(REPLACE(url, 'https://', ''), 'http://', ''), 1, 1)) || 
					LOWER(SUBSTRING(REPLACE(REPLACE(url, 'https://', ''), 'http://', ''), 2))
			END
		ELSE name
	END
	WHERE name = '' OR name IS NULL OR name = url`

	_, err := db.Exec(query)
	return err
}

// AdminStats содержит статистику для администратора
type AdminStats struct {
	TotalNews      int
	NewsToday      int
	NewsYesterday  int
	TotalUsers     int
}

// GetAdminStats возвращает статистику для администратора
func GetAdminStats(db *sql.DB) (AdminStats, error) {
	var stats AdminStats
	
	// Получаем текущую дату и дату вчера
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.Add(-24 * time.Hour)
	todayEnd := todayStart.Add(24 * time.Hour)
	
	// Общее количество новостей
	err := db.QueryRow("SELECT COUNT(*) FROM news").Scan(&stats.TotalNews)
	if err != nil {
		return stats, fmt.Errorf("ошибка при получении общего количества новостей: %w", err)
	}
	
	// Новости за сегодня
	err = db.QueryRow(`
		SELECT COUNT(*) FROM news 
		WHERE published_at >= $1 AND published_at < $2
	`, todayStart, todayEnd).Scan(&stats.NewsToday)
	if err != nil {
		return stats, fmt.Errorf("ошибка при получении новостей за сегодня: %w", err)
	}
	
	// Новости за вчера
	err = db.QueryRow(`
		SELECT COUNT(*) FROM news 
		WHERE published_at >= $1 AND published_at < $2
	`, yesterdayStart, todayStart).Scan(&stats.NewsYesterday)
	if err != nil {
		return stats, fmt.Errorf("ошибка при получении новостей за вчера: %w", err)
	}
	
	// Общее количество пользователей
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	if err != nil {
		return stats, fmt.Errorf("ошибка при получении количества пользователей: %w", err)
	}
	
	return stats, nil
}
