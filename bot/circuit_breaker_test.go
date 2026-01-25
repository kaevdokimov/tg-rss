package bot

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 30*time.Second, 5*time.Second)
	
	assert.NotNil(t, cb)
	assert.Equal(t, "test", cb.name)
	assert.Equal(t, 3, cb.failureThreshold)
	assert.Equal(t, 30*time.Second, cb.recoveryTimeout)
	assert.Equal(t, 5*time.Second, cb.resetTimeout)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, int32(0), cb.GetFailures())
}

func TestCircuitBreaker_SuccessfulCall(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 30*time.Second, 5*time.Second)
	
	called := false
	err := cb.Call(func() error {
		called = true
		return nil
	})
	
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, int32(0), cb.GetFailures())
}

func TestCircuitBreaker_FailureCall(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 30*time.Second, 5*time.Second)
	
	testErr := errors.New("test error")
	err := cb.Call(func() error {
		return testErr
	})
	
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, int32(1), cb.GetFailures())
}

func TestCircuitBreaker_OpenAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 30*time.Second, 5*time.Second)
	testErr := errors.New("test error")
	
	// Выполняем 3 неудачные попытки
	for i := 0; i < 3; i++ {
		err := cb.Call(func() error {
			return testErr
		})
		assert.Error(t, err)
	}
	
	// Circuit breaker должен открыться
	assert.Equal(t, StateOpen, cb.GetState())
	assert.Equal(t, int32(3), cb.GetFailures())
	
	// Следующая попытка должна быть заблокирована
	called := false
	err := cb.Call(func() error {
		called = true
		return nil
	})
	
	assert.Error(t, err)
	assert.Equal(t, ErrCircuitBreakerOpen, err)
	assert.False(t, called, "Функция не должна быть вызвана когда circuit открыт")
}

func TestCircuitBreaker_HalfOpenAfterRecoveryTimeout(t *testing.T) {
	// Используем короткий timeout для быстрого теста
	cb := NewCircuitBreaker("test", 2, 100*time.Millisecond, 50*time.Millisecond)
	testErr := errors.New("test error")
	
	// Открываем circuit breaker
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error {
			return testErr
		})
	}
	
	assert.Equal(t, StateOpen, cb.GetState())
	
	// Ждем recovery timeout
	time.Sleep(150 * time.Millisecond)
	
	// Следующая попытка должна перевести в Half-Open
	called := false
	err := cb.Call(func() error {
		called = true
		return nil
	})
	
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, StateClosed, cb.GetState(), "После успешной попытки в Half-Open должен закрыться")
}

func TestCircuitBreaker_RecoveryFromHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 100*time.Millisecond, 50*time.Millisecond)
	testErr := errors.New("test error")
	
	// Открываем circuit
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error {
			return testErr
		})
	}
	
	// Ждем recovery timeout
	time.Sleep(150 * time.Millisecond)
	
	// Успешный вызов в Half-Open состоянии
	err := cb.Call(func() error {
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, int32(0), cb.GetFailures())
}

func TestCircuitBreaker_FailureInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 100*time.Millisecond, 50*time.Millisecond)
	testErr := errors.New("test error")
	
	// Открываем circuit
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error {
			return testErr
		})
	}
	
	// Ждем recovery timeout
	time.Sleep(150 * time.Millisecond)
	
	// Неудачный вызов в Half-Open состоянии
	err := cb.Call(func() error {
		return testErr
	})
	
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState(), "После неудачи в Half-Open должен вернуться в Open")
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 30*time.Second, 5*time.Second)
	testErr := errors.New("test error")
	
	// Открываем circuit
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error {
			return testErr
		})
	}
	
	assert.Equal(t, StateOpen, cb.GetState())
	assert.Equal(t, int32(2), cb.GetFailures())
	
	// Сбрасываем
	cb.Reset()
	
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, int32(0), cb.GetFailures())
	
	// Проверяем, что работает после reset
	err := cb.Call(func() error {
		return nil
	})
	
	assert.NoError(t, err)
}

