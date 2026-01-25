package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	ttl := 5 * time.Minute
	c := NewCache(ttl)

	assert.NotNil(t, c)
	assert.Equal(t, ttl, c.ttl)
	assert.NotNil(t, c.logger)
}

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache(5 * time.Minute)

	// Set и Get строкового значения
	c.Set("key1", "value1")
	val, ok := c.Get("key1")

	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestCache_GetNonExistent(t *testing.T) {
	c := NewCache(5 * time.Minute)

	val, ok := c.Get("nonexistent")

	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestCache_Expiration(t *testing.T) {
	// Короткий TTL для быстрого теста
	c := NewCache(100 * time.Millisecond)

	c.Set("key1", "value1")

	// Сразу после установки - значение должно быть
	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	// Ждем истечения TTL
	time.Sleep(150 * time.Millisecond)

	// После истечения - значение должно отсутствовать
	val, ok = c.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestCache_Delete(t *testing.T) {
	c := NewCache(5 * time.Minute)

	c.Set("key1", "value1")

	// Проверяем, что значение есть
	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	// Удаляем
	c.Delete("key1")

	// Проверяем, что значение удалено
	val, ok = c.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(5 * time.Minute)

	// Добавляем несколько значений
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	// Проверяем размер
	size := c.Size()
	assert.Equal(t, 3, size)

	// Очищаем
	c.Clear()

	// Проверяем, что все удалено
	size = c.Size()
	assert.Equal(t, 0, size)

	// Проверяем, что значения недоступны
	_, ok := c.Get("key1")
	assert.False(t, ok)

	_, ok = c.Get("key2")
	assert.False(t, ok)
}

func TestCache_Size(t *testing.T) {
	c := NewCache(5 * time.Minute)

	// Начальный размер
	assert.Equal(t, 0, c.Size())

	// Добавляем значения
	c.Set("key1", "value1")
	assert.Equal(t, 1, c.Size())

	c.Set("key2", "value2")
	assert.Equal(t, 2, c.Size())

	c.Set("key3", "value3")
	assert.Equal(t, 3, c.Size())

	// Удаляем одно значение
	c.Delete("key2")
	assert.Equal(t, 2, c.Size())
}

func TestCache_SizeWithExpiredEntries(t *testing.T) {
	c := NewCache(100 * time.Millisecond)

	// Добавляем значения
	c.Set("key1", "value1")
	c.Set("key2", "value2")

	assert.Equal(t, 2, c.Size())

	// Ждем истечения TTL
	time.Sleep(150 * time.Millisecond)

	// Size должен удалить истекшие записи и вернуть 0
	assert.Equal(t, 0, c.Size())
}

func TestCache_Cleanup(t *testing.T) {
	c := NewCache(100 * time.Millisecond)

	// Добавляем значения
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	// Ждем истечения TTL
	time.Sleep(150 * time.Millisecond)

	// Запускаем cleanup
	c.Cleanup()

	// Проверяем, что все удалено
	assert.Equal(t, 0, c.Size())
}

func TestCache_UpdateValue(t *testing.T) {
	c := NewCache(5 * time.Minute)

	// Устанавливаем начальное значение
	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	// Обновляем значение
	c.Set("key1", "value2")

	val, ok = c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value2", val)
}

func TestCache_DifferentTypes(t *testing.T) {
	c := NewCache(5 * time.Minute)

	// Строка
	c.Set("string", "test")
	val, ok := c.Get("string")
	assert.True(t, ok)
	assert.Equal(t, "test", val.(string))

	// Число
	c.Set("int", 42)
	val, ok = c.Get("int")
	assert.True(t, ok)
	assert.Equal(t, 42, val.(int))

	// Структура
	type TestStruct struct {
		Name string
		Age  int
	}
	testData := TestStruct{Name: "Alice", Age: 30}
	c.Set("struct", testData)
	val, ok = c.Get("struct")
	assert.True(t, ok)
	assert.Equal(t, testData, val.(TestStruct))

	// Срез
	testSlice := []string{"a", "b", "c"}
	c.Set("slice", testSlice)
	val, ok = c.Get("slice")
	assert.True(t, ok)
	assert.Equal(t, testSlice, val.([]string))
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache(5 * time.Minute)

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	// Параллельная запись
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := string(rune('a' + (id+j)%26))
				c.Set(key, id*numOperations+j)
			}
		}(i)
	}

	// Параллельное чтение
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := string(rune('a' + (id+j)%26))
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Проверяем, что кэш в рабочем состоянии
	c.Set("test", "value")
	val, ok := c.Get("test")
	assert.True(t, ok)
	assert.Equal(t, "value", val)
}

func TestCache_ConcurrentDeleteAndGet(t *testing.T) {
	c := NewCache(5 * time.Minute)

	// Заполняем кэш
	for i := 0; i < 100; i++ {
		c.Set(string(rune('a'+i%26)), i)
	}

	var wg sync.WaitGroup

	// Параллельное удаление
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c.Delete(string(rune('a' + id%26)))
		}(i)
	}

	// Параллельное чтение
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c.Get(string(rune('a' + id%26)))
		}(i)
	}

	wg.Wait()

	// Кэш должен остаться в рабочем состоянии
	require.NotPanics(t, func() {
		c.Size()
	})
}

func TestGlobalCaches(t *testing.T) {
	// Проверяем, что глобальные кэши инициализированы
	t.Run("UserCache", func(t *testing.T) {
		assert.NotNil(t, UserCache)
		assert.Equal(t, 5*time.Minute, UserCache.ttl)
	})

	t.Run("SourceCache", func(t *testing.T) {
		assert.NotNil(t, SourceCache)
		assert.Equal(t, 10*time.Minute, SourceCache.ttl)
	})

	t.Run("ContentCache", func(t *testing.T) {
		assert.NotNil(t, ContentCache)
		assert.Equal(t, 30*time.Minute, ContentCache.ttl)
	})

	t.Run("SubscriptionCache", func(t *testing.T) {
		assert.NotNil(t, SubscriptionCache)
		assert.Equal(t, 5*time.Minute, SubscriptionCache.ttl)
	})
}

func TestCacheEntry(t *testing.T) {
	entry := CacheEntry{
		Value:      "test",
		Expiration: time.Now().Add(5 * time.Minute),
	}

	assert.Equal(t, "test", entry.Value)
	assert.True(t, entry.Expiration.After(time.Now()))
}

// Benchmark тесты
func BenchmarkCache_Set(b *testing.B) {
	c := NewCache(5 * time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("key", "value")
	}
}

func BenchmarkCache_Get(b *testing.B) {
	c := NewCache(5 * time.Minute)
	c.Set("key", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("key")
	}
}

func BenchmarkCache_SetConcurrent(b *testing.B) {
	c := NewCache(5 * time.Minute)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(string(rune('a'+i%26)), i)
			i++
		}
	})
}

func BenchmarkCache_GetConcurrent(b *testing.B) {
	c := NewCache(5 * time.Minute)
	for i := 0; i < 26; i++ {
		c.Set(string(rune('a'+i)), i)
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(string(rune('a' + i%26)))
			i++
		}
	})
}
