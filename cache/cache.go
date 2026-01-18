package cache

import (
	"sync"
	"time"

	"tg-rss/monitoring"
)

// CacheEntry представляет элемент кэша
type CacheEntry struct {
	Value      interface{}
	Expiration time.Time
}

// Cache представляет thread-safe in-memory кэш
type Cache struct {
	data   sync.Map
	ttl    time.Duration
	logger *monitoring.StructuredLogger
}

// NewCache создает новый кэш с указанным TTL
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		ttl:    ttl,
		logger: monitoring.GetLogger("cache"),
	}
}

// Set сохраняет значение в кэше
func (c *Cache) Set(key string, value interface{}) {
	entry := CacheEntry{
		Value:      value,
		Expiration: time.Now().Add(c.ttl),
	}
	c.data.Store(key, entry)
	c.logger.Debug("cache entry set", "key", key, "ttl", c.ttl)
}

// Get получает значение из кэша
func (c *Cache) Get(key string) (interface{}, bool) {
	entry, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}

	cacheEntry := entry.(CacheEntry)
	if time.Now().After(cacheEntry.Expiration) {
		// Кэш истек, удаляем
		c.data.Delete(key)
		c.logger.Debug("cache entry expired", "key", key)
		return nil, false
	}

	return cacheEntry.Value, true
}

// Delete удаляет значение из кэша
func (c *Cache) Delete(key string) {
	c.data.Delete(key)
	c.logger.Debug("cache entry deleted", "key", key)
}

// Clear очищает весь кэш
func (c *Cache) Clear() {
	c.data = sync.Map{}
	c.logger.Info("cache cleared")
}

// Size возвращает количество элементов в кэше
func (c *Cache) Size() int {
	count := 0
	c.data.Range(func(key, value interface{}) bool {
		cacheEntry := value.(CacheEntry)
		if time.Now().After(cacheEntry.Expiration) {
			// Удаляем просроченные записи при подсчете
			c.data.Delete(key)
		} else {
			count++
		}
		return true
	})
	return count
}

// Cleanup удаляет просроченные записи
func (c *Cache) Cleanup() {
	c.data.Range(func(key, value interface{}) bool {
		cacheEntry := value.(CacheEntry)
		if time.Now().After(cacheEntry.Expiration) {
			c.data.Delete(key)
			c.logger.Debug("expired cache entry cleaned up", "key", key.(string))
		}
		return true
	})
}

// StartCleanupWorker запускает фоновый процесс очистки кэша
func (c *Cache) StartCleanupWorker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			c.Cleanup()
		}
	}()
	c.logger.Info("cache cleanup worker started", "interval", interval)
}

// Глобальные кэши для различных типов данных
var (
	// UserCache кэширует информацию о пользователях (5 минут TTL)
	UserCache = NewCache(5 * time.Minute)

	// SourceCache кэширует информацию об источниках RSS (10 минут TTL)
	SourceCache = NewCache(10 * time.Minute)

	// ContentCache кэширует скрапированный контент (30 минут TTL)
	ContentCache = NewCache(30 * time.Minute)

	// SubscriptionCache кэширует подписки пользователей (5 минут TTL)
	SubscriptionCache = NewCache(5 * time.Minute)
)

func init() {
	// Запускаем очистку кэша каждые 5 минут
	UserCache.StartCleanupWorker(5 * time.Minute)
	SourceCache.StartCleanupWorker(5 * time.Minute)
	ContentCache.StartCleanupWorker(5 * time.Minute)
	SubscriptionCache.StartCleanupWorker(5 * time.Minute)
}