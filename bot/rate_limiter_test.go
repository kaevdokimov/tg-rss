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
		t.Error("Первый запрос должен быть разрешен")
	}
	
	// Второй запрос сразу после первого должен быть отклонен
	if limiter.Allow(chatID) {
		t.Error("Второй запрос сразу после первого должен быть отклонен")
	}
	
	// После периода времени запрос должен быть разрешен
	time.Sleep(period + 10*time.Millisecond)
	if !limiter.Allow(chatID) {
		t.Error("Запрос после периода времени должен быть разрешен")
	}
}

func TestRateLimiterDifferentChats(t *testing.T) {
	period := 100 * time.Millisecond
	limiter := NewRateLimiter(period)
	
	chatID1 := int64(11111)
	chatID2 := int64(22222)
	
	// Оба чата должны иметь независимые лимиты
	if !limiter.Allow(chatID1) {
		t.Error("Первый чат должен быть разрешен")
	}
	if !limiter.Allow(chatID2) {
		t.Error("Второй чат должен быть разрешен независимо")
	}
	
	// Первый чат должен быть заблокирован
	if limiter.Allow(chatID1) {
		t.Error("Первый чат должен быть заблокирован")
	}
	
	// Второй чат тоже должен быть заблокирован (так как он тоже только что использовался)
	if limiter.Allow(chatID2) {
		t.Error("Второй чат также должен быть заблокирован после немедленного использования")
	}
	
	// После периода времени оба чата должны быть разрешены
	time.Sleep(period + 10*time.Millisecond)
	if !limiter.Allow(chatID1) {
		t.Error("Первый чат должен быть разрешен после периода")
	}
	if !limiter.Allow(chatID2) {
		t.Error("Второй чат должен быть разрешен после периода")
	}
}
