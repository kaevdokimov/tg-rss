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
	DBConnectionsOpen  int64
	DBConnectionsIdle  int64
	DBConnectionsInUse int64
	DBConnectionsWait  int64

	// Cache метрики
	CacheHits       map[string]int64
	CacheMisses     map[string]int64
	CacheSize       map[string]int64
	CacheEvictions  map[string]int64
	CacheOperations map[string]int64

	// Queue метрики
	QueueSize      map[string]int64
	QueueProcessed map[string]int64
	QueueErrors    map[string]int64
	QueueLatencyMs map[string]int64

	// Rate limiting метрики
	RateLimitHits     map[string]int64
	RateLimitMisses   map[string]int64
	RateLimitRejected map[string]int64

	// Время последнего обновления
	LastUpdate time.Time
}

var globalMetrics = &Metrics{
	CircuitBreakerCalls:     make(map[string]int64),
	CircuitBreakerFailures:  make(map[string]int64),
	CircuitBreakerRejected:  make(map[string]int64),
	ContentValidationErrors: make(map[string]int64),
	CacheHits:               make(map[string]int64),
	CacheMisses:             make(map[string]int64),
	CacheSize:               make(map[string]int64),
	CacheEvictions:          make(map[string]int64),
	CacheOperations:         make(map[string]int64),
	QueueSize:               make(map[string]int64),
	QueueProcessed:          make(map[string]int64),
	QueueErrors:             make(map[string]int64),
	QueueLatencyMs:          make(map[string]int64),
	RateLimitHits:           make(map[string]int64),
	RateLimitMisses:         make(map[string]int64),
	RateLimitRejected:       make(map[string]int64),
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

	// Копируем метрики кэшей
	cacheHits := make(map[string]int64)
	cacheMisses := make(map[string]int64)
	cacheSize := make(map[string]int64)
	cacheEvictions := make(map[string]int64)
	cacheOperations := make(map[string]int64)
	for k, v := range globalMetrics.CacheHits {
		cacheHits[k] = v
	}
	for k, v := range globalMetrics.CacheMisses {
		cacheMisses[k] = v
	}
	for k, v := range globalMetrics.CacheSize {
		cacheSize[k] = v
	}
	for k, v := range globalMetrics.CacheEvictions {
		cacheEvictions[k] = v
	}
	for k, v := range globalMetrics.CacheOperations {
		cacheOperations[k] = v
	}

	// Копируем метрики очередей
	queueSize := make(map[string]int64)
	queueProcessed := make(map[string]int64)
	queueErrors := make(map[string]int64)
	queueLatencyMs := make(map[string]int64)
	for k, v := range globalMetrics.QueueSize {
		queueSize[k] = v
	}
	for k, v := range globalMetrics.QueueProcessed {
		queueProcessed[k] = v
	}
	for k, v := range globalMetrics.QueueErrors {
		queueErrors[k] = v
	}
	for k, v := range globalMetrics.QueueLatencyMs {
		queueLatencyMs[k] = v
	}

	// Копируем метрики rate limiting
	rateLimitHits := make(map[string]int64)
	rateLimitMisses := make(map[string]int64)
	rateLimitRejected := make(map[string]int64)
	for k, v := range globalMetrics.RateLimitHits {
		rateLimitHits[k] = v
	}
	for k, v := range globalMetrics.RateLimitMisses {
		rateLimitMisses[k] = v
	}
	for k, v := range globalMetrics.RateLimitRejected {
		rateLimitRejected[k] = v
	}

	globalMetrics.mu.RUnlock()

	// Возвращаем копию для безопасности
	return &Metrics{
		RSSPollsTotal:           globalMetrics.RSSPollsTotal,
		RSSPollsErrors:          globalMetrics.RSSPollsErrors,
		RSSItemsProcessed:       globalMetrics.RSSItemsProcessed,
		RedisMessagesProduced:   globalMetrics.RedisMessagesProduced,
		RedisMessagesConsumed:   globalMetrics.RedisMessagesConsumed,
		RedisErrors:             globalMetrics.RedisErrors,
		TelegramMessagesSent:    globalMetrics.TelegramMessagesSent,
		TelegramMessagesErrors:  globalMetrics.TelegramMessagesErrors,
		TelegramCommandsTotal:   globalMetrics.TelegramCommandsTotal,
		DBQueriesTotal:          globalMetrics.DBQueriesTotal,
		DBQueriesErrors:         globalMetrics.DBQueriesErrors,
		CircuitBreakerCalls:     cbCalls,
		CircuitBreakerFailures:  cbFailures,
		CircuitBreakerRejected:  cbRejected,
		HTTPRequestsTotal:       globalMetrics.HTTPRequestsTotal,
		HTTPRequestsActive:      globalMetrics.HTTPRequestsActive,
		HTTPRequestsErrors:      globalMetrics.HTTPRequestsErrors,
		HTTPRequestsTimeout:     globalMetrics.HTTPRequestsTimeout,
		ContentValidations:      globalMetrics.ContentValidations,
		ContentValidationErrors: contentValidationErrors,
		DBConnectionsOpen:       globalMetrics.DBConnectionsOpen,
		DBConnectionsIdle:       globalMetrics.DBConnectionsIdle,
		DBConnectionsInUse:      globalMetrics.DBConnectionsInUse,
		DBConnectionsWait:       globalMetrics.DBConnectionsWait,
		CacheHits:               cacheHits,
		CacheMisses:             cacheMisses,
		CacheSize:               cacheSize,
		CacheEvictions:          cacheEvictions,
		CacheOperations:         cacheOperations,
		QueueSize:               queueSize,
		QueueProcessed:          queueProcessed,
		QueueErrors:             queueErrors,
		QueueLatencyMs:          queueLatencyMs,
		RateLimitHits:           rateLimitHits,
		RateLimitMisses:         rateLimitMisses,
		RateLimitRejected:       rateLimitRejected,
		LastUpdate:              globalMetrics.LastUpdate,
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

// Cache метрики
func IncrementCacheHits(cacheName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CacheHits[cacheName]++
	globalMetrics.LastUpdate = time.Now()
}

func IncrementCacheMisses(cacheName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CacheMisses[cacheName]++
	globalMetrics.LastUpdate = time.Now()
}

func UpdateCacheSize(cacheName string, size int64) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CacheSize[cacheName] = size
	globalMetrics.LastUpdate = time.Now()
}

func IncrementCacheEvictions(cacheName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CacheEvictions[cacheName]++
	globalMetrics.LastUpdate = time.Now()
}

func IncrementCacheOperations(cacheName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.CacheOperations[cacheName]++
	globalMetrics.LastUpdate = time.Now()
}

// Queue метрики
func UpdateQueueSize(queueName string, size int64) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.QueueSize[queueName] = size
	globalMetrics.LastUpdate = time.Now()
}

