package main

import (
	"log"
	"tg-rss/bot"
	"tg-rss/config"
	"tg-rss/db"
)

func main() {
	// Настройки
	cfgDB := config.LoadDBConfig()
	cfgTgBot := config.LoadTgBotConfig()

	// Инициализация базы данных
	dbConn, err := db.Connect(cfgDB)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer dbConn.Close()

	db.InitSchema(dbConn)

	// Обновляем названия существующих источников
	err = db.UpdateSourceNames(dbConn)
	if err != nil {
		log.Printf("Предупреждение: не удалось обновить названия источников: %v", err)
	}

	bot.StartBot(cfgTgBot, dbConn)
}
