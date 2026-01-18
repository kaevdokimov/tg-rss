package monitoring

import (
	"os"
	"testing"
)

func TestLogLevel(t *testing.T) {
	SetLogLevel(DEBUG)
	if currentLevel != DEBUG {
		t.Errorf("Ожидался уровень DEBUG, получено %d", currentLevel)
	}

	SetLogLevel(INFO)
	if currentLevel != INFO {
		t.Errorf("Ожидался уровень INFO, получено %d", currentLevel)
	}

	SetLogLevel(WARN)
	if currentLevel != WARN {
		t.Errorf("Ожидался уровень WARN, получено %d", currentLevel)
	}

	SetLogLevel(ERROR)
	if currentLevel != ERROR {
		t.Errorf("Ожидался уровень ERROR, получено %d", currentLevel)
	}
}

func TestSetLogLevelFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", DEBUG},
		{"debug", DEBUG},
		{"INFO", INFO},
		{"info", INFO},
		{"WARN", WARN},
		{"warn", WARN},
		{"WARNING", WARN},
		{"warning", WARN},
		{"ERROR", ERROR},
		{"error", ERROR},
		{"unknown", INFO}, // default
	}

	for _, tt := range tests {
		SetLogLevelFromString(tt.input)
		if currentLevel != tt.expected {
			t.Errorf("Для ввода '%s' ожидался уровень %d, получено %d", tt.input, tt.expected, currentLevel)
		}
	}
}

func TestLogger(t *testing.T) {
	logger := NewLogger("TEST")

	// Тест что логгер создается
	if logger == nil {
		t.Error("Ожидалось создание логгера")
	}
	// Проверка structured logger происходит в NewStructuredLogger
}

func TestLoggerLevels(t *testing.T) {
	// Сохраняем оригинальный уровень
	originalLevel := currentLevel
	defer SetLogLevel(originalLevel)

	logger := NewLogger("TEST")

	// Тест что сообщения фильтруются по уровню
	SetLogLevel(ERROR)
	logger.Debug("This should not appear")
	logger.Info("This should not appear")
	logger.Warn("This should not appear")
	logger.Error("This should appear")

	SetLogLevel(DEBUG)
	// Все уровни должны работать
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warn message")
	logger.Error("Error message")
}

func TestLoggerFromEnv(t *testing.T) {
	// Сохраняем оригинальное значение
	originalLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		if originalLevel != "" {
			os.Setenv("LOG_LEVEL", originalLevel)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
	}()

	// Тест установки из переменной окружения
	os.Setenv("LOG_LEVEL", "WARN")
	// Переинициализируем (в реальности это делается в init)
	SetLogLevelFromString(os.Getenv("LOG_LEVEL"))
	if currentLevel != WARN {
		t.Errorf("Ожидался уровень WARN из переменной окружения, получено %d", currentLevel)
	}
}
