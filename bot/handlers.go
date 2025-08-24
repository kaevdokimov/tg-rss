package bot

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"tg-rss/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartCommandHandler запускает обработку команд Telegram
func StartCommandHandler(bot *tgbotapi.BotAPI, dbConn *sql.DB, interval int) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = interval

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// Обработка callback-запросов от inline кнопок
		if update.CallbackQuery != nil {
			log.Printf("[%s] Callback: %s", update.CallbackQuery.From.UserName, update.CallbackQuery.Data)
			handleCallback(bot, dbConn, update.CallbackQuery)
			continue
		}

		// Обработка обычных сообщений
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		// Проверяем, является ли сообщение командой
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				handleStart(bot, dbConn, update.Message.Chat.UserName, update.Message.Chat.ID)
			case "help":
				handleHelp(bot, update.Message.Chat.ID)
			case "add":
				handleAddSource(bot, dbConn, update.Message.Chat.ID, update.Message.CommandArguments())
			case "sources":
				handleShowSources(bot, dbConn, update.Message.Chat.ID)
			case "addsub":
				handleAddSubscription(bot, dbConn, update.Message.Chat.ID, update.Message.CommandArguments())
			case "delsub":
				handleDelSubscription(bot, dbConn, update.Message.Chat.ID, update.Message.CommandArguments())
			case "news":
				handleLatestNewsImproved(bot, dbConn, update.Message.Chat.ID, 10)
			default:
				handleUnknownCommand(bot, update.Message.Chat.ID)
			}
		} else {
			// Обработка обычных текстовых сообщений (например, URL для добавления источника)
			handleTextMessage(bot, dbConn, update.Message)
		}
	}
}

