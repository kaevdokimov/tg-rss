package bot

import (
	"fmt"
	"tg-rss/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// createWelcomeKeyboard создает клавиатуру для приветствия
func createWelcomeKeyboard(hasSubscriptions bool) tgbotapi.InlineKeyboardMarkup {
	if hasSubscriptions {
		// Если есть подписки, показываем обычное меню
		return createMainKeyboard()
	}

	// Для новых пользователей показываем быстрый старт
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚀 Быстрый старт", "quick_start"),
			tgbotapi.NewInlineKeyboardButtonData("📖 Туториал", "tutorial"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📰 Последние новости", "news"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Мои источники", "sources"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❓ Помощь", "help"),
		),
	)
	return keyboard
}

// createMainKeyboard создает основную клавиатуру с главными командами
func createMainKeyboard() tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📰 Последние новости", "news"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Мои источники", "sources"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить источник", "add_source"),
			tgbotapi.NewInlineKeyboardButtonData("📝 Мои подписки", "my_subscriptions"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❓ Помощь", "help"),
		),
	)
	return keyboard
}

// createTutorialKeyboard создает клавиатуру для туториала
func createTutorialKeyboard(step int, totalSteps int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Кнопки навигации по шагам
	var navRow []tgbotapi.InlineKeyboardButton
	if step > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("tutorial_step_%d", step-1)))
	}
	if step < totalSteps {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("Далее ➡️", fmt.Sprintf("tutorial_step_%d", step+1)))
	} else {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("✅ Завершить", "tutorial_complete"))
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	// Кнопка пропуска
	skipRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⏭️ Пропустить туториал", "tutorial_skip"),
	)
	rows = append(rows, skipRow)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// createQuickStartKeyboard создает клавиатуру для быстрого старта
func createQuickStartKeyboard(sources []db.Source, subscribedIds map[int64]bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Показываем первые 5 популярных источников
	maxSources := 5
	if len(sources) < maxSources {
		maxSources = len(sources)
	}

	for i := 0; i < maxSources; i++ {
		source := sources[i]
		isSubscribed := subscribedIds[source.Id]

		var buttonText string
		var callbackData string
		if isSubscribed {
			buttonText = fmt.Sprintf("✅ %s", source.Name)
			callbackData = fmt.Sprintf("quick_unsubscribe_%d", source.Id)
		} else {
			buttonText = fmt.Sprintf("➕ %s", source.Name)
			callbackData = fmt.Sprintf("quick_subscribe_%d", source.Id)
		}

		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		)
		rows = append(rows, row)
	}

	// Кнопка "Подписаться на все"
	if maxSources > 0 {
		allRow := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Подписаться на все", "quick_subscribe_all"),
		)
		rows = append(rows, allRow)
	}

	// Кнопка "Назад"
	backRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "main_menu"),
	)
	rows = append(rows, backRow)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// createSourcesKeyboard создает клавиатуру со списком источников
func createSourcesKeyboard(sources []db.Source) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, source := range sources {
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("📰 %s", source.Name),
				fmt.Sprintf("source_%d", source.Id),
			),
		)
		rows = append(rows, row)
	}

	// Добавляем кнопку "Подписаться на все" если есть источники
	if len(sources) > 0 {
		allRow := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Подписаться на все", "subscribe_all"),
		)
		rows = append(rows, allRow)
	}

	// Добавляем кнопку "Назад"
	backRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🏠", "main_menu"),
	)
	rows = append(rows, backRow)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// createSubscriptionKeyboard создает клавиатуру для управления подпиской на источник
func createSubscriptionKeyboard(sourceId int64, isSubscribed bool) tgbotapi.InlineKeyboardMarkup {
	var action string
	var buttonText string

	if isSubscribed {
		action = "unsubscribe"
		buttonText = "❌ Отписаться"
	} else {
		action = "subscribe"
		buttonText = "✅ Подписаться"
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("%s_%d", action, sourceId)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад к источникам", "sources"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠", "main_menu"),
		),
	)
	return keyboard
}

// createAddSourceKeyboard создает клавиатуру для добавления источника
func createAddSourceKeyboard() tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠", "main_menu"),
		),
	)
	return keyboard
}

// createMySubscriptionsKeyboard создает клавиатуру с подписками пользователя
func createMySubscriptionsKeyboard(subscriptions []db.Subscription, sources []db.Source) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Создаем map для быстрого поиска источников по ID
	sourceMap := make(map[int64]db.Source)
	for _, source := range sources {
		sourceMap[source.Id] = source
	}

	for _, sub := range subscriptions {
		if source, exists := sourceMap[sub.SourceId]; exists {
			row := tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("❌ %s", source.Name),
					fmt.Sprintf("unsubscribe_%d", sub.SourceId),
				),
			)
			rows = append(rows, row)
		}
	}

	// Добавляем кнопку "Назад"
	backRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🏠", "main_menu"),
	)
	rows = append(rows, backRow)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// createNewsKeyboard создает клавиатуру для отдельной новости
func createNewsKeyboard(link string, _ int64) tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📖 Читать", link),
			tgbotapi.NewInlineKeyboardButtonData("🏠", "main_menu"),
		),
	)
	return keyboard
}

// createNewsListKeyboard создает клавиатуру для списка новостей с пагинацией
func createNewsListKeyboard(currentPage, totalPages int, hasMore bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Кнопки навигации
	var navRow []tgbotapi.InlineKeyboardButton

	if currentPage > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("news_page_%d", currentPage-1)))
	}

	if hasMore {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("Вперед ➡️", fmt.Sprintf("news_page_%d", currentPage+1)))
	}

	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	// Информация о странице
	if totalPages > 1 {
		pageRow := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("📄 %d/%d", currentPage, totalPages),
				"page_info",
			),
		)
		rows = append(rows, pageRow)
	}

	// Кнопка обновления
	refreshRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", "news"),
		tgbotapi.NewInlineKeyboardButtonData("🏠", "main_menu"),
	)
	rows = append(rows, refreshRow)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}
