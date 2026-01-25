package bot

import (
	"sync"
	"time"

	"tg-rss/monitoring"
)

// RateLimiter реализует простой rate limiter для отправки сообщений
type RateLimiter struct {
	rates  map[int64]time.Time // chatID -> последнее время отправки
	mu     sync.Mutex
	period time.Duration // минимальный интервал между сообщениями
}

// GlobalRateLimiter реализует глобальный rate limiter для всех сообщений
// Защищает от превышения глобальных лимитов Telegram API (30 сообщений/секунду)
type GlobalRateLimiter struct {
	lastSent    time.Time
	mu          sync.Mutex
	minInterval time.Duration // минимальный интервал между любыми сообщениями
}

// NewRateLimiter создает новый RateLimiter с указанным периодом
func NewRateLimiter(period time.Duration) *RateLimiter {
	rl := &RateLimiter{
		rates:  make(map[int64]time.Time),
		period: period,
	}

	// Запускаем автоматическую очистку каждые 10 минут
	go rl.startPeriodicCleanup()

	return rl
}

// startPeriodicCleanup запускает периодическую очистку устаревших записей
func (r *RateLimiter) startPeriodicCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Удаляем записи старше 1 часа
		r.Cleanup(1 * time.Hour)
	}
}

// NewGlobalRateLimiter создает новый глобальный rate limiter
// Используется для защиты от превышения глобальных лимитов Telegram API
func NewGlobalRateLimiter(minInterval time.Duration) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		lastSent:    time.Time{},
		minInterval: minInterval,
	}
}

// Allow проверяет, можно ли отправить сообщение пользователю
func (r *RateLimiter) Allow(chatID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	last, exists := r.rates[chatID]

	if !exists || now.Sub(last) >= r.period {
		r.rates[chatID] = now
		monitoring.IncrementRateLimitHits("user_rate_limiter")
		return true
	}

	monitoring.IncrementRateLimitRejected("user_rate_limiter")
	return false
}

// AllowGlobal проверяет, можно ли отправить сообщение с учетом глобальных лимитов
func (gr *GlobalRateLimiter) AllowGlobal() bool {
	gr.mu.Lock()
	defer gr.mu.Unlock()

	now := time.Now()
	if gr.lastSent.IsZero() || now.Sub(gr.lastSent) >= gr.minInterval {
		gr.lastSent = now
		monitoring.IncrementRateLimitHits("global_rate_limiter")
		return true
	}

	monitoring.IncrementRateLimitRejected("global_rate_limiter")
	return false
}

// SetMinInterval устанавливает минимальный интервал между сообщениями
func (gr *GlobalRateLimiter) SetMinInterval(interval time.Duration) {
	gr.mu.Lock()
	defer gr.mu.Unlock()
	gr.minInterval = interval
}

// GetMinInterval возвращает текущий минимальный интервал
func (gr *GlobalRateLimiter) GetMinInterval() time.Duration {
	gr.mu.Lock()
	defer gr.mu.Unlock()
	return gr.minInterval
}

// Cleanup удаляет устаревшие записи
func (r *RateLimiter) Cleanup(maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for chatID, lastSeen := range r.rates {
		if now.Sub(lastSeen) > maxAge {
			delete(r.rates, chatID)
		}
	}
}
