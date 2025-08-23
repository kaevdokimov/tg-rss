package bot

import (
	"fmt"
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
