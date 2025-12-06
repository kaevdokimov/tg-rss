package monitoring

import (
	"log"
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

// Logger предоставляет структурированное логирование
type Logger struct {
	prefix string
}

// NewLogger создает новый логгер с префиксом
func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

func (l *Logger) log(level LogLevel, levelStr string, format string, args ...interface{}) {
	if level < currentLevel {
		return
	}

	prefix := ""
	if l.prefix != "" {
		prefix = "[" + l.prefix + "] "
	}

	message := prefix + "[" + levelStr + "] " + format
	log.Printf(message, args...)
}

// Debug логирует сообщение уровня DEBUG
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, "DEBUG", format, args...)
}

// Info логирует сообщение уровня INFO
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, "INFO", format, args...)
}

// Warn логирует сообщение уровня WARN
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, "WARN", format, args...)
}

// Error логирует сообщение уровня ERROR
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, "ERROR", format, args...)
}

// Fatal логирует сообщение уровня ERROR и завершает программу
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(ERROR, "FATAL", format, args...)
	os.Exit(1)
}

// Инициализация логирования при импорте пакета
func init() {
	// Можно установить уровень из переменной окружения
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		SetLogLevelFromString(level)
	}
}
