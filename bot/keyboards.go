package bot

import (
	"fmt"
	"net/url"
	"tg-rss/db"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

	// Добавляем кнопку "Назад"
	backRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "main_menu"),
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
			tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "main_menu"),
		),
	)
	return keyboard
}

// createAddSourceKeyboard создает клавиатуру для добавления источника
func createAddSourceKeyboard() tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "main_menu"),
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
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "main_menu"),
	)
	rows = append(rows, backRow)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// createNewsKeyboard создает клавиатуру для отдельной новости
func createNewsKeyboard(link string, _ int64) tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📖 Читать", link),
			tgbotapi.NewInlineKeyboardButtonData("📤 Поделиться", fmt.Sprintf("share_link_%s", link)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Меню", "main_menu"),
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
		tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "main_menu"),
	)
	rows = append(rows, refreshRow)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// createShareKeyboard создает клавиатуру для поделиться новостью
func createShareKeyboard(link, title string) tgbotapi.InlineKeyboardMarkup {
	// URL-кодируем параметры для безопасной передачи
	encodedLink := url.QueryEscape(link)
	encodedTitle := url.QueryEscape(title)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📤 Поделиться в Telegram", fmt.Sprintf("https://t.me/share/url?url=%s&text=%s", encodedLink, encodedTitle)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Скопировать ссылку", fmt.Sprintf("copy_link_%s", link)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_news"),
		),
	)
	return keyboard
}

// createNewsFilterKeyboard создает клавиатуру для фильтрации новостей
func createNewsFilterKeyboard() tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📰 Все новости", "news_all"),
			tgbotapi.NewInlineKeyboardButtonData("⭐ Избранные", "news_favorites"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🕐 Сегодня", "news_today"),
			tgbotapi.NewInlineKeyboardButtonData("📅 Неделя", "news_week"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "main_menu"),
		),
	)
	return keyboard
}
