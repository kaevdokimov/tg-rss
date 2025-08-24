package bot

import (
	"sync"
	"time"
)

// RateLimiter реализует простой rate limiter для отправки сообщений
type RateLimiter struct {
	rates  map[int64]time.Time // chatID -> последнее время отправки
	mu     sync.Mutex
	period time.Duration // минимальный интервал между сообщениями
}

// NewRateLimiter создает новый RateLimiter с указанным периодом
func NewRateLimiter(period time.Duration) *RateLimiter {
	return &RateLimiter{
		rates:  make(map[int64]time.Time),
		period: period,
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
		return true
	}
	
	return false
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
