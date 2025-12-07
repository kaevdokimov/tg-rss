package bot

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"tg-rss/db"
	"tg-rss/monitoring"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var handlerLogger = monitoring.NewLogger("Handler")

// StartCommandHandler запускает обработку команд Telegram
func StartCommandHandler(bot *tgbotapi.BotAPI, dbConn *sql.DB, interval int) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = interval

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// Обработка callback-запросов от inline кнопок
		if update.CallbackQuery != nil {
			monitoring.IncrementTelegramCommands()
			handlerLogger.Debug("[%s] Callback: %s", update.CallbackQuery.From.UserName, update.CallbackQuery.Data)
			handleCallback(bot, dbConn, update.CallbackQuery)
			continue
		}

		// Обработка обычных сообщений
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			monitoring.IncrementTelegramCommands()
		}
		handlerLogger.Debug("[%s] %s", update.Message.From.UserName, update.Message.Text)

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
			case "subscribe_all":
				handleSubscribeAll(bot, dbConn, update.Message.Chat.ID)
			case "news":
				handleLatestNewsImproved(bot, dbConn, update.Message.Chat.ID, 10)
			case "tutorial":
				handleTutorial(bot, dbConn, update.Message.Chat.ID)
			case "stats":
				handleAdminStats(bot, dbConn, update.Message.Chat.ID)
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
		handlerLogger.Error("Ошибка добавления пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при подключении к боту.\n\nПожалуйста, попробуйте позже или обратитесь к администратору, если проблема сохраняется.")
		bot.Send(msg)
		return
	}

	handlerLogger.Info("Пользователь %s подключился к боту с chatId %d", user.Username, insertedId)

	// Проверяем, есть ли у пользователя подписки
	subscriptions, err := db.GetUserSubscriptionsWithDetails(dbConn, chatId)
	hasSubscriptions := err == nil && len(subscriptions) > 0

	// Улучшенное приветствие
	welcomeText := `👋 *Добро пожаловать в RSS News Bot!*

Я помогу вам получать свежие новости из ваших любимых источников прямо в Telegram.

*Что я умею:*
📰 Получать новости из RSS-лент
🔔 Отправлять уведомления о новых новостях
📋 Управлять подписками на источники
🔍 Просматривать последние новости

*Быстрый старт:*
1️⃣ Подпишитесь на популярные источники
2️⃣ Или добавьте свой RSS-источник
3️⃣ Получайте новости автоматически!`

	if hasSubscriptions {
		welcomeText += "\n\n✅ У вас уже есть подписки! Используйте кнопки ниже для управления."
	} else {
		welcomeText += "\n\n💡 *Совет:* Нажмите «🚀 Быстрый старт» чтобы подписаться на популярные источники!"
	}

	msg := tgbotapi.NewMessage(chatId, welcomeText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createWelcomeKeyboard(hasSubscriptions)
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
		handlerLogger.Error("Ошибка при добавлении источника: %v", err)
		// Проверяем, существует ли уже источник
		var msg tgbotapi.MessageConfig
		_, existsErr := db.FindSourceActiveByUrl(dbConn, link)
		if existsErr == nil {
			msg = tgbotapi.NewMessage(chatId, "ℹ️ Этот источник уже существует в базе данных.\n\nВы можете подписаться на него через меню «📋 Мои источники».")
		} else {
			msg = tgbotapi.NewMessage(chatId, "❌ Не удалось добавить источник.\n\nВозможные причины:\n• Неверный формат URL\n• Источник недоступен\n• Проблема с подключением\n\nПопробуйте позже или проверьте правильность URL.")
		}
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
		handlerLogger.Error("Ошибка при поиске источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось найти добавленный источник")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Проверяем, существует ли пользователь, если нет - регистрируем его
	exists, err := db.UserExists(dbConn, chatId)
	if err != nil {
		handlerLogger.Error("Ошибка при проверке существования пользователя: %v", err)
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
			handlerLogger.Error("Ошибка при регистрации пользователя: %v", err)
			msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при регистрации пользователя")
			msg.ReplyMarkup = createMainKeyboard()
			bot.Send(msg)
			return
		}
		handlerLogger.Info("Автоматически зарегистрирован пользователь с chatId %d", chatId)
	}

	var subscription = db.Subscription{
		ChatId:   chatId,
		SourceId: source.Id,
	}

	// Проверяем, не подписан ли уже пользователь
	isSubscribed, err := db.IsUserSubscribed(dbConn, chatId, source.Id)
	if err == nil && isSubscribed {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("ℹ️ Вы уже подписаны на источник «%s».\n\nИспользуйте меню «📝 Мои подписки» для управления подписками.", source.Name))
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	err = db.SaveSubscription(dbConn, subscription)
	if err != nil {
		handlerLogger.Error("Ошибка при добавлении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("❌ Не удалось добавить подписку на «%s».\n\nВозможные причины:\n• Подписка уже существует\n• Проблема с базой данных\n\nПопробуйте позже.", source.Name))
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	successMsg := tgbotapi.NewMessage(chatId, fmt.Sprintf("✅ Вы успешно подписались на источник «%s»!\n\nТеперь вы будете получать новости из этого источника автоматически.", source.Name))
	successMsg.ReplyMarkup = createMainKeyboard()
	bot.Send(successMsg)
}

// handleShowSources обрабатывает команду /sources для вывода списка источников
func handleShowSources(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	sources, err := db.FindActiveSources(dbConn)
	if err != nil {
		handlerLogger.Error("Ошибка при получении списка источников: %v", err)
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
		handlerLogger.Error("Ошибка при проверке существования пользователя: %v", err)
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
			handlerLogger.Error("Ошибка при регистрации пользователя: %v", err)
			msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при регистрации пользователя")
			msg.ReplyMarkup = createMainKeyboard()
			bot.Send(msg)
			return
		}
		handlerLogger.Info("Автоматически зарегистрирован пользователь с chatId %d", chatId)
	}

	var subscription = db.Subscription{
		ChatId:   chatId,
		SourceId: sourceIdInt,
	}

	err = db.SaveSubscription(dbConn, subscription)
	if err != nil {
		handlerLogger.Error("Ошибка при добавлении подписки: %v", err)
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
		handlerLogger.Error("Ошибка при удалении подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Не удалось удалить подписку. Возможно, она не существует")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, "✅ Подписка успешно удалена!")
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleSubscribeAll подписывает пользователя на все активные источники
func handleSubscribeAll(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	// Получаем все активные источники
	sources, err := db.FindActiveSources(dbConn)
	if err != nil {
		handlerLogger.Error("Ошибка при получении источников: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении списка источников.\n\nПопробуйте позже или обратитесь к администратору, если проблема сохраняется.")
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

	// Проверяем, существует ли пользователь, если нет - регистрируем его
	exists, err := db.UserExists(dbConn, chatId)
	if err != nil {
		handlerLogger.Error("Ошибка при проверке существования пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке пользователя.\n\nПопробуйте позже или используйте команду /start.")
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
			handlerLogger.Error("Ошибка при регистрации пользователя: %v", err)
			msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при регистрации пользователя.\n\nПожалуйста, используйте команду /start для регистрации.")
			msg.ReplyMarkup = createMainKeyboard()
			bot.Send(msg)
			return
		}
		handlerLogger.Info("Автоматически зарегистрирован пользователь с chatId %d", chatId)
	}

	// Подписываем на все источники
	subscribedCount := 0
	alreadySubscribedCount := 0
	errorsCount := 0

	for _, source := range sources {
		// Проверяем, не подписан ли уже
		isSubscribed, err := db.IsUserSubscribed(dbConn, chatId, source.Id)
		if err != nil {
			handlerLogger.Error("Ошибка при проверке подписки на источник %d: %v", source.Id, err)
			errorsCount++
			continue
		}
		if isSubscribed {
			alreadySubscribedCount++
			continue
		}

		// Добавляем подписку
		subscription := db.Subscription{
			ChatId:   chatId,
			SourceId: source.Id,
		}
		err = db.SaveSubscription(dbConn, subscription)
		if err != nil {
			handlerLogger.Error("Ошибка при добавлении подписки на источник %d: %v", source.Id, err)
			errorsCount++
			continue
		}
		subscribedCount++
	}

	// Формируем сообщение с результатами
	var msgText string
	if subscribedCount > 0 {
		msgText = fmt.Sprintf("✅ Вы успешно подписались на %d источников!", subscribedCount)
		if alreadySubscribedCount > 0 {
			msgText += fmt.Sprintf("\n\nℹ️ Вы уже были подписаны на %d источников.", alreadySubscribedCount)
		}
		if errorsCount > 0 {
			msgText += fmt.Sprintf("\n\n⚠️ Не удалось подписаться на %d источников.", errorsCount)
		}
	} else if alreadySubscribedCount > 0 {
		msgText = fmt.Sprintf("ℹ️ Вы уже подписаны на все %d доступных источников.", alreadySubscribedCount)
	} else {
		msgText = fmt.Sprintf("❌ Не удалось подписаться на источники.\n\nОшибок: %d", errorsCount)
	}

	msg := tgbotapi.NewMessage(chatId, msgText)
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleLatestNewsImproved обрабатывает команду /news с улучшенным форматированием
func handleLatestNewsImproved(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, count int) {
	news, err := db.GetLatestNewsByUser(dbConn, chatId, count)
	if err != nil {
		handlerLogger.Error("Ошибка при получении новостей: %v", err)
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

	message := "📰 *Последние новости:*\n"
	for i, item := range news {
		message += formatMessage(i+1, item.Title, item.Description, item.PublishedAt, item.SourceName)
	}
	// Убираем лишний перенос в конце
	message = strings.TrimRight(message, "\n")

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

*Административные команды:*
/stats - Статистика бота (только для администратора)

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

// handleTutorial запускает интерактивный туториал
func handleTutorial(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	// Начинаем с первого шага
	showTutorialStep(bot, dbConn, chatId, 1)
}

// AdminChatID - ChatID администратора
const AdminChatID int64 = 234501916

// handleAdminStats обрабатывает команду /stats для администратора
func handleAdminStats(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	// Проверяем, является ли пользователь администратором
	if chatId != AdminChatID {
		msg := tgbotapi.NewMessage(chatId, "❌ У вас нет доступа к этой команде.")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Получаем статистику
	stats, err := db.GetAdminStats(dbConn)
	if err != nil {
		handlerLogger.Error("Ошибка при получении статистики: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении статистики.\n\nПопробуйте позже.")
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Форматируем сообщение со статистикой
	statsText := fmt.Sprintf(`📊 *Статистика бота*

📰 *Новости:*
• Всего новостей: %d
• За сегодня: %d
• За вчера: %d

👥 *Пользователи:*
• Всего пользователей: %d`,
		stats.TotalNews,
		stats.NewsToday,
		stats.NewsYesterday,
		stats.TotalUsers,
	)

	msg := tgbotapi.NewMessage(chatId, statsText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// showTutorialStep показывает шаг туториала
func showTutorialStep(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, step int) {
	const totalSteps = 4

	var text string
	switch step {
	case 1:
		text = `📖 *Туториал: Шаг 1 из 4*

*Что такое RSS News Bot?*

Я бот, который помогает получать новости из RSS-лент прямо в Telegram.

*Основные возможности:*
• Подписка на RSS-источники
• Автоматические уведомления о новых новостях
• Просмотр последних новостей
• Управление подписками

Нажмите "Далее" чтобы узнать, как начать работу!`
	case 2:
		text = `📖 *Туториал: Шаг 2 из 4*

*Как подписаться на источники?*

Есть два способа:

1️⃣ *Быстрый старт* - подпишитесь на популярные источники одним нажатием

2️⃣ *Добавить свой источник* - отправьте URL RSS-ленты, например:
   • https://tass.ru/rss/v2.xml
   • https://lenta.ru/rss/google-newsstand/main/

После подписки вы будете получать новости автоматически!`
	case 3:
		text = `📖 *Туториал: Шаг 3 из 4*

*Управление подписками*

• *Мои подписки* - посмотреть все ваши подписки и отписаться
• *Мои источники* - посмотреть все доступные источники
• *Последние новости* - просмотреть последние новости из ваших подписок

Вы можете подписаться на несколько источников и получать новости от всех них!`
	case 4:
		text = `📖 *Туториал: Шаг 4 из 4*

*Готово! 🎉*

Теперь вы знаете, как работать с ботом:

✅ Подписывайтесь на источники
✅ Получайте новости автоматически
✅ Управляйте подписками

*Совет:* Используйте кнопку "🚀 Быстрый старт" чтобы быстро подписаться на популярные источники!

Готовы начать? Нажмите "Завершить"!`
	default:
		text = "Туториал завершен!"
	}

	msg := tgbotapi.NewMessage(chatId, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createTutorialKeyboard(step, totalSteps)
	bot.Send(msg)
}
