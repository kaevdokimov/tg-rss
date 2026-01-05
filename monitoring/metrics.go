package monitoring

import (
	"sync"
	"time"
)

// Metrics собирает метрики приложения
type Metrics struct {
	mu sync.RWMutex

	// RSS метрики
	RSSPollsTotal      int64
	RSSPollsErrors     int64
	RSSItemsProcessed int64

	// Redis метрики
	RedisMessagesProduced int64
	RedisMessagesConsumed  int64
	RedisErrors            int64

	// Telegram метрики
	TelegramMessagesSent    int64
	TelegramMessagesErrors  int64
	TelegramCommandsTotal   int64

	// Database метрики
	DBQueriesTotal  int64
	DBQueriesErrors int64

	// Время последнего обновления
	LastUpdate time.Time
}

var globalMetrics = &Metrics{
	LastUpdate: time.Now(),
}

// GetMetrics возвращает текущие метрики
func GetMetrics() *Metrics {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	
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
		LastUpdate:             globalMetrics.LastUpdate,
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
func Reset() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics = &Metrics{
		LastUpdate: time.Now(),
	}
}
