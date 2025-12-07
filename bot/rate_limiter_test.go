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

func TestRateLimiterCleanup(t *testing.T) {
	period := 50 * time.Millisecond
	limiter := NewRateLimiter(period)
	
	chatID1 := int64(11111)
	chatID2 := int64(22222)
	chatID3 := int64(33333)
	
	// Используем все чаты
	limiter.Allow(chatID1)
	limiter.Allow(chatID2)
	limiter.Allow(chatID3)
	
	// Проверяем, что все чаты заблокированы
	if limiter.Allow(chatID1) {
		t.Error("Первый чат должен быть заблокирован")
	}
	if limiter.Allow(chatID2) {
		t.Error("Второй чат должен быть заблокирован")
	}
	if limiter.Allow(chatID3) {
		t.Error("Третий чат должен быть заблокирован")
	}
	
	// Ждем, чтобы записи стали устаревшими
	time.Sleep(60 * time.Millisecond)
	
	// Очищаем устаревшие записи (maxAge меньше времени ожидания)
	limiter.Cleanup(10 * time.Millisecond)
	
	// После очистки все чаты должны быть доступны
	if !limiter.Allow(chatID1) {
		t.Error("Первый чат должен быть доступен после очистки")
	}
	if !limiter.Allow(chatID2) {
		t.Error("Второй чат должен быть доступен после очистки")
	}
	if !limiter.Allow(chatID3) {
		t.Error("Третий чат должен быть доступен после очистки")
	}
}

func TestGlobalRateLimiter(t *testing.T) {
	minInterval := 100 * time.Millisecond
	limiter := NewGlobalRateLimiter(minInterval)
	
	// Первый запрос должен быть разрешен
	if !limiter.AllowGlobal() {
		t.Error("Первый запрос должен быть разрешен")
	}
	
	// Второй запрос сразу после первого должен быть отклонен
	if limiter.AllowGlobal() {
		t.Error("Второй запрос сразу после первого должен быть отклонен")
	}
	
	// После периода времени запрос должен быть разрешен
	time.Sleep(minInterval + 10*time.Millisecond)
	if !limiter.AllowGlobal() {
		t.Error("Запрос после периода времени должен быть разрешен")
	}
}

func TestGlobalRateLimiterSetMinInterval(t *testing.T) {
	minInterval := 50 * time.Millisecond
	limiter := NewGlobalRateLimiter(minInterval)
	
	// Проверяем начальный интервал
	if limiter.GetMinInterval() != minInterval {
		t.Errorf("Ожидался начальный интервал %v, получено %v", minInterval, limiter.GetMinInterval())
	}
	
	// Устанавливаем новый интервал
	newInterval := 200 * time.Millisecond
	limiter.SetMinInterval(newInterval)
	
	// Проверяем, что интервал изменился
	if limiter.GetMinInterval() != newInterval {
		t.Errorf("Ожидался интервал %v, получено %v", newInterval, limiter.GetMinInterval())
	}
	
	// Используем лимитер
	if !limiter.AllowGlobal() {
		t.Error("Первый запрос должен быть разрешен")
	}
	
	// С новым интервалом запрос должен быть отклонен
	if limiter.AllowGlobal() {
		t.Error("Запрос должен быть отклонен с новым интервалом")
	}
	
	// После нового интервала запрос должен быть разрешен
	time.Sleep(newInterval + 10*time.Millisecond)
	if !limiter.AllowGlobal() {
		t.Error("Запрос после нового интервала должен быть разрешен")
	}
}

func TestGlobalRateLimiterConcurrency(t *testing.T) {
	minInterval := 50 * time.Millisecond
	limiter := NewGlobalRateLimiter(minInterval)
	
	// Тестируем конкурентный доступ
	done := make(chan bool, 10)
	allowed := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func() {
			allowed <- limiter.AllowGlobal()
			done <- true
		}()
	}
	
	// Ждем завершения всех горутин
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Только один запрос должен быть разрешен
	allowedCount := 0
	for i := 0; i < 10; i++ {
		if <-allowed {
			allowedCount++
		}
	}
	
	if allowedCount != 1 {
		t.Errorf("Ожидался только один разрешенный запрос, получено %d", allowedCount)
	}
}
