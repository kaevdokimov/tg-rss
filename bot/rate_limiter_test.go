package bot

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	period := 100 * time.Millisecond
	limiter := NewRateLimiter(period)
	
	chatID := int64(12345)
	
	// Первый запрос должен быть разрешен
	if !limiter.Allow(chatID) {
		t.Error("First request should be allowed")
	}
	
	// Второй запрос сразу после первого должен быть отклонен
	if limiter.Allow(chatID) {
		t.Error("Second request immediately after first should be denied")
	}
	
	// После периода времени запрос должен быть разрешен
	time.Sleep(period + 10*time.Millisecond)
	if !limiter.Allow(chatID) {
		t.Error("Request after period should be allowed")
	}
}

func TestRateLimiterDifferentChats(t *testing.T) {
	period := 100 * time.Millisecond
	limiter := NewRateLimiter(period)
	
	chatID1 := int64(11111)
	chatID2 := int64(22222)
	
	// Оба чата должны иметь независимые лимиты
	if !limiter.Allow(chatID1) {
		t.Error("First chat should be allowed")
	}
	if !limiter.Allow(chatID2) {
		t.Error("Second chat should be allowed independently")
	}
	
	// Первый чат должен быть заблокирован
	if limiter.Allow(chatID1) {
		t.Error("First chat should be blocked")
	}
	
	// Второй чат тоже должен быть заблокирован (так как он тоже только что использовался)
	if limiter.Allow(chatID2) {
		t.Error("Second chat should also be blocked after immediate use")
	}
	
	// После периода времени оба чата должны быть разрешены
	time.Sleep(period + 10*time.Millisecond)
	if !limiter.Allow(chatID1) {
		t.Error("First chat should be allowed after period")
	}
	if !limiter.Allow(chatID2) {
		t.Error("Second chat should be allowed after period")
	}
}