func TestCircuitBreaker_ConcurrentCalls(t *testing.T) {
	cb := NewCircuitBreaker("test", 10, 30*time.Second, 5*time.Second)
	
	var wg sync.WaitGroup
	successCount := int32(0)
	errorCount := int32(0)
	
	// 100 параллельных вызовов
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			err := cb.Call(func() error {
				// Половина успешных, половина с ошибками
				if idx%2 == 0 {
					return nil
				}
				return errors.New("test error")
			})
			
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&errorCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Проверяем, что все вызовы обработаны
	total := successCount + errorCount
	assert.Equal(t, int32(100), total, "Все вызовы должны быть обработаны")
	
	// Circuit может быть открыт после большого количества ошибок
	// Проверяем, что он в разумном состоянии
	state := cb.GetState()
	assert.True(t, state == StateClosed || state == StateOpen, "Состояние должно быть либо Closed, либо Open")
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 100*time.Millisecond, 50*time.Millisecond)
	testErr := errors.New("test error")
	
	// 1. Начальное состояние - Closed
	assert.Equal(t, StateClosed, cb.GetState())
	
	// 2. Первая ошибка - остается Closed
	_ = cb.Call(func() error {
		return testErr
	})
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, int32(1), cb.GetFailures())
	
	// 3. Вторая ошибка - переходит в Open
	_ = cb.Call(func() error {
		return testErr
	})
	assert.Equal(t, StateOpen, cb.GetState())
	assert.Equal(t, int32(2), cb.GetFailures())
	
	// 4. Попытка вызова в Open - блокируется
	err := cb.Call(func() error {
		return nil
	})
	assert.Equal(t, ErrCircuitBreakerOpen, err)
	
	// 5. Ждем recovery timeout - переходит в HalfOpen
	time.Sleep(150 * time.Millisecond)
	
	// 6. Успешный вызов в HalfOpen - закрывается
	err = cb.Call(func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, int32(0), cb.GetFailures())
}

func TestCircuitBreakerError(t *testing.T) {
	err := NewCircuitBreakerError("test message")
	assert.NotNil(t, err)
	assert.Equal(t, "test message", err.Error())
	assert.Equal(t, "test message", err.Message)
}

func TestGlobalCircuitBreakers(t *testing.T) {
	// Проверяем, что глобальные circuit breakers инициализированы
	t.Run("RSS Circuit Breaker", func(t *testing.T) {
		cb := GetRSSCircuitBreaker()
		assert.NotNil(t, cb)
		assert.Equal(t, "rss_sources", cb.name)
	})
	
	t.Run("Scraper Circuit Breaker", func(t *testing.T) {
		cb := GetScraperCircuitBreaker()
		assert.NotNil(t, cb)
		assert.Equal(t, "content_scraper", cb.name)
	})
	
	t.Run("Telegram Circuit Breaker", func(t *testing.T) {
		cb := GetTelegramCircuitBreaker()
		assert.NotNil(t, cb)
		assert.Equal(t, "telegram_api", cb.name)
	})
}

// Benchmark для проверки производительности
func BenchmarkCircuitBreaker_SuccessfulCall(b *testing.B) {
	cb := NewCircuitBreaker("bench", 10, 30*time.Second, 5*time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Call(func() error {
			return nil
		})
	}
}

func BenchmarkCircuitBreaker_FailedCall(b *testing.B) {
	cb := NewCircuitBreaker("bench", 1000000, 30*time.Second, 5*time.Second)
	testErr := errors.New("test error")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Call(func() error {
			return testErr
		})
	}
}

func BenchmarkCircuitBreaker_ConcurrentCalls(b *testing.B) {
	cb := NewCircuitBreaker("bench", 10, 30*time.Second, 5*time.Second)
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cb.Call(func() error {
				return nil
			})
		}
	})
}
