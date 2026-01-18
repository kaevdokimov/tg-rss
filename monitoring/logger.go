package monitoring

import (
	"os"
)

// LogLevel определяет уровень логирования
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var currentLevel LogLevel = INFO

// SetLogLevel устанавливает уровень логирования
func SetLogLevel(level LogLevel) {
	currentLevel = level
}

// SetLogLevelFromString устанавливает уровень логирования из строки
func SetLogLevelFromString(level string) {
	switch level {
	case "DEBUG", "debug":
		currentLevel = DEBUG
	case "INFO", "info":
		currentLevel = INFO
	case "WARN", "warn", "WARNING", "warning":
		currentLevel = WARN
	case "ERROR", "error":
		currentLevel = ERROR
	default:
		currentLevel = INFO
	}
}

// Logger предоставляет структурированное логирование (legacy wrapper)
type Logger struct {
	structuredLogger *StructuredLogger
}

// NewLogger создает новый логгер с префиксом (legacy wrapper для совместимости)
func NewLogger(prefix string) *Logger {
	return &Logger{
		structuredLogger: NewStructuredLogger(prefix),
	}
}

// Debug логирует сообщение уровня DEBUG
func (l *Logger) Debug(format string, args ...interface{}) {
	if DEBUG >= currentLevel {
		l.structuredLogger.Debug(format, args...)
	}
}

// Info логирует сообщение уровня INFO
func (l *Logger) Info(format string, args ...interface{}) {
	if INFO >= currentLevel {
		l.structuredLogger.Info(format, args...)
	}
}

// Warn логирует сообщение уровня WARN
func (l *Logger) Warn(format string, args ...interface{}) {
	if WARN >= currentLevel {
		l.structuredLogger.Warn(format, args...)
	}
}

// Error логирует сообщение уровня ERROR
func (l *Logger) Error(format string, args ...interface{}) {
	if ERROR >= currentLevel {
		l.structuredLogger.Error(format, args...)
	}
}

// Fatal логирует сообщение уровня ERROR и завершает программу
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.structuredLogger.Fatal(format, args...)
}

// Инициализация логирования при импорте пакета
func init() {
	// Можно установить уровень из переменной окружения
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		SetLogLevelFromString(level)
	}
}
