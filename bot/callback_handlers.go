package bot

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"tg-rss/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleCallback обрабатывает callback-запросы от inline кнопок
func handleCallback(bot *tgbotapi.BotAPI, dbConn *sql.DB, callback *tgbotapi.CallbackQuery) {
	chatId := callback.Message.Chat.ID
	data := callback.Data

	// Отвечаем на callback, чтобы убрать "часики" у кнопки
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	bot.Send(callbackResponse)

	switch {
	case data == "main_menu":
		handleMainMenu(bot, chatId)
	case data == "news":
		handleLatestNewsImproved(bot, dbConn, chatId, 10)
	case data == "sources":
		handleShowSources(bot, dbConn, chatId)
	case data == "add_source":
		handleAddSourcePrompt(bot, chatId)
	case data == "my_subscriptions":
		handleMySubscriptions(bot, dbConn, chatId)
	case data == "help":
		handleHelp(bot, chatId)
	case strings.HasPrefix(data, "source_"):
		handleSourceDetails(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "subscribe_"):
		handleSubscribe(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "unsubscribe_"):
		handleUnsubscribe(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "share_link_"):
		handleShareNews(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "copy_link_"):
		handleCopyLink(bot, chatId, data)
	case strings.HasPrefix(data, "news_page_"):
		handleNewsPage(bot, dbConn, chatId, data)
	case data == "back_to_news":
		handleLatestNewsImproved(bot, dbConn, chatId, 10)
	default:
		handleUnknownCallback(bot, chatId)
	}
}

// handleMainMenu показывает главное меню
func handleMainMenu(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "🏠 *Главное меню*\n\nВыберите действие:")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleAddSourcePrompt показывает инструкцию для добавления источника
func handleAddSourcePrompt(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "➕ *Добавление источника*\n\nОтправьте URL RSS-ленты, которую хотите добавить.\n\nПримеры:\n• https://tass.ru/rss/v2.xml\n• https://rss.cnn.com/rss/edition.rss")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createAddSourceKeyboard()
	bot.Send(msg)
}

// handleMySubscriptions показывает подписки пользователя
func handleMySubscriptions(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	subscriptions, err := db.GetUserSubscriptionsWithDetails(dbConn, chatId)
	if err != nil {
		log.Printf("Ошибка при получении подписок: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении подписок")
		bot.Send(msg)
		return
	}

	if len(subscriptions) == 0 {
		msg := tgbotapi.NewMessage(chatId, "📝 У вас пока нет подписок на источники.\n\nДобавьте источники через меню «Мои источники»")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Получаем информацию об источниках
	sources, err := db.FindActiveSources(dbConn)
	if err != nil {
		log.Printf("Ошибка при получении источников: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении источников")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, "📝 *Ваши подписки:*\n\nНажмите на источник, чтобы отписаться от него")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createMySubscriptionsKeyboard(subscriptions, sources)
	bot.Send(msg)
}

// handleSourceDetails показывает детали источника с возможностью подписки/отписки
func handleSourceDetails(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) != 2 {
		handleUnknownCallback(bot, chatId)
		return
	}

	sourceId, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		handleUnknownCallback(bot, chatId)
		return
	}

	source, err := db.FindActiveSourceById(dbConn, sourceId)
	if err != nil {
		log.Printf("Ошибка при поиске источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Источник не найден")
		bot.Send(msg)
		return
	}

	// Проверяем, подписан ли пользователь
	isSubscribed, err := db.IsUserSubscribed(dbConn, chatId, sourceId)
	if err != nil {
		log.Printf("Ошибка при проверке подписки: %v", err)
		isSubscribed = false
	}

	var statusText string
	if isSubscribed {
		statusText = "✅ Вы подписаны на этот источник"
	} else {
		statusText = "❌ Вы не подписаны на этот источник"
	}

	msgText := fmt.Sprintf("📰 *%s*\n\n🔗 %s\n\n%s", source.Name, source.Url, statusText)
	msg := tgbotapi.NewMessage(chatId, msgText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = createSubscriptionKeyboard(sourceId, isSubscribed)
	bot.Send(msg)
}

