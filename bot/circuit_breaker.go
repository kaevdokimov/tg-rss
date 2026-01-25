package bot

import (
	"sync"
	"sync/atomic"
	"time"

	"tg-rss/monitoring"
)

// CircuitBreakerState представляет состояние circuit breaker
type CircuitBreakerState int32

const (
	StateClosed   CircuitBreakerState = iota // Нормальная работа
	StateOpen                                // Открыт - блокирует запросы
	StateHalfOpen                            // Полуоткрыт - тестирует восстановление
)

// CircuitBreaker реализует паттерн Circuit Breaker для защиты от каскадных сбоев
type CircuitBreaker struct {
	name string

	// Настройки
	failureThreshold int           // Порог срабатывания (количество последовательных ошибок)
	recoveryTimeout  time.Duration // Время ожидания перед переходом в Half-Open
	resetTimeout     time.Duration // Время ожидания в Half-Open состоянии

	// Состояние
	state       int32 // CircuitBreakerState
	failures    int32 // Текущее количество ошибок
	lastFailure time.Time
	lastAttempt time.Time

	// Синхронизация
	mu sync.RWMutex
}

// NewCircuitBreaker создает новый circuit breaker
func NewCircuitBreaker(name string, failureThreshold int, recoveryTimeout, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		resetTimeout:     resetTimeout,
		state:            int32(StateClosed),
	}
}

// Call выполняет функцию через circuit breaker
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.canExecute() {
		monitoring.IncrementCircuitBreakerRejected(cb.name)
		return ErrCircuitBreakerOpen
	}

	monitoring.IncrementCircuitBreakerCalls(cb.name)
	err := fn()

	if err != nil {
		cb.recordFailure()
		monitoring.IncrementCircuitBreakerFailures(cb.name)
		return err
	}

	cb.recordSuccess()
	return nil
}

// canExecute проверяет, можно ли выполнить запрос
func (cb *CircuitBreaker) canExecute() bool {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		cb.mu.RLock()
		lastFailure := cb.lastFailure
		cb.mu.RUnlock()
		
		if time.Since(lastFailure) >= cb.recoveryTimeout {
			cb.setState(StateHalfOpen)
			return true
		}
		return false
	case StateHalfOpen:
		cb.mu.RLock()
		lastAttempt := cb.lastAttempt
		cb.mu.RUnlock()
		
		return time.Since(lastAttempt) >= cb.resetTimeout
	default:
		return false
	}
}

// recordFailure фиксирует ошибку
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()
	failures := atomic.AddInt32(&cb.failures, 1)

	if failures >= int32(cb.failureThreshold) {
		cb.setState(StateOpen)
	}
}

// recordSuccess фиксирует успешное выполнение
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastAttempt = time.Now()

	if CircuitBreakerState(atomic.LoadInt32(&cb.state)) == StateHalfOpen {
		// Успешное выполнение в Half-Open состоянии - закрываем circuit
		atomic.StoreInt32(&cb.failures, 0)
		cb.setState(StateClosed)
	}
}

// setState устанавливает новое состояние
func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	atomic.StoreInt32(&cb.state, int32(state))
}

// GetState возвращает текущее состояние
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&cb.state))
}

// GetFailures возвращает количество текущих ошибок
func (cb *CircuitBreaker) GetFailures() int32 {
	return atomic.LoadInt32(&cb.failures)
}

// Reset сбрасывает circuit breaker в исходное состояние
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.StoreInt32(&cb.failures, 0)
	atomic.StoreInt32(&cb.state, int32(StateClosed))
}

// ErrCircuitBreakerOpen ошибка, когда circuit breaker открыт
var ErrCircuitBreakerOpen = NewCircuitBreakerError("circuit breaker is open")

// CircuitBreakerError ошибка circuit breaker
type CircuitBreakerError struct {
	Message string
}

func NewCircuitBreakerError(message string) *CircuitBreakerError {
	return &CircuitBreakerError{Message: message}
}

func (e *CircuitBreakerError) Error() string {
	return e.Message
}

// Глобальные circuit breaker'ы для разных типов запросов
var (
	// Circuit breaker для RSS источников
	rssCircuitBreaker = NewCircuitBreaker("rss_sources", 5, 30*time.Second, 5*time.Second)

	// Circuit breaker для скраппинга контента
	scraperCircuitBreaker = NewCircuitBreaker("content_scraper", 3, 60*time.Second, 10*time.Second)

	// Circuit breaker для Telegram API
	telegramCircuitBreaker = NewCircuitBreaker("telegram_api", 10, 120*time.Second, 15*time.Second)
)

// GetRSSCircuitBreaker возвращает circuit breaker для RSS
func GetRSSCircuitBreaker() *CircuitBreaker {
	return rssCircuitBreaker
}

// GetScraperCircuitBreaker возвращает circuit breaker для скраппинга
func GetScraperCircuitBreaker() *CircuitBreaker {
	return scraperCircuitBreaker
}

// GetTelegramCircuitBreaker возвращает circuit breaker для Telegram
func GetTelegramCircuitBreaker() *CircuitBreaker {
	return telegramCircuitBreaker
}
