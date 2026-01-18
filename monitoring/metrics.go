package monitoring

import (
	"sync"
	"time"
)

// Metrics собирает метрики приложения
type Metrics struct {
	mu sync.RWMutex

	// RSS метрики
	RSSPollsTotal     int64
	RSSPollsErrors    int64
	RSSItemsProcessed int64

	// Redis метрики
	RedisMessagesProduced int64
	RedisMessagesConsumed int64
	RedisErrors           int64

	// Telegram метрики
	TelegramMessagesSent   int64
	TelegramMessagesErrors int64
	TelegramCommandsTotal  int64

	// Database метрики
	DBQueriesTotal  int64
	DBQueriesErrors int64

	// Circuit Breaker метрики
	CircuitBreakerCalls    map[string]int64
	CircuitBreakerFailures map[string]int64
	CircuitBreakerRejected map[string]int64

	// HTTP метрики
	HTTPRequestsTotal   int64
	HTTPRequestsActive  int64
	HTTPRequestsErrors  int64
	HTTPRequestsTimeout int64

	// Content validation метрики
	ContentValidations      int64
	ContentValidationErrors map[string]int64

	// Database connection метрики
	DBConnectionsOpen     int64
	DBConnectionsIdle     int64
	DBConnectionsInUse    int64
	DBConnectionsWait     int64

	// Время последнего обновления
	LastUpdate time.Time
}

var globalMetrics = &Metrics{
	CircuitBreakerCalls:     make(map[string]int64),
	CircuitBreakerFailures:  make(map[string]int64),
	CircuitBreakerRejected:  make(map[string]int64),
	ContentValidationErrors: make(map[string]int64),
	LastUpdate:              time.Now(),
}

// GetMetrics возвращает текущие метрики
func GetMetrics() *Metrics {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	// Создаем копии map'ов для безопасности
	cbCalls := make(map[string]int64)
	cbFailures := make(map[string]int64)
	cbRejected := make(map[string]int64)

	globalMetrics.mu.RLock()
	for k, v := range globalMetrics.CircuitBreakerCalls {
		cbCalls[k] = v
	}
	for k, v := range globalMetrics.CircuitBreakerFailures {
		cbFailures[k] = v
	}
	for k, v := range globalMetrics.CircuitBreakerRejected {
		cbRejected[k] = v
	}
	// Копируем метрики content validation
	contentValidationErrors := make(map[string]int64)
	for k, v := range globalMetrics.ContentValidationErrors {
		contentValidationErrors[k] = v
	}
	globalMetrics.mu.RUnlock()

	// Возвращаем копию для безопасности
	return &Metrics{
		RSSPollsTotal:          globalMetrics.RSSPollsTotal,
		RSSPollsErrors:         globalMetrics.RSSPollsErrors,
		RSSItemsProcessed:      globalMetrics.RSSItemsProcessed,
		RedisMessagesProduced:  globalMetrics.RedisMessagesProduced,
		RedisMessagesConsumed:  globalMetrics.RedisMessagesConsumed,
		RedisErrors:            globalMetrics.RedisErrors,
		TelegramMessagesSent:   globalMetrics.TelegramMessagesSent,
		TelegramMessagesErrors: globalMetrics.TelegramMessagesErrors,
		TelegramCommandsTotal:  globalMetrics.TelegramCommandsTotal,
		DBQueriesTotal:         globalMetrics.DBQueriesTotal,
		DBQueriesErrors:        globalMetrics.DBQueriesErrors,
		CircuitBreakerCalls:      cbCalls,
		CircuitBreakerFailures:   cbFailures,
		CircuitBreakerRejected:   cbRejected,
		HTTPRequestsTotal:        globalMetrics.HTTPRequestsTotal,
		HTTPRequestsActive:       globalMetrics.HTTPRequestsActive,
		HTTPRequestsErrors:       globalMetrics.HTTPRequestsErrors,
		HTTPRequestsTimeout:      globalMetrics.HTTPRequestsTimeout,
		ContentValidations:       globalMetrics.ContentValidations,
		ContentValidationErrors:  contentValidationErrors,
		DBConnectionsOpen:        globalMetrics.DBConnectionsOpen,
		DBConnectionsIdle:        globalMetrics.DBConnectionsIdle,
		DBConnectionsInUse:       globalMetrics.DBConnectionsInUse,
		DBConnectionsWait:        globalMetrics.DBConnectionsWait,
		LastUpdate:               globalMetrics.LastUpdate,
	}
}

