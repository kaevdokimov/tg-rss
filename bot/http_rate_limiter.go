package bot

import (
	"net/http"
	"sync"
	"time"

	"tg-rss/monitoring"
)

// HTTPRateLimiter ограничивает количество одновременных HTTP запросов
type HTTPRateLimiter struct {
	semaphore chan struct{} // Семaphore для ограничения одновременных запросов
	mu        sync.RWMutex
}

// NewHTTPRateLimiter создает новый HTTP rate limiter
func NewHTTPRateLimiter(maxConcurrent int) *HTTPRateLimiter {
	return &HTTPRateLimiter{
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

// Do выполняет HTTP запрос с rate limiting
func (rl *HTTPRateLimiter) Do(client *http.Client, req *http.Request) (*http.Response, error) {
	monitoring.IncrementHTTPRequestsTotal()

	// Захватываем семафор
	select {
	case rl.semaphore <- struct{}{}:
		// Успешно захватили слот
		monitoring.IncrementHTTPRequestsActive()
		defer func() {
			<-rl.semaphore // Освобождаем слот
			monitoring.DecrementHTTPRequestsActive()
		}()
	case <-time.After(30 * time.Second):
		// Таймаут ожидания слота
		monitoring.IncrementHTTPRequestsTimeout()
		return nil, ErrHTTPRateLimitTimeout
	}

	// Выполняем запрос
	resp, err := client.Do(req)
	if err != nil {
		monitoring.IncrementHTTPRequestsErrors()
		return nil, err
	}

	return resp, nil
}

// GlobalHTTPRateLimiter глобальный rate limiter для HTTP запросов
var GlobalHTTPRateLimiter = NewHTTPRateLimiter(50) // Максимум 50 одновременных запросов

// ErrHTTPRateLimitTimeout ошибка таймаута rate limiter
var ErrHTTPRateLimitTimeout = NewHTTPRateLimitError("HTTP rate limit timeout")

// HTTPRateLimitError ошибка HTTP rate limiter
type HTTPRateLimitError struct {
	Message string
}

func NewHTTPRateLimitError(message string) *HTTPRateLimitError {
	return &HTTPRateLimitError{Message: message}
}

func (e *HTTPRateLimitError) Error() string {
	return e.Message
}

// Метрики для HTTP rate limiter
// Добавляем в monitoring/metrics.go новые поля для HTTP метрик

// В monitoring/metrics.go нужно добавить:
// HTTPRequestsTotal    int64
// HTTPRequestsActive   int64
// HTTPRequestsErrors   int64
// HTTPRequestsTimeout  int64

// Функции для инкремента/декремента HTTP метрик
func IncrementHTTPRequestsTotal() {
	// Реализация в monitoring/metrics.go
}

func IncrementHTTPRequestsActive() {
	// Реализация в monitoring/metrics.go
}

func DecrementHTTPRequestsActive() {
	// Реализация в monitoring/metrics.go
}

func IncrementHTTPRequestsErrors() {
	// Реализация в monitoring/metrics.go
}

func IncrementHTTPRequestsTimeout() {
	// Реализация в monitoring/metrics.go
}