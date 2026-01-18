package bot

import (
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleTelegramError обрабатывает ошибки Telegram API и возвращает понятное сообщение для пользователя
func handleTelegramError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Обработка различных типов ошибок Telegram API
	switch {
	case strings.Contains(errStr, "Too Many Requests"):
		// Извлекаем время ожидания, если указано
		if strings.Contains(errStr, "retry after") {
			return "⏳ Превышен лимит запросов к Telegram. Пожалуйста, подождите немного и попробуйте снова."
		}
		return "⏳ Превышен лимит запросов. Пожалуйста, подождите немного."

	case strings.Contains(errStr, "Bad Request"):
		if strings.Contains(errStr, "message is too long") {
			return "❌ Сообщение слишком длинное. Попробуйте другой запрос."
		}
		if strings.Contains(errStr, "can't parse entities") {
			return "❌ Ошибка форматирования сообщения. Попробуйте снова."
		}
		return "❌ Неверный запрос. Проверьте данные и попробуйте снова."

	case strings.Contains(errStr, "Unauthorized"):
		return "❌ Ошибка авторизации. Обратитесь к администратору."

	case strings.Contains(errStr, "Forbidden"):
		if strings.Contains(errStr, "bot was blocked") {
			return "ℹ️ Бот был заблокирован пользователем."
		}
		if strings.Contains(errStr, "chat not found") {
			return "❌ Чат не найден. Убедитесь, что бот добавлен в чат."
		}
		return "❌ Доступ запрещен."

	case strings.Contains(errStr, "Not Found"):
		return "❌ Ресурс не найден."

	case strings.Contains(errStr, "Conflict"):
		return "⚠️ Конфликт данных. Попробуйте позже."

	case strings.Contains(errStr, "Internal Server Error"):
		return "🔧 Временная проблема на стороне Telegram. Попробуйте позже."

	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
		return "⏱️ Превышено время ожидания. Проверьте подключение к интернету и попробуйте снова."

	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection"):
		return "🌐 Проблема с подключением. Проверьте интернет и попробуйте снова."

	default:
		// Для неизвестных ошибок возвращаем общее сообщение
		return "❌ Произошла ошибка. Попробуйте позже или обратитесь к администратору."
	}
}

// sendErrorMessage отправляет понятное сообщение об ошибке пользователю
// TODO: использовать в обработчиках команд при возникновении ошибок
func sendErrorMessage(bot *tgbotapi.BotAPI, chatId int64, err error) {
	errorMsg := handleTelegramError(err)
	if errorMsg == "" {
		errorMsg = "❌ Произошла ошибка. Попробуйте позже."
	}

	msg := tgbotapi.NewMessage(chatId, errorMsg)
	msg.ReplyMarkup = createMainKeyboard()
	bot.Send(msg)
}

// isRateLimitError проверяет, является ли ошибка ошибкой rate limiting
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Too Many Requests")
}

// extractRetryAfter извлекает время ожидания из ошибки rate limiting
// Формат ошибки: "Too Many Requests: retry after 52833"
func extractRetryAfter(err error) int {
	if err == nil {
		return 0
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "retry after") {
		return 0
	}

	// Парсим число из строки "retry after 52833"
	re := regexp.MustCompile(`retry after (\d+)`)
	matches := re.FindStringSubmatch(errStr)
	if len(matches) >= 2 {
		if seconds, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
			return seconds
		}
	}

	return 0
}