// IncrementRSSPolls увеличивает счетчик опросов RSS
func IncrementRSSPolls() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RSSPollsTotal++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementRSSPollsErrors увеличивает счетчик ошибок опросов RSS
func IncrementRSSPollsErrors() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RSSPollsErrors++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementRSSItemsProcessed увеличивает счетчик обработанных RSS элементов
func IncrementRSSItemsProcessed() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RSSItemsProcessed++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementRedisMessagesProduced увеличивает счетчик отправленных сообщений в Redis
func IncrementRedisMessagesProduced() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RedisMessagesProduced++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementRedisMessagesConsumed увеличивает счетчик потребленных сообщений из Redis
func IncrementRedisMessagesConsumed() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RedisMessagesConsumed++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementRedisErrors увеличивает счетчик ошибок Redis
func IncrementRedisErrors() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RedisErrors++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementTelegramMessagesSent увеличивает счетчик отправленных сообщений в Telegram
func IncrementTelegramMessagesSent() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.TelegramMessagesSent++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementTelegramMessagesErrors увеличивает счетчик ошибок отправки в Telegram
func IncrementTelegramMessagesErrors() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.TelegramMessagesErrors++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementTelegramCommands увеличивает счетчик команд Telegram
func IncrementTelegramCommands() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.TelegramCommandsTotal++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementDBQueries увеличивает счетчик запросов к БД
func IncrementDBQueries() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.DBQueriesTotal++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementDBQueriesErrors увеличивает счетчик ошибок запросов к БД
func IncrementDBQueriesErrors() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.DBQueriesErrors++
	globalMetrics.LastUpdate = time.Now()
}

// Getters для Prometheus-style метрик
func GetRSSPolls() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.RSSPollsTotal
}

func GetRSSPollsErrors() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.RSSPollsErrors
}

func GetRSSItemsProcessed() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.RSSItemsProcessed
}

func GetRedisMessagesProduced() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.RedisMessagesProduced
}

func GetRedisMessagesConsumed() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.RedisMessagesConsumed
}

func GetRedisErrors() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.RedisErrors
}

func GetTelegramMessagesSent() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.TelegramMessagesSent
}

func GetTelegramMessagesErrors() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.TelegramMessagesErrors
}

func GetTelegramCommands() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.TelegramCommandsTotal
}

func GetDBQueries() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.DBQueriesTotal
}

func GetDBQueriesErrors() int64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.DBQueriesErrors
}

// Reset сбрасывает все метрики
// IncrementCircuitBreakerCalls увеличивает счетчик вызовов circuit breaker
func IncrementCircuitBreakerCalls(name string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CircuitBreakerCalls[name]++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementCircuitBreakerFailures увеличивает счетчик ошибок circuit breaker
func IncrementCircuitBreakerFailures(name string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CircuitBreakerFailures[name]++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementCircuitBreakerRejected увеличивает счетчик отклоненных запросов circuit breaker
func IncrementCircuitBreakerRejected(name string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CircuitBreakerRejected[name]++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementHTTPRequestsTotal увеличивает счетчик общих HTTP запросов
func IncrementHTTPRequestsTotal() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.HTTPRequestsTotal++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementHTTPRequestsActive увеличивает счетчик активных HTTP запросов
func IncrementHTTPRequestsActive() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.HTTPRequestsActive++
	globalMetrics.LastUpdate = time.Now()
}

// DecrementHTTPRequestsActive уменьшает счетчик активных HTTP запросов
func DecrementHTTPRequestsActive() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.HTTPRequestsActive--
	globalMetrics.LastUpdate = time.Now()
}

// IncrementHTTPRequestsErrors увеличивает счетчик ошибок HTTP запросов
func IncrementHTTPRequestsErrors() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.HTTPRequestsErrors++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementHTTPRequestsTimeout увеличивает счетчик таймаутов HTTP запросов
func IncrementHTTPRequestsTimeout() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.HTTPRequestsTimeout++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementContentValidations увеличивает счетчик успешных валидаций контента
func IncrementContentValidations() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.ContentValidations++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementContentValidationErrors увеличивает счетчик ошибок валидации контента
func IncrementContentValidationErrors(field string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.ContentValidationErrors[field]++
	globalMetrics.LastUpdate = time.Now()
}

// UpdateDBConnectionMetrics обновляет метрики подключений к БД
func UpdateDBConnectionMetrics(open, idle, inUse, wait int64) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.DBConnectionsOpen = open
	globalMetrics.DBConnectionsIdle = idle
	globalMetrics.DBConnectionsInUse = inUse
	globalMetrics.DBConnectionsWait = wait
	globalMetrics.LastUpdate = time.Now()
}

func Reset() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics = &Metrics{
		CircuitBreakerCalls:     make(map[string]int64),
		CircuitBreakerFailures:  make(map[string]int64),
		CircuitBreakerRejected:  make(map[string]int64),
		ContentValidationErrors: make(map[string]int64),
		LastUpdate:              time.Now(),
	}
}
