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
		case "news5":
			sendLatestNews(bot, update.Message.Chat.ID, 5)
		case "news10":
			sendLatestNews(bot, update.Message.Chat.ID, 10)
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
		title TEXT NOT NULL,
		link TEXT NOT NULL UNIQUE,
		published_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
	rows, err := db.Query("SELECT title, link, published_at FROM news ORDER BY published_at DESC LIMIT $1", count)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении новостей"))
		return
	}
	defer rows.Close()

	var message string
	var counter int
	for rows.Next() {
		var title, link string
		var publishedAt time.Time
		rows.Scan(&title, &link, &publishedAt)
		counter++
		message += fmt.Sprintf("%d. [%s](%s) - %s\n", counter, title, link, publishedAt.Format("02.01.2006 15:04"))
	}
	if message == "" {
		message = "Новостей пока нет"
	}
	msg := tgbotapi.NewMessage(chatID, message)
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

		time.Sleep(10 * time.Second) // Обновление каждые 10 секунд
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

		_, err := db.Exec("INSERT INTO news (title, link, published_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", item.Title, item.Link, publishedAt)
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

			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("[%s](%s)", item.Title, item.Link))
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		}

		for rows.Next() {
			var chatID int64
			rows.Scan(&chatID)

			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("[%s](%s) - %s\n", item.Title, item.Link, publishedAt.Format("02.01.2006 15:04")))
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		}
	}
}
