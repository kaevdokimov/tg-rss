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
	case data == "quick_start":
		handleQuickStart(bot, dbConn, chatId)
	case data == "tutorial":
		handleTutorial(bot, dbConn, chatId)
	case data == "tutorial_skip":
		handleTutorialSkip(bot, chatId)
	case data == "tutorial_complete":
		handleTutorialComplete(bot, chatId)
	case strings.HasPrefix(data, "tutorial_step_"):
		handleTutorialStep(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "quick_subscribe_"):
		handleQuickSubscribe(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "quick_unsubscribe_"):
		handleQuickUnsubscribe(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "source_"):
		handleSourceDetails(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "subscribe_"):
		handleSubscribe(bot, dbConn, chatId, data)
	case strings.HasPrefix(data, "unsubscribe_"):
		handleUnsubscribe(bot, dbConn, chatId, data)
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
	msg := tgbotapi.NewMessage(chatId, "🏠\n\nВыберите действие:")
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
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении списка источников.\n\nПопробуйте позже или обратитесь к администратору, если проблема сохраняется.")
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
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке статуса подписки.\n\nПопробуйте позже.")
		bot.Send(msg)
		return
	}

	var statusText string
	if isSubscribed {
		statusText = "✅ *Статус:* Вы подписаны на этот источник\n\nВы получаете новости из этого источника автоматически."
	} else {
		statusText = "❌ *Статус:* Вы не подписаны на этот источник\n\nНажмите кнопку «Подписаться» чтобы начать получать новости."
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
	source, err := db.FindActiveSourceById(dbConn, sourceId)
	if err != nil {
		log.Printf("Ошибка при поиске источника: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Источник не найден.\n\nВозможно, он был удален или деактивирован.")
		bot.Send(msg)
		return
	}

	// Проверяем, не подписан ли уже пользователь
	isSubscribed, err := db.IsUserSubscribed(dbConn, chatId, sourceId)
	if err != nil {
		log.Printf("Ошибка при проверке подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке подписки.\n\nПопробуйте позже.")
		bot.Send(msg)
		return
	}

	if isSubscribed {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("ℹ️ Вы уже подписаны на источник «%s».\n\nИспользуйте меню «📝 Мои подписки» для управления подписками.", source.Name))
		bot.Send(msg)
		return
	}

	// Проверяем, существует ли пользователь, если нет - регистрируем его
	exists, err := db.UserExists(dbConn, chatId)
	if err != nil {
		log.Printf("Ошибка при проверке существования пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке пользователя.\n\nПопробуйте позже или используйте команду /start.")
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
			msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при регистрации пользователя.\n\nПожалуйста, используйте команду /start для регистрации.")
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
		// Проверяем, не существует ли уже подписка
		var msg tgbotapi.MessageConfig
		isSubscribedCheck, _ := db.IsUserSubscribed(dbConn, chatId, sourceId)
		if isSubscribedCheck {
			msg = tgbotapi.NewMessage(chatId, fmt.Sprintf("ℹ️ Вы уже подписаны на источник «%s».", source.Name))
		} else {
			msg = tgbotapi.NewMessage(chatId, fmt.Sprintf("❌ Не удалось добавить подписку на «%s».\n\nПопробуйте позже.", source.Name))
		}
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("✅ Вы успешно подписались на источник «%s»!\n\nТеперь вы будете получать новости из этого источника автоматически.", source.Name))
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

	// Проверяем, подписан ли пользователь
	isSubscribed, err := db.IsUserSubscribed(dbConn, chatId, sourceId)
	if err != nil {
		log.Printf("Ошибка при проверке подписки: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при проверке подписки.\n\nПопробуйте позже.")
		bot.Send(msg)
		return
	}

	if !isSubscribed {
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("ℹ️ Вы не подписаны на источник «%s».\n\nИспользуйте меню «📋 Мои источники» чтобы подписаться.", source.Name))
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
		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("❌ Не удалось отписаться от источника «%s».\n\nПопробуйте позже.", source.Name))
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("✅ Вы отписались от источника «%s».\n\nВы больше не будете получать новости из этого источника.", source.Name))
	bot.Send(msg)
}

// handleUnknownCallback обрабатывает неизвестные callback-запросы
func handleUnknownCallback(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "❓ Неизвестная команда")
	msg.ReplyMarkup = createMainKeyboard()
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

// handleQuickStart обрабатывает быстрый старт - подписка на популярные источники
func handleQuickStart(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64) {
	// Получаем все активные источники
	sources, err := db.FindActiveSources(dbConn)
	if err != nil {
		log.Printf("Ошибка при получении источников: %v", err)
		msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении источников")
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

	// Получаем текущие подписки пользователя
	subscriptions, err := db.GetUserSubscriptionsWithDetails(dbConn, chatId)
	subscribedIds := make(map[int64]bool)
	if err == nil {
		for _, sub := range subscriptions {
			subscribedIds[sub.SourceId] = true
		}
	}

	msgText := `🚀 *Быстрый старт*

Выберите популярные источники, на которые хотите подписаться:

*Доступные источники:*`
	
	msg := tgbotapi.NewMessage(chatId, msgText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createQuickStartKeyboard(sources, subscribedIds)
	bot.Send(msg)
}

// handleQuickSubscribe обрабатывает подписку через быстрый старт
func handleQuickSubscribe(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		handleUnknownCallback(bot, chatId)
		return
	}

	if parts[2] == "all" {
		// Подписка на все источники
		sources, err := db.FindActiveSources(dbConn)
		if err != nil {
			log.Printf("Ошибка при получении источников: %v", err)
			msg := tgbotapi.NewMessage(chatId, "❌ Ошибка при получении источников")
			bot.Send(msg)
			return
		}

		subscribedCount := 0
		for _, source := range sources {
			// Проверяем, не подписан ли уже
			isSubscribed, err := db.IsUserSubscribed(dbConn, chatId, source.Id)
			if err != nil || isSubscribed {
				continue
			}

			// Проверяем существование пользователя
			exists, err := db.UserExists(dbConn, chatId)
			if err == nil && !exists {
				user := db.User{
					ChatId:   chatId,
					Username: "unknown",
				}
				db.SaveUser(dbConn, user)
			}

			subscription := db.Subscription{
				ChatId:   chatId,
				SourceId: source.Id,
			}
			if err := db.SaveSubscription(dbConn, subscription); err == nil {
				subscribedCount++
			}
		}

		msg := tgbotapi.NewMessage(chatId, fmt.Sprintf("✅ Вы успешно подписались на %d источников!", subscribedCount))
		msg.ReplyMarkup = createMainKeyboard()
		bot.Send(msg)
		return
	}

	// Подписка на один источник
	sourceId, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		handleUnknownCallback(bot, chatId)
		return
	}

	// Используем существующую логику подписки
	handleSubscribe(bot, dbConn, chatId, fmt.Sprintf("subscribe_%d", sourceId))
	
	// Обновляем клавиатуру быстрого старта
	handleQuickStart(bot, dbConn, chatId)
}

// handleQuickUnsubscribe обрабатывает отписку через быстрый старт
func handleQuickUnsubscribe(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) != 3 {
		handleUnknownCallback(bot, chatId)
		return
	}

	sourceId, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		handleUnknownCallback(bot, chatId)
		return
	}

	// Используем существующую логику отписки
	handleUnsubscribe(bot, dbConn, chatId, fmt.Sprintf("unsubscribe_%d", sourceId))
	
	// Обновляем клавиатуру быстрого старта
	handleQuickStart(bot, dbConn, chatId)
}

// handleTutorialStep обрабатывает переход к шагу туториала
func handleTutorialStep(bot *tgbotapi.BotAPI, dbConn *sql.DB, chatId int64, data string) {
	parts := strings.Split(data, "_")
	if len(parts) != 3 {
		handleUnknownCallback(bot, chatId)
		return
	}

	step, err := strconv.Atoi(parts[2])
	if err != nil {
		handleUnknownCallback(bot, chatId)
		return
	}

	showTutorialStep(bot, dbConn, chatId, step)
}

// handleTutorialSkip пропускает туториал
func handleTutorialSkip(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "✅ Туториал пропущен.\n\nИспользуйте кнопки меню для навигации. Если нужна помощь, нажмите /help")
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// handleTutorialComplete завершает туториал
func handleTutorialComplete(bot *tgbotapi.BotAPI, chatId int64) {
	msg := tgbotapi.NewMessage(chatId, "🎉 *Туториал завершен!*\n\nТеперь вы готовы использовать бота. Нажмите «🚀 Быстрый старт» чтобы подписаться на популярные источники, или используйте меню для других действий.")
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}
