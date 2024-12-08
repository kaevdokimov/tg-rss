package bot

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strconv"

	"tg-rss/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartCommandHandler запускает обработку команд Telegram
func StartCommandHandler(bot *tgbotapi.BotAPI, dbConn *sql.DB, interval int) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = interval

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		switch update.Message.Command() {
		case "start":
			handleStart(bot, dbConn, update.Message.Chat.UserName, update.Message.Chat.ID)
		case "add":
			handleAddSource(bot, dbConn, update.Message.Chat.ID, update.Message.CommandArguments())
		case "sources":
			handleShowSources(bot, dbConn, update.Message.Chat.ID)
		case "addsub":
			handleAddSubscription(bot, dbConn, update.Message.Chat.ID, update.Message.CommandArguments())
		case "delsub":
			handleDelSubscription(bot, dbConn, update.Message.Chat.ID, update.Message.CommandArguments())
		case "news":
			handleLatestNews(bot, dbConn, update.Message.Chat.ID, 10)
		default:
			handleUnknownCommand(bot, update.Message.Chat.ID)
		}
	}
}

// handleStart обрабатывает команду /start
func handleStart(bot *tgbotapi.BotAPI, dbConn *sql.DB, username string, chatId int64) {
	var user = db.User{
		Username: username,
		ChatId:   chatId,
	}
	_, err := db.SaveUser(dbConn, user)
	if err != nil {
		log.Printf("Ошибка при сохранении пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Ошибка при подключении к боту. Попробуйте позже")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, "Вы успешно подключились к боту!")
	bot.Send(msg)
}

// handleAddSource обрабатывает команду /add для добавления нового источника
func handleAddSource(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, link string) {
	if link == "" {
		msg := tgbotapi.NewMessage(chatId, "Укажите URL источника после команды /add")
		bot.Send(msg)
		return
	}

	u, err := url.Parse(link)
	if err != nil {
		msg := tgbotapi.NewMessage(chatId, "Укажите валидный URL источника после команды /add")
		bot.Send(msg)
		return
	}

	var source = db.Source{
		Name: u.Host,
		Url:  link,
	}

	err = db.SaveSource(dbConn, source)
	if err != nil {
		log.Printf("Ошибка при добавлении источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Не удалось добавить источник. Возможно, он уже существует")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("Источник %s успешно добавлен!", link))
	bot.Send(msg)

	source, err = db.FindSourceActiveByUrl(dbConn, link)

	var subscription = db.Subscription{
		ChatId:   chatId,
		SourceId: source.Id,
	}

	err = db.SaveSubscription(dbConn, subscription)
}

// handleShowSources обрабатывает команду /sources для вывода списка источников
func handleShowSources(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	sources, err := db.FindActiveSources(dbConn)
	if err != nil {
		log.Printf("Ошибка при получении списка источников: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Не удалось получить список источников")
		bot.Send(msg)
		return
	}
	msgText := "ID: Название\n"
	for _, source := range sources {
		msgText += fmt.Sprintf("%d: %s\n", source.Id, source.Name)
	}
	msg := tgbotapi.NewMessage(chatId, msgText)
	bot.Send(msg)

}

// handleAddSubscription обрабатывает команду /add-sub для добавления подписки на источник
func handleAddSubscription(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, sourceId string) {
	if sourceId == "" {
		msg := tgbotapi.NewMessage(chatId, "Укажите ID источника после команды /add-sub")
		bot.Send(msg)
		return
	}

	sourceIdInt, err := strconv.ParseInt(sourceId, 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("ID источника не выглядит как число: %q.\n Укажите ID источника после команды /add-sub", sourceId))
		bot.Send(msg)
		return
	}

	_, err = db.FindActiveSourceById(dbConn, sourceIdInt)

	if err != nil {
		msg := tgbotapi.NewMessage(chatId, "Укажите существующий ID источника после команды /add-sub")
		bot.Send(msg)
		return
	}

	var subscription = db.Subscription{
		ChatId:   chatId,
		SourceId: sourceIdInt,
	}

	err = db.SaveSubscription(dbConn, subscription)
	if err != nil {
		log.Printf("Ошибка при добавлении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Не удалось добавить подписку. Возможно, она уже существует")
		bot.Send(msg)
		return
	}

}

// handleDelSubscription обрабатывает команду /del-sub для удаления подписки на источник
func handleDelSubscription(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, sourceId string) {
	if sourceId == "" {
		msg := tgbotapi.NewMessage(chatId, "Укажите ID источника после команды /add-sub")
		bot.Send(msg)
		return
	}

	sourceIdInt, err := strconv.ParseInt(sourceId, 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("ID источника не выглядит как число: %q.\n Укажите ID источника после команды /add-sub", sourceId))
		bot.Send(msg)
		return
	}

	_, err = db.FindActiveSourceById(dbConn, sourceIdInt)

	if err != nil {
		msg := tgbotapi.NewMessage(chatId, "Укажите существующий ID источника после команды /add-sub")
		bot.Send(msg)
		return
	}

	var subscription = db.Subscription{
		ChatId:   chatId,
		SourceId: sourceIdInt,
	}

	err = db.DeleteSubscription(dbConn, subscription)
	if err != nil {
		log.Printf("Ошибка при удалении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Не удалось удалить подписку. Возможно, она не существует")
		bot.Send(msg)
		return
	}
}

// handleLatestNews обрабатывает команду /news для вывода последних новостей
func handleLatestNews(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, count int) {
	news, err := db.GetLatestNewsByUser(dbConn, chatId, count)
	if err != nil {
		log.Printf("Ошибка при получении новостей: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Ошибка при получении новостей. Попробуйте позже")
		bot.Send(msg)
		return
	}

	if len(news) == 0 {
		msg := tgbotapi.NewMessage(chatId, "Новостей пока нет")
		bot.Send(msg)
		return
	}

	message := "Последние новости:\n\n"
	for i, item := range news {
		message += formatMessage(i+1, item.Title, item.Link, item.Description, item.PublishedAt)
	}

	msg := tgbotapi.NewMessage(chatId, message)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = true
	bot.Send(msg)
}

// handleUnknownCommand обрабатывает неизвестные команды
func handleUnknownCommand(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "Неизвестная команда. Попробуйте /start, /add или /news")
	bot.Send(msg)
}
