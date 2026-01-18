package monitoring

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// StructuredLogger предоставляет структурированное логирование с zap
type StructuredLogger struct {
	logger *zap.SugaredLogger
}

// NewStructuredLogger создает новый структурированный логгер
func NewStructuredLogger(component string) *StructuredLogger {
	config := zap.NewProductionConfig()

	// Настройка уровня логирования из переменной окружения
	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case "DEBUG", "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "INFO", "info":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "WARN", "warning":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "ERROR", "error":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// Настройка вывода в JSON для production
	if os.Getenv("ENV") == "production" {
		config.Encoding = "json"
	} else {
		config.Encoding = "console"
	}

	logger, err := config.Build()
	if err != nil {
		// Fallback to basic logger
		logger = zap.NewNop()
	}

	sugar := logger.Sugar().With("component", component)
	return &StructuredLogger{logger: sugar}
}

// Debug логирует сообщение уровня DEBUG
func (l *StructuredLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debugw(msg, keysAndValues...)
}

// Info логирует сообщение уровня INFO
func (l *StructuredLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Infow(msg, keysAndValues...)
}

// Warn логирует сообщение уровня WARN
func (l *StructuredLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warnw(msg, keysAndValues...)
}

// Error логирует сообщение уровня ERROR
func (l *StructuredLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Errorw(msg, keysAndValues...)
}

// Fatal логирует сообщение уровня FATAL и завершает программу
func (l *StructuredLogger) Fatal(msg string, keysAndValues ...interface{}) {
	l.logger.Fatalw(msg, keysAndValues...)
}

// With добавляет постоянные поля к логгеру
func (l *StructuredLogger) With(keysAndValues ...interface{}) *StructuredLogger {
	return &StructuredLogger{
		logger: l.logger.With(keysAndValues...),
	}
}

// Sync синхронизирует буферы логов
func (l *StructuredLogger) Sync() error {
	return l.logger.Sync()
}

// Глобальные логгеры для компонентов
var (
	DefaultLogger = NewStructuredLogger("app")
)

// GetLogger возвращает логгер для указанного компонента
func GetLogger(component string) *StructuredLogger {
	return NewStructuredLogger(component)
}

// LogOperation логирует операцию с ее результатом
func (l *StructuredLogger) LogOperation(operation string, success bool, duration int64, extraFields ...interface{}) {
	fields := []interface{}{
		"operation", operation,
		"success", success,
		"duration_ms", duration,
	}

	// Добавляем дополнительные поля
	fields = append(fields, extraFields...)

	if success {
		l.Info("operation completed", fields...)
	} else {
		l.Error("operation failed", fields...)
	}
}

// LogHTTPRequest логирует HTTP запрос
func (l *StructuredLogger) LogHTTPRequest(method, url, status string, duration int64, size int64) {
	l.Info("http request",
		"method", method,
		"url", url,
		"status", status,
		"duration_ms", duration,
		"response_size", size,
	)
}

// LogDatabaseQuery логирует запрос к БД
func (l *StructuredLogger) LogDatabaseQuery(query string, duration int64, rowsAffected int64, err error) {
	if err != nil {
		l.Error("database query failed",
			"query", query,
			"duration_ms", duration,
			"error", err.Error(),
		)
	} else {
		l.Debug("database query completed",
			"query", query,
			"duration_ms", duration,
			"rows_affected", rowsAffected,
		)
	}
}

// LogCircuitBreakerEvent логирует события circuit breaker
func (l *StructuredLogger) LogCircuitBreakerEvent(name, event string, state string) {
	l.Info("circuit breaker event",
		"circuit_breaker", name,
		"event", event,
		"state", state,
	)
}