func IncrementQueueProcessed(queueName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.QueueProcessed[queueName]++
	globalMetrics.LastUpdate = time.Now()
}

func IncrementQueueErrors(queueName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.QueueErrors[queueName]++
	globalMetrics.LastUpdate = time.Now()
}

func UpdateQueueLatency(queueName string, latencyMs int64) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	// Экспоненциальное сглаживание для latency
	if current, exists := globalMetrics.QueueLatencyMs[queueName]; exists {
		globalMetrics.QueueLatencyMs[queueName] = (current*9 + latencyMs) / 10
	} else {
		globalMetrics.QueueLatencyMs[queueName] = latencyMs
	}
	globalMetrics.LastUpdate = time.Now()
}

// Rate limiting метрики
func IncrementRateLimitHits(limiterName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RateLimitHits[limiterName]++
	globalMetrics.LastUpdate = time.Now()
}

func IncrementRateLimitMisses(limiterName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RateLimitMisses[limiterName]++
	globalMetrics.LastUpdate = time.Now()
}

func IncrementRateLimitRejected(limiterName string) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RateLimitRejected[limiterName]++
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
		CacheHits:               make(map[string]int64),
		CacheMisses:             make(map[string]int64),
		CacheSize:               make(map[string]int64),
		CacheEvictions:          make(map[string]int64),
		CacheOperations:         make(map[string]int64),
		QueueSize:               make(map[string]int64),
		QueueProcessed:          make(map[string]int64),
		QueueErrors:             make(map[string]int64),
		QueueLatencyMs:          make(map[string]int64),
		RateLimitHits:           make(map[string]int64),
		RateLimitMisses:         make(map[string]int64),
		RateLimitRejected:       make(map[string]int64),
		LastUpdate:              time.Now(),
	}
}