// handleStart обрабатывает команду /start
func handleStart(bot *tgbotapi.BotAPI, dbConn *sql.DB, username string, chatId int64) {
	var user = db.User{
		Username: username,
		ChatId:   chatId,
	}

	insertedId, err := db.SaveUser(dbConn, user)
	if err != nil {
		log.Printf("Ошибка добавления пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "Ошибка при подключении к боту. Попробуйте позже 😫")
		bot.Send(msg)
		return
	}

	log.Printf("Пользователь %s подключился к боту с chatId %d", user.Username, insertedId)
	msg := tgbotapi.NewMessage(chatId, "👋 Привет, я бот для получения новостей с сайтов!\n\nИспользуйте кнопки ниже для навигации:")
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleAddSource обрабатывает команду /add для добавления нового источника
func handleAddSource(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, link string) {
	if link == "" {
		msg := tgbotapi.NewMessage(chatId, "❌ Укажите URL источника после команды /add")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	u, err := url.Parse(link)
	if err != nil {
		msg := tgbotapi.NewMessage(chatId, "❌ Укажите валидный URL источника после команды /add")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Создаем более читаемое название источника
	sourceName := u.Host
	if u.Host == "" {
		sourceName = "Неизвестный источник"
	} else {
		// Убираем www. если есть
		if strings.HasPrefix(u.Host, "www.") {
			sourceName = u.Host[4:]
		}
		// Делаем первую букву заглавной
		if len(sourceName) > 0 {
			sourceName = strings.ToUpper(sourceName[:1]) + sourceName[1:]
		}
	}

	var source = db.Source{
		Name: sourceName,
		Url:  link,
	}

	err = db.SaveSource(dbConn, source)
	if err != nil {
		log.Printf("Ошибка при добавлении источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось добавить источник. Возможно, он уже существует")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("✅ Источник [%s](%s) успешно добавлен!", source.Name, source.Url))
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)

	source, err = db.FindSourceActiveByUrl(dbConn, link)
	if err != nil {
		log.Printf("Ошибка при поиске источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось найти добавленный источник")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Проверяем, существует ли пользователь, если нет - регистрируем его
	exists, err := db.UserExists(dbConn, chatId)
	if err != nil {
		log.Printf("Ошибка при проверке существования пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке пользователя")
		msg.ReplyMarkup = createMainKeyboard()
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
			msg.ReplyMarkup = createMainKeyboard()
			bot.Send(msg)
			return
		}
		log.Printf("Автоматически зарегистрирован пользователь с chatId %d", chatId)
	}

	var subscription = db.Subscription{
		ChatId:   chatId,
		SourceId: source.Id,
	}

	err = db.SaveSubscription(dbConn, subscription)
	if err != nil {
		log.Printf("Ошибка при добавлении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось добавить подписку. Возможно, она уже существует")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	successMsg := tgbotapi.NewMessage(chatId, "✅ Подписка на источник успешно добавлена!")
	successMsg.ReplyMarkup = createMainKeyboard()
	bot.Send(successMsg)
}

// handleShowSources обрабатывает команду /sources для вывода списка источников
func handleShowSources(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	sources, err := db.FindActiveSources(dbConn)
	if err != nil {
		log.Printf("Ошибка при получении списка источников: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось получить список источников")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}
	if len(sources) == 0 {
		msg := tgbotapi.NewMessage(chatId, "📋 Источников пока нет.\n\nДобавьте первый источник через кнопку «Добавить источник»")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, "📋 *Доступные источники:*\n\nНажмите на источник, чтобы подписаться или отписаться от него")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createSourcesKeyboard(sources)
	bot.Send(msg)
}

// handleAddSubscription обрабатывает команду /add-sub для добавления подписки на источник
func handleAddSubscription(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, sourceId string) {
	if sourceId == "" {
		msg := tgbotapi.NewMessage(chatId, "❌ Укажите ID источника после команды /add-sub")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	sourceIdInt, err := strconv.ParseInt(sourceId, 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("❌ ID источника не выглядит как число: %q.\n Укажите ID источника после команды /add-sub", sourceId))
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	_, err = db.FindActiveSourceById(dbConn, sourceIdInt)

	if err != nil {
		msg := tgbotapi.NewMessage(chatId, "❌ Укажите существующий ID источника после команды /add-sub")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Проверяем, существует ли пользователь, если нет - регистрируем его
	exists, err := db.UserExists(dbConn, chatId)
	if err != nil {
		log.Printf("Ошибка при проверке существования пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке пользователя")
		msg.ReplyMarkup = createMainKeyboard()
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
			msg.ReplyMarkup = createMainKeyboard()
			bot.Send(msg)
			return
		}
		log.Printf("Автоматически зарегистрирован пользователь с chatId %d", chatId)
	}

	var subscription = db.Subscription{
		ChatId:   chatId,
		SourceId: sourceIdInt,
	}

	err = db.SaveSubscription(dbConn, subscription)
	if err != nil {
		log.Printf("Ошибка при добавлении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось добавить подписку. Возможно, она уже существует")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, "✅ Подписка успешно добавлена!")
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleDelSubscription обрабатывает команду /del-sub для удаления подписки на источник
func handleDelSubscription(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, sourceId string) {
	if sourceId == "" {
		msg := tgbotapi.NewMessage(chatId, "❌ Укажите ID источника после команды /delsub")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	sourceIdInt, err := strconv.ParseInt(sourceId, 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("❌ ID источника не выглядит как число: %q.\n Укажите ID источника после команды /delsub", sourceId))
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	_, err = db.FindActiveSourceById(dbConn, sourceIdInt)

	if err != nil {
		msg := tgbotapi.NewMessage(chatId, "❌ Укажите существующий ID источника после команды /delsub")
		msg.ReplyMarkup = createMainKeyboard()
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
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось удалить подписку. Возможно, она не существует")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, "✅ Подписка успешно удалена!")
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleLatestNewsImproved обрабатывает команду /news с улучшенным форматированием
func handleLatestNewsImproved(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, count int) {
	news, err := db.GetLatestNewsByUser(dbConn, chatId, count)
	if err != nil {
		log.Printf("Ошибка при получении новостей: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении новостей. Попробуйте позже")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	if len(news) == 0 {
		msg := tgbotapi.NewMessage(chatId, "📰 Новостей пока нет.\n\nПодпишитесь на источники, чтобы получать новости")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	message := "📰 *Последние новости:*\n\n"
	for i, item := range news {
		message += formatMessage(i+1, item.Title, item.Description, item.PublishedAt, item.SourceName)
	}

	msg := tgbotapi.NewMessage(chatId, message)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = createNewsListKeyboard(1, 1, false)
	bot.Send(msg)
}

// handleHelp обрабатывает команду /help для вывода справки
func handleHelp(bot *tgbotapi.BotAPI, chatId int64) {
	helpText := `📚 *Справка по командам бота*

*Основные команды:*
/start - Начать работу с ботом
/help - Показать эту справку

*Работа с источниками:*
/add <URL> - Добавить новый RSS источник
/sources - Показать список всех источников

*Управление подписками:*
/addsub <ID> - Подписаться на источник по ID
/delsub <ID> - Отписаться от источника по ID

*Получение новостей:*
/news - Показать последние 10 новостей

*Примеры использования:*
/add https://tass.ru/rss/v2.xml
/addsub 1
/delsub 1

💡 *Совет:* Используйте кнопки меню для более удобной навигации!`

	msg := tgbotapi.NewMessage(chatId, helpText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleTextMessage обрабатывает обычные текстовые сообщения
func handleTextMessage(bot *tgbotapi.BotAPI, dbConn *sql.DB, message *tgbotapi.Message) {
	text := message.Text
	chatId := message.Chat.ID

	// Проверяем, является ли текст URL
	if isValidURL(text) {
		handleAddSource(bot, dbConn, chatId, text)
		return
	}

	// Если это не URL, показываем главное меню
	msg := tgbotapi.NewMessage(chatId, "🏠\n\nВыберите действие:")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// isValidURL проверяет, является ли строка валидным URL
func isValidURL(text string) bool {
	u, err := url.Parse(text)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// handleUnknownCommand обрабатывает неизвестные команды
func handleUnknownCommand(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "Неизвестная команда. Попробуйте /start или используйте кнопки меню")
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}
