package bot

import (
	"database/sql"
	"log"
	"tg-rss/config"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func StartBot(cfgTgBot *config.TgBotConfig, dbConn *sql.DB) {
	interval := time.Duration(cfgTgBot.Timeout) * time.Second
	// Инициализация Telegram-бота
	bot, err := tgbotapi.NewBotAPI(cfgTgBot.ApiKey)
	if err != nil {
		log.Fatalf("Ошибка инициализации бота: %v", err)
	}
	log.Printf("Бот авторизован как %s", bot.Self.UserName)

	// Запуск обработки команд
	go StartCommandHandler(bot, dbConn, cfgTgBot.Timeout)

	// Запуск опроса RSS-источников
	go StartRSSPolling(dbConn, bot, interval, time.Local)

	// Задержка для предотвращения выхода из программы
	select {}
}
