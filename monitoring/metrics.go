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

	// Kafka метрики
	KafkaMessagesProduced int64
	KafkaMessagesConsumed  int64
	KafkaErrors            int64

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
		KafkaMessagesProduced:  globalMetrics.KafkaMessagesProduced,
		KafkaMessagesConsumed:  globalMetrics.KafkaMessagesConsumed,
		KafkaErrors:            globalMetrics.KafkaErrors,
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

// IncrementKafkaMessagesProduced увеличивает счетчик отправленных сообщений в Kafka
func IncrementKafkaMessagesProduced() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.KafkaMessagesProduced++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementKafkaMessagesConsumed увеличивает счетчик потребленных сообщений из Kafka
func IncrementKafkaMessagesConsumed() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.KafkaMessagesConsumed++
	globalMetrics.LastUpdate = time.Now()
}

// IncrementKafkaErrors увеличивает счетчик ошибок Kafka
func IncrementKafkaErrors() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.KafkaErrors++
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

// Reset сбрасывает все метрики
func Reset() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics = &Metrics{
		LastUpdate: time.Now(),
	}
}
