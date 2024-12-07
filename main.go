package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/lib/pq"
	"github.com/mmcdole/gofeed"
)

const TIMEOUT = 60

var (
	TGBotKey string
	// Настройки базы данных
	DBHost string
	DBPort int
	DBUser string
	DBPass string
	DBName string
)

func init() {
	// Считывание переменных окружения
	TGBotKey = getEnv("TELEGRAM_API_KEY", "")
	DBHost = getEnv("POSTGRES_HOST", "")
	DBPort, _ = strconv.Atoi(getEnv("POSTGRES_PORT", "5432"))
	DBUser = getEnv("POSTGRES_USER", "postgres")
	DBPass = getEnv("POSTGRES_PASSWORD", "password")
	DBName = getEnv("POSTGRES_NAME", "news_bot")

	// Проверка обязательных переменных
	if TGBotKey == "" {
		log.Fatal("Не задана переменная окружения TELEGRAM_API_KEY")
	}
	if DBHost == "" {
		log.Fatal("Не задана переменная окружения POSTGRES_HOST")
	}
	if DBUser == "" {
		log.Fatal("Не задана переменная окружения POSTGRES_USER")
	}
	if DBPass == "" {
		log.Fatal("Не задана переменная окружения POSTGRES_PASSWORD")
	}
	if DBName == "" {
		log.Fatal("Не задана переменная окружения POSTGRES_NAME")
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Структура для хранения новостей
type News struct {
	Title string
	Link  string
}

var (
	db             *sql.DB
	sources        []string
	newsSentToUser map[string]struct{} // Для предотвращения повторной отправки новостей
)

func main() {
	var err error
	// Подключение к базе данных
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", DBHost, DBPort, DBUser, DBPass, DBName)
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}
	defer db.Close()

	// Инициализация
	newsSentToUser = make(map[string]struct{})
	initDB()

	// Запуск Telegram бота
	bot, err := tgbotapi.NewBotAPI(TGBotKey)
	if err != nil {
		log.Fatalf("Не удалось создать Telegram бота: %v", err)
	}

	bot.Debug = true
	log.Printf("Бот авторизован как %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Запуск парсинга новостей
	go startRSSPolling(bot)

	// Обработка команд
	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Command() {
		case "start":
			saveUser(update.Message.Chat.ID, bot)
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Вы подписались на обновления новостей!"))
		case "add":
			source := update.Message.CommandArguments()
			if source == "" {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Укажите URL источника!"))
				continue
			}
			addSource(source, bot, update.Message.Chat.ID)
		case "news":
			sendLatestNews(bot, update.Message.Chat.ID, 10)
		case "search":
			search := update.Message.CommandArguments()
			if search == "" {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Укажите поисковый запрос!"))
				continue
			}
			sendFoundNews(search, bot, update.Message.Chat.ID, 10)
		default:
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неизвестная команда"))
		}
	}
}

// Инициализация базы данных
func initDB() {
	query := `
	CREATE TABLE IF NOT EXISTS sources (
		id SERIAL PRIMARY KEY,
		url TEXT NOT NULL UNIQUE
	);

	CREATE TABLE IF NOT EXISTS news (
		id SERIAL PRIMARY KEY,
		title VARCHAR(1024) NOT NULL,
		description TEXT NOT NULL,
		link VARCHAR(1024) NOT NULL UNIQUE,
		published_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		tsvector_title_description TSVECTOR GENERATED ALWAYS AS (setweight(to_tsvector('russian', title), 'A') || setweight(to_tsvector('russian', description), 'B')) STORED
	);
	CREATE TABLE IF NOT EXISTS users (
		chat_id BIGINT PRIMARY KEY
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка при инициализации базы данных: %v", err)
	}
}

// Сохранение пользователя
func saveUser(chatID int64, bot *tgbotapi.BotAPI) {
	_, err := db.Exec("INSERT INTO users (chat_id) VALUES ($1) ON CONFLICT DO NOTHING", chatID)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при сохранении пользователя"))
		log.Printf("Ошибка при сохранении пользователя: %v", err)
		return
	}
}

// Добавление источника
func addSource(url string, bot *tgbotapi.BotAPI, chatID int64) {
	_, err := db.Exec("INSERT INTO sources (url) VALUES ($1) ON CONFLICT DO NOTHING", url)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении источника"))
		return
	}
	sources = append(sources, url)
	bot.Send(tgbotapi.NewMessage(chatID, "Источник добавлен"))
}

// Отправка последних новостей
func sendLatestNews(bot *tgbotapi.BotAPI, chatID int64, count int) {
	rows, err := db.Query("SELECT title, description, link, published_at FROM news ORDER BY published_at DESC LIMIT $1", count)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении новостей"))
		return
	}
	defer rows.Close()

	var message string
	for rows.Next() {
		var title, description, link string
		var publishedAt time.Time
		rows.Scan(&title, &description, &link, &publishedAt)
		message += fmt.Sprintf("%s\n[%s](%s)\n%s", publishedAt.Format("02.01.2006 15:04"), title, link, description)
	}
	if message == "" {
		message = "Новостей пока нет"
	}
	msg := tgbotapi.NewMessage(chatID, message)
	msg.DisableWebPagePreview = true
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func sendFoundNews(search string, bot *tgbotapi.BotAPI, chatID int64, count int) {
	rows, err := db.Query(fmt.Sprintf("select count(*) as total from news where tsvector_title_description @@ to_tsquery('%s:*');", search))
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при поиске и определении количества новостей"))
		return
	}
	var total int
	for rows.Next() {
		rows.Scan(&total)
	}
	if total == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "Новостей не найдено"))
		return
	}

	rows, err = db.Query(fmt.Sprintf("select ts_rank(tsvector_title_description, to_tsquery('%s:*')) AS rank, title, description, link, published_at from news where tsvector_title_description @@ to_tsquery('%s:*') order by rank desc published_at desc limit %d;", search, count))
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении новостей"))
		return
	}
	defer rows.Close()

	message := fmt.Sprintf("Всего найдено новостей: %d\n", total)
	message += fmt.Sprintf("Поиск по запросу: %s\n\n", search)
	for rows.Next() {
		var title, description, link string
		var publishedAt time.Time
		rows.Scan(&title, &description, &link, &publishedAt)
		message += fmt.Sprintf("%s\n[%s](%s)\n%s", publishedAt.Format("02.01.2006 15:04"), title, link, description)
	}
	if message == "" {
		message = "Новостей пока нет"
	}
	msg := tgbotapi.NewMessage(chatID, message)
	msg.DisableWebPagePreview = true
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// Запуск парсинга RSS лент
func startRSSPolling(bot *tgbotapi.BotAPI) {
	for {
		rows, err := db.Query("SELECT url FROM sources")
		if err != nil {
			log.Printf("Ошибка при получении источников: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		var urls []string
		for rows.Next() {
			var url string
			rows.Scan(&url)
			urls = append(urls, url)
		}
		rows.Close()

		for _, url := range urls {
			parseRSS(url, bot)
		}

		time.Sleep(TIMEOUT * time.Second) // Обновление каждые 60 секунд
	}
}

// Парсинг RSS
func parseRSS(url string, bot *tgbotapi.BotAPI) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
	if err != nil {
		log.Printf("Ошибка при парсинге RSS ленты %s: %v", url, err)
		return
	}

	for _, item := range feed.Items {
		if _, exists := newsSentToUser[item.Link]; exists {
			continue
		}
		newsSentToUser[item.Link] = struct{}{}

		publishedAt := time.Now() // Если дата отсутствует, используем текущее время
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		}

		_, err := db.Exec("INSERT INTO news (title, description, link, published_at) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", item.Title, item.Description, item.Link, publishedAt)
		if err != nil {
			log.Printf("Ошибка при сохранении новости: %v", err)
			continue
		}

		// Получение всех пользователей
		rows, err := db.Query("SELECT chat_id FROM users")
		if err != nil {
			log.Printf("Ошибка при получении пользователей: %v", err)
			continue
		}
		defer rows.Close()

		for rows.Next() {
			var chatID int64
			rows.Scan(&chatID)

			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("%s\n[%s](%s)\n%s", publishedAt.Format("02.01.2006 15:04"), item.Title, item.Link, item.Description))
			msg.ParseMode = "Markdown"
			msg.DisableWebPagePreview = true
			bot.Send(msg)
		}
	}
}