// handleSubscribe подписывает пользователя на источник
func handleSubscribe(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) != 2 {
		handleUnknownCallback(bot, chatId)
		return
	}

	sourceId, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		handleUnknownCallback(bot, chatId)
		return
	}

	// Проверяем существование источника
	_, err = db.FindActiveSourceById(dbConn, sourceId)
	if err != nil {
		log.Printf("Ошибка при поиске источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Источник не найден")
		bot.Send(msg)
		return
	}

	// Проверяем, не подписан ли уже пользователь
	isSubscribed, err := db.IsUserSubscribed(dbConn, chatId, sourceId)
	if err != nil {
		log.Printf("Ошибка при проверке подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке подписки")
		bot.Send(msg)
		return
	}

	if isSubscribed {
		msg := tgbotapi.NewMessage(chatId, "ℹ️ Вы уже подписаны на этот источник")
		bot.Send(msg)
		return
	}

	// Проверяем, существует ли пользователь, если нет - регистрируем его
	exists, err := db.UserExists(dbConn, chatId)
	if err != nil {
		log.Printf("Ошибка при проверке существования пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке пользователя")
		bot.Send(msg)
		return
	}

	if !exists {
		// Регистрируем пользователя
		user := db.User{
			ChatId:   chatId,
			Username: "unknown", // Будет обновлено при следующем /start
		}
		_, err = db.SaveUser(dbConn, user)
		if err != nil {
			log.Printf("Ошибка при регистрации пользователя: %v", err)
			msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при регистрации пользователя")
			bot.Send(msg)
			return
		}
		log.Printf("Автоматически зарегистрирован пользователь с chatId %d", chatId)
	}

	// Добавляем подписку
	subscription := db.Subscription{
		ChatId:   chatId,
		SourceId: sourceId,
	}

	err = db.SaveSubscription(dbConn, subscription)
	if err != nil {
		log.Printf("Ошибка при добавлении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при добавлении подписки")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, "✅ Вы успешно подписались на источник!")
	bot.Send(msg)
}

// handleUnsubscribe отписывает пользователя от источника
func handleUnsubscribe(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) != 2 {
		handleUnknownCallback(bot, chatId)
		return
	}

	sourceId, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		handleUnknownCallback(bot, chatId)
		return
	}

	// Проверяем существование источника
	source, err := db.FindActiveSourceById(dbConn, sourceId)
	if err != nil {
		log.Printf("Ошибка при поиске источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Источник не найден")
		bot.Send(msg)
		return
	}

	// Удаляем подписку
	subscription := db.Subscription{
		ChatId:   chatId,
		SourceId: sourceId,
	}

	err = db.DeleteSubscription(dbConn, subscription)
	if err != nil {
		log.Printf("Ошибка при удалении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при удалении подписки")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("✅ Вы отписались от источника «%s»", source.Name))
	bot.Send(msg)
}

// handleUnknownCallback обрабатывает неизвестные callback-запросы
func handleUnknownCallback(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "❓ Неизвестная команда")
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleShareNews обрабатывает запрос на поделиться новостью
func handleShareNews(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		handleUnknownCallback(bot, chatId)
		return
	}

	// Восстанавливаем ссылку из частей (share_link_https://example.com -> https://example.com)
	link := strings.Join(parts[2:], "_")

	// Получаем заголовок новости из БД по ссылке
	title, err := getNewsTitleByLink(dbConn, link)
	if err != nil {
		log.Printf("Ошибка при получении заголовка новости: %v", err)
		title = "Новость" // fallback заголовок
	}

	msg := tgbotapi.NewMessage(chatId, "📤 *Поделиться новостью:*\n\nИспользуйте кнопку ниже для шаринга")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createShareKeyboard(link, title)
	bot.Send(msg)
}

// handleCopyLink обрабатывает запрос на копирование ссылки
func handleCopyLink(bot *tgbotapi.BotAPI, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		handleUnknownCallback(bot, chatId)
		return
	}

	// Восстанавливаем ссылку из частей
	link := strings.Join(parts[2:], "_")

	msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("🔗 *Ссылка скопирована:*\n\n`%s`", link))
	msg.ParseMode = tgbotapi.ModeMarkdown
	bot.Send(msg)
}

// handleNewsPage обрабатывает пагинацию новостей
func handleNewsPage(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) != 3 {
		handleUnknownCallback(bot, chatId)
		return
	}

	_, err := strconv.Atoi(parts[2])
	if err != nil {
		handleUnknownCallback(bot, chatId)
		return
	}

	// Пока что просто показываем первые 10 новостей
	// В будущем можно добавить настоящую пагинацию
	handleLatestNewsImproved(bot, dbConn, chatId, 10)
}

// getNewsTitleByLink получает заголовок новости по ссылке из БД
func getNewsTitleByLink(dbConn *sql.DB, link string) (string, error) {
	var title string
	query := "SELECT title FROM news WHERE link = $1 LIMIT 1"
	err := dbConn.QueryRow(query, link).Scan(&title)
	if err != nil {
		return "", err
	}
	return title, nil
}
