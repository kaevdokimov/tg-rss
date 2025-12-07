package bot

import (
	"errors"
	"testing"
)

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "rate limit error",
			err:      errors.New("Too Many Requests: retry after 52833"),
			expected: true,
		},
		{
			name:     "rate limit error without retry after",
			err:      errors.New("Too Many Requests"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("Bad Request"),
			expected: false,
		},
		{
			name:     "error with 'too many' but not rate limit",
			err:      errors.New("too many items in list"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRateLimitError(tt.err)
			if result != tt.expected {
				t.Errorf("isRateLimitError(%v) = %v, ожидалось %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestExtractRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: 0,
		},
		{
			name:     "error with retry after",
			err:      errors.New("Too Many Requests: retry after 52833"),
			expected: 52833,
		},
		{
			name:     "error with small retry after",
			err:      errors.New("Too Many Requests: retry after 5"),
			expected: 5,
		},
		{
			name:     "error with retry after 0",
			err:      errors.New("Too Many Requests: retry after 0"),
			expected: 0,
		},
		{
			name:     "error without retry after",
			err:      errors.New("Too Many Requests"),
			expected: 0,
		},
		{
			name:     "error with invalid retry after",
			err:      errors.New("Too Many Requests: retry after abc"),
			expected: 0,
		},
		{
			name:     "other error",
			err:      errors.New("Bad Request"),
			expected: 0,
		},
		{
			name:     "error with retry after in different format",
			err:      errors.New("retry after 100 seconds"),
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRetryAfter(tt.err)
			if result != tt.expected {
				t.Errorf("extractRetryAfter(%v) = %d, ожидалось %d", tt.err, result, tt.expected)
			}
		})
	}
}

func TestHandleTelegramError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "rate limit error with retry after",
			err:      errors.New("Too Many Requests: retry after 52833"),
			expected: "⏳ Превышен лимит запросов к Telegram. Пожалуйста, подождите немного и попробуйте снова.",
		},
		{
			name:     "rate limit error without retry after",
			err:      errors.New("Too Many Requests"),
			expected: "⏳ Превышен лимит запросов. Пожалуйста, подождите немного.",
		},
		{
			name:     "bad request - message too long",
			err:      errors.New("Bad Request: message is too long"),
			expected: "❌ Сообщение слишком длинное. Попробуйте другой запрос.",
		},
		{
			name:     "bad request - parse entities",
			err:      errors.New("Bad Request: can't parse entities"),
			expected: "❌ Ошибка форматирования сообщения. Попробуйте снова.",
		},
		{
			name:     "bad request generic",
			err:      errors.New("Bad Request"),
			expected: "❌ Неверный запрос. Проверьте данные и попробуйте снова.",
		},
		{
			name:     "unauthorized",
			err:      errors.New("Unauthorized"),
			expected: "❌ Ошибка авторизации. Обратитесь к администратору.",
		},
		{
			name:     "forbidden - bot blocked",
			err:      errors.New("Forbidden: bot was blocked"),
			expected: "ℹ️ Бот был заблокирован пользователем.",
		},
		{
			name:     "forbidden - chat not found",
			err:      errors.New("Forbidden: chat not found"),
			expected: "❌ Чат не найден. Убедитесь, что бот добавлен в чат.",
		},
		{
			name:     "forbidden generic",
			err:      errors.New("Forbidden"),
			expected: "❌ Доступ запрещен.",
		},
		{
			name:     "not found",
			err:      errors.New("Not Found"),
			expected: "❌ Ресурс не найден.",
		},
		{
			name:     "conflict",
			err:      errors.New("Conflict"),
			expected: "⚠️ Конфликт данных. Попробуйте позже.",
		},
		{
			name:     "internal server error",
			err:      errors.New("Internal Server Error"),
			expected: "🔧 Временная проблема на стороне Telegram. Попробуйте позже.",
		},
		{
			name:     "timeout",
			err:      errors.New("timeout"),
			expected: "⏱️ Превышено время ожидания. Проверьте подключение к интернету и попробуйте снова.",
		},
		{
			name:     "deadline",
			err:      errors.New("deadline exceeded"),
			expected: "⏱️ Превышено время ожидания. Проверьте подключение к интернету и попробуйте снова.",
		},
		{
			name:     "network error",
			err:      errors.New("network error"),
			expected: "🌐 Проблема с подключением. Проверьте интернет и попробуйте снова.",
		},
		{
			name:     "connection error",
			err:      errors.New("connection refused"),
			expected: "🌐 Проблема с подключением. Проверьте интернет и попробуйте снова.",
		},
		{
			name:     "unknown error",
			err:      errors.New("unknown error type"),
			expected: "❌ Произошла ошибка. Попробуйте позже или обратитесь к администратору.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleTelegramError(tt.err)
			if result != tt.expected {
				t.Errorf("handleTelegramError(%v) = %q, ожидалось %q", tt.err, result, tt.expected)
			}
		})
	}
}
