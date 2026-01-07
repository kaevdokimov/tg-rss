package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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
	Id              int64
	Title           string
	Description     string
	Link            string
	PublishedAt     time.Time
	FullText        sql.NullString
	Author          sql.NullString
	Category        sql.NullString
	Tags            []string
	Images          []string
	MetaKeywords    sql.NullString
	MetaDescription sql.NullString
	MetaData        sql.NullString // JSON
	ContentHTML     sql.NullString
	ScrapedAt       sql.NullTime
	ScrapeStatus    sql.NullString
	ScrapeError     sql.NullString
	UpdatedAt       time.Time
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
		sent, err := IsNewsSentToUser(db, chatID, newsID)
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
func IsNewsSentToUser(db *sql.DB, chatID, newsID int64) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM messages
		WHERE chat_id = $1 AND news_id = $2
	`, chatID, newsID).Scan(&count)

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
	log.Println("Начинаем инициализацию схемы базы данных...")

	// Оптимизация: Разделяем инициализацию на этапы для ускорения

	// Этап 1: Создание таблиц и типов
	log.Println("Этап 1: Создание таблиц и типов...")
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
		-- Дополнительные поля для полного контента и метаданных
		full_text TEXT,
		author VARCHAR(255),
		category VARCHAR(255),
		tags TEXT[], -- массив тегов
		images TEXT[], -- массив URL изображений
		meta_keywords TEXT,
		meta_description TEXT,
		meta_data JSONB, -- дополнительные метаданные в формате JSON
		content_html TEXT, -- HTML контента для анализа
		scraped_at TIMESTAMP, -- когда был выполнен парсинг страницы
		scrape_status VARCHAR(50) DEFAULT 'pending', -- pending, success, failed
		scrape_error TEXT, -- ошибка при парсинге, если была
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		tvs tsvector NULL GENERATED ALWAYS AS (
			to_tsvector('russian', COALESCE(title, '') || ' ' || COALESCE(description, '') || ' ' || COALESCE(full_text, ''))
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
	CREATE INDEX IF NOT EXISTS idx_news_scrape_status ON news (scrape_status);
	CREATE INDEX IF NOT EXISTS idx_news_id_desc ON news (id DESC);
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
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка при создании схемы БД (этап 1): %v", err)
	}
	log.Println("Этап 1 завершен: таблицы созданы")

	// Этап 2: Инициализация основных источников (асинхронно)
	log.Println("Этап 2: Инициализация источников новостей...")
	initSourcesAsync(db)
	log.Println("Инициализация схемы завершена")
}

func initSourcesAsync(db *sql.DB) {
	// Проверяем переменную окружения для отключения асинхронной инициализации
	if os.Getenv("DISABLE_ASYNC_SOURCES_INIT") == "true" {
		log.Println("Асинхронная инициализация источников отключена")
		return
	}

	// Оптимизация: Инициализируем источники асинхронно, чтобы не блокировать запуск приложения
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Паника при инициализации источников: %v", r)
			}
		}()

		// Группируем источники по категориям для поэтапной загрузки
		basicSources := `
		INSERT INTO public.sources ("name",url,created_at,status) VALUES
		('Lenta.ru','https://lenta.ru/rss/google-newsstand/main/','2025-08-24 02:26:05.39313','active'::public."status_enum"),
		('Ria.ru','https://ria.ru/export/rss2/index.xml?page_type=google_newsstand','2025-08-24 02:26:07.792288','active'::public."status_enum"),
		('ТАСС','https://tass.ru/rss/v2.xml?sections=MjU%3D','2025-08-24 02:26:11.897401','active'::public."status_enum"),
		('Ведомости','https://www.vedomosti.ru/rss/news','2025-08-24 02:26:09.000000','active'::public."status_enum"),
		('РБК','https://rssexport.rbc.ru/rbcnews/news/30/full.rss','2025-08-24 02:26:09.780376','active'::public."status_enum"),
		('Газета.Ru','https://www.gazeta.ru/export/rss/first.xml','2025-08-24 02:26:12.000000','active'::public."status_enum"),
		('Government.ru','http://government.ru/all/rss/','2025-08-24 02:26:14.245899','active'::public."status_enum")
		ON CONFLICT (url) DO NOTHING;
		`

		if _, err := db.Exec(basicSources); err != nil && !strings.Contains(err.Error(), "duplicate key") {
			log.Printf("Ошибка при инициализации основных источников: %v", err)
		} else {
			log.Println("Основные источники инициализированы")
		}

		// Небольшая задержка перед загрузкой дополнительных источников
		time.Sleep(2 * time.Second)

		// Дополнительные источники (тематические и региональные)
		extendedSources := `
		INSERT INTO public.sources ("name",url,created_at,status) VALUES
		('Lenta.ru - Новости','https://lenta.ru/rss/news','2025-08-24 02:26:05.39313','active'::public."status_enum"),
		('Lenta.ru - Топ 7','https://lenta.ru/rss/top7','2025-08-24 02:26:05.39313','active'::public."status_enum"),
		('Ria.ru - Все новости','https://ria.ru/export/rss2/index.xml','2025-08-24 02:26:07.792288','active'::public."status_enum"),
		('Ria.ru - Политика','https://ria.ru/export/rss2/politics/index.xml','2025-08-24 02:26:07.792288','active'::public."status_enum"),
		('Ria.ru - Экономика','https://ria.ru/export/rss2/economy/index.xml','2025-08-24 02:26:07.792288','active'::public."status_enum"),
		('Ria.ru - Общество','https://ria.ru/export/rss2/society/index.xml','2025-08-24 02:26:07.792288','active'::public."status_enum"),
		('Ria.ru - Спорт','https://sport.ria.ru/export/rss2/sport/index.xml','2025-08-24 02:26:07.792288','active'::public."status_enum"),
		('ТАСС - Все новости','https://tass.ru/rss/v2.xml','2025-08-24 02:26:11.897401','active'::public."status_enum"),
		('Интерфакс','https://www.interfax.ru/rss.asp','2025-08-24 02:26:15.000000','active'::public."status_enum"),
		('Аргументы и Факты','https://aif.ru/rss/all.php','2025-08-24 02:26:08.000000','active'::public."status_enum"),
		('Ведомости - Статьи','https://www.vedomosti.ru/rss/articles','2025-08-24 02:26:09.000000','active'::public."status_enum"),
		('РБК - Главная','https://rssexport.rbc.ru/rbcnews/news/30/full.rss','2025-08-24 02:26:09.780376','active'::public."status_enum"),
		('Коммерсант','https://www.kommersant.ru/RSS/news.xml','2025-08-24 02:26:16.000000','active'::public."status_enum"),
		('Полит.ру','https://polit.ru/rss/index.xml','2025-08-24 02:26:10.000000','active'::public."status_enum"),
		('News.mail.ru','https://news.mail.ru/rss/','2025-08-24 02:26:17.000000','active'::public."status_enum"),
		('NEWSru.com','https://newsru.com/plain/rss/all.xml','2025-08-24 02:26:18.000000','active'::public."status_enum"),
		('iXBT.com - Новости','https://www.ixbt.com/export/softnews.rss','2025-08-24 02:26:19.000000','active'::public."status_enum"),
		('iXBT.com - Статьи','https://www.ixbt.com/export/articles.rss','2025-08-24 02:26:19.000000','active'::public."status_enum"),
		('Sports.ru','https://www.sports.ru/sports_docs.xml','2025-08-24 02:26:20.000000','active'::public."status_enum"),
		('Travel.ru','https://www.travel.ru/news/feed/','2025-08-24 02:26:21.000000','active'::public."status_enum"),
		('ECOportal.su','https://ecoportal.su/rss/news.xml','2025-08-24 02:26:22.000000','active'::public."status_enum"),
		('Meduza','https://meduza.io/rss2/all','2025-08-24 02:26:23.000000','active'::public."status_enum"),
		('Независимая газета','https://www.ng.ru/rss/','2025-08-24 02:26:24.000000','active'::public."status_enum"),
		('Россия в глобальной политике','https://globalaffairs.ru/feed/','2025-08-24 02:26:25.000000','active'::public."status_enum"),
		('Фонтанка.ру','https://www.fontanka.ru/rss','2025-08-24 02:26:26.000000','active'::public."status_enum"),
		('Новости Кузбасса','https://kuzbassnews.ru/engine/rss.php','2025-08-24 02:26:27.000000','active'::public."status_enum")
		ON CONFLICT (url) DO NOTHING;
		`

		if _, err := db.Exec(extendedSources); err != nil && !strings.Contains(err.Error(), "duplicate key") {
			log.Printf("Ошибка при инициализации дополнительных источников: %v", err)
		} else {
			log.Println("Все источники новостей успешно инициализированы")
		}
	}()
}

// fixEscapedCharactersInNews исправляет экранированные символы в заголовках новостей

// UpdateOutdatedRSSSources обновляет устаревшие RSS URL источников
func UpdateOutdatedRSSSources(db *sql.DB) {
	log.Println("Обновление устаревших RSS источников...")

	// Обновляем источники с новыми URL
	updates := []struct {
		oldURL string
		newURL string
		name   string
	}{
		{"https://www.interfax.ru/rss.xml", "https://www.interfax.ru/rss.asp", "Интерфакс"},
		{"https://www.rbc.ru/rss", "https://rssexport.rbc.ru/rbcnews/news/30/full.rss", "РБК"},
		{"https://www.travel.ru/inc/side/yandex.rdf", "https://www.travel.ru/news/feed/", "Travel.ru"},
		{"https://www.fontanka.ru/_transmission_for_yandex.thtml", "https://www.fontanka.ru/rss", "Фонтанка.ру"},
		{"https://ria.ru/export/rss2/politics/index.xml", "https://rssexport.rbc.ru/rbcnews/news/30/full.rss", "Ria.ru - Политика"},
		{"https://ria.ru/export/rss2/economy/index.xml", "https://rssexport.rbc.ru/rbcnews/news/30/full.rss", "Ria.ru - Экономика"},
		{"https://ria.ru/export/rss2/society/index.xml", "https://rssexport.rbc.ru/rbcnews/news/30/full.rss", "Ria.ru - Общество"},
		{"https://sport.ria.ru/export/rss2/sport/index.xml", "https://rssexport.rbc.ru/rbcnews/news/30/full.rss", "Ria.ru - Спорт"},
	}

	for _, update := range updates {
		// Проверяем, существует ли уже источник с новым URL
		var existingID int
		err := db.QueryRow("SELECT id FROM sources WHERE url = $1", update.newURL).Scan(&existingID)

		if err == nil {
			// Новый URL уже существует, удаляем старый источник
			_, err = db.Exec("DELETE FROM sources WHERE url = $1", update.oldURL)
			if err != nil {
				log.Printf("Ошибка удаления старого источника %s (%s): %v", update.name, update.oldURL, err)
			} else {
				log.Printf("✅ Удален старый источник %s (%s), новый URL уже существует", update.name, update.oldURL)
			}
		} else if err == sql.ErrNoRows {
			// Новый URL не существует, обновляем старый
			result, err := db.Exec(`
				UPDATE sources
				SET url = $2
				WHERE url = $1
			`, update.oldURL, update.newURL)

			if err != nil {
				log.Printf("Ошибка обновления источника %s (%s): %v", update.name, update.oldURL, err)
			} else {
				rowsAffected, _ := result.RowsAffected()
				if rowsAffected > 0 {
					log.Printf("✅ Обновлен источник %s: %s → %s", update.name, update.oldURL, update.newURL)
				} else {
					log.Printf("ℹ️  Источник %s (%s) не найден для обновления", update.name, update.oldURL)
				}
			}
		} else {
			log.Printf("Ошибка проверки существования источника %s (%s): %v", update.name, update.newURL, err)
		}
	}

	log.Println("Обновление RSS источников завершено")
}

// fixEscapedCharactersInNews исправляет экранированные символы в заголовках новостей
func fixEscapedCharactersInNews(db *sql.DB) {
	// Исправляем дефисы: заменяем \- на -
	result, err := db.Exec(`
		UPDATE news
		SET title = REPLACE(title, '\-', '-')
		WHERE title LIKE '%\-%'
	`)
	if err != nil {
		log.Printf("Предупреждение: ошибка при исправлении дефисов в заголовках: %v", err)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Исправлено дефисов в заголовках: %d записей", rowsAffected)
	}

	// Исправляем точки: заменяем \. на . (если они были экранированы)
	result, err = db.Exec(`
		UPDATE news 
		SET title = REPLACE(title, '\.', '.')
		WHERE title LIKE '%\.%'
	`)
	if err != nil {
		log.Printf("Предупреждение: ошибка при исправлении точек в заголовках: %v", err)
		return
	}
	rowsAffected, _ = result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Исправлено точек в заголовках: %d записей", rowsAffected)
	}
}

// migrateNewsTable добавляет новые поля для скраппинга к существующей таблице news
func migrateNewsTable(db *sql.DB) {
	migrationQueries := []string{
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS full_text TEXT`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS author VARCHAR(255)`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS category VARCHAR(255)`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS tags TEXT[]`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS images TEXT[]`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS meta_keywords TEXT`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS meta_description TEXT`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS meta_data JSONB`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS content_html TEXT`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS scraped_at TIMESTAMP`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS scrape_status VARCHAR(50) DEFAULT 'pending'`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS scrape_error TEXT`,
		`ALTER TABLE news ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`,
	}

	for _, query := range migrationQueries {
		_, err := db.Exec(query)
		if err != nil {
			log.Printf("Предупреждение при миграции таблицы news: %v (запрос: %s)", err, query)
		}
	}

	// Обновляем tsvector для включения full_text, если он еще не обновлен
	// Проверяем, существует ли уже правильный tsvector
	checkQuery := `
		SELECT COUNT(*) FROM pg_attribute 
		WHERE attrelid = 'news'::regclass 
		AND attname = 'tvs'
	`
	var count int
	err := db.QueryRow(checkQuery).Scan(&count)
	if err == nil && count > 0 {
		// Пересоздаем tsvector с учетом full_text
		updateTsvectorQuery := `
			ALTER TABLE news DROP COLUMN IF EXISTS tvs;
			ALTER TABLE news ADD COLUMN tvs tsvector NULL GENERATED ALWAYS AS (
				to_tsvector('russian', COALESCE(title, '') || ' ' || COALESCE(description, '') || ' ' || COALESCE(full_text, ''))
			) STORED;
		`
		_, err = db.Exec(updateTsvectorQuery)
		if err != nil {
			log.Printf("Предупреждение при обновлении tsvector: %v", err)
		}
	}
	
	log.Println("Миграция таблицы news завершена")
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
	NewsSuccess    int // Количество успешно обработанных новостей
	NewsFailed     int // Количество новостей с ошибками обработки
	NewsPending    int // Количество новостей в ожидании обработки
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

	// Количество новостей по статусам обработки
	err = db.QueryRow("SELECT COUNT(*) FROM news WHERE scrape_status = 'success'").Scan(&stats.NewsSuccess)
	if err != nil {
		return stats, fmt.Errorf("ошибка при получении количества успешных новостей: %w", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM news WHERE scrape_status = 'failed'").Scan(&stats.NewsFailed)
	if err != nil {
		return stats, fmt.Errorf("ошибка при получении количества неудачных новостей: %w", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM news WHERE scrape_status = 'pending' OR scrape_status IS NULL").Scan(&stats.NewsPending)
	if err != nil {
		return stats, fmt.Errorf("ошибка при получении количества новостей в ожидании: %w", err)
	}

	return stats, nil
}
