package redis

import (
	"crypto/md5"
	"fmt"
	"reflect"
	"testing"
	"tg-rss/config"
	"time"
)

// TestContentCache_BasicOperations тестирует базовые операции кэша
func TestContentCache_BasicOperations(t *testing.T) {
	redisConfig := &config.RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1, // Используем отдельную БД для тестов
	}

	cache, err := NewContentCache(redisConfig)
	if err != nil {
		t.Skipf("Redis недоступен: %v", err)
	}
	defer cache.Close()

	// Создаем тестовый контент
	now := time.Now().UTC()
	testContent := &CachedNewsContent{
		FullText:        "Это тестовый текст новости для тестирования Redis кэша",
		Author:          "Тестовый Автор",
		Category:        "Тестовая Категория",
		Tags:            []string{"тест", "redis"},
		Images:          []string{"https://example.com/image1.jpg"},
		MetaKeywords:    "тест, redis",
		MetaDescription: "Тестовое описание",
		ContentHTML:     "<p>Тестовый контент</p>",
		PublishedAt:     &now,
	}

	testURL := "https://example.com/test-article"

	// Тест 1: Запись и чтение
	err = cache.Set(testURL, testContent, 30*time.Minute)
	if err != nil {
		t.Fatalf("Ошибка записи в кэш: %v", err)
	}

	retrieved, found := cache.Get(testURL)
	if !found {
		t.Fatal("Контент не найден в кэше")
	}

	if retrieved.FullText != testContent.FullText {
		t.Errorf("Ожидался FullText %q, получено %q", testContent.FullText, retrieved.FullText)
	}

	if retrieved.Author != testContent.Author {
		t.Errorf("Ожидался Author %q, получено %q", testContent.Author, retrieved.Author)
	}

	if !reflect.DeepEqual(retrieved.Tags, testContent.Tags) {
		t.Errorf("Ожидались Tags %v, получено %v", testContent.Tags, retrieved.Tags)
	}

	// Тест 2: Чтение несуществующего контента
	_, found = cache.Get("https://example.com/nonexistent")
	if found {
		t.Error("Неожиданно найден несуществующий контент")
	}

	// Тест 3: Удаление контента
	err = cache.Delete(testURL)
	if err != nil {
		t.Fatalf("Ошибка удаления из кэша: %v", err)
	}

	_, found = cache.Get(testURL)
	if found {
		t.Error("Контент найден после удаления")
	}
}

// BenchmarkContentCache_PutGet тестирует производительность Redis кэша
func BenchmarkContentCache_PutGet(b *testing.B) {
	redisConfig := &config.RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1, // Используем отдельную БД для тестов
	}

	cache, err := NewContentCache(redisConfig)
	if err != nil {
		b.Skipf("Redis недоступен: %v", err)
	}
	defer cache.Close()

	// Создаем тестовый контент
	testContent := &CachedNewsContent{
		FullText:        "Это тестовый текст новости для бенчмарка производительности Redis кэша",
		Author:          "Тестовый Автор",
		Category:        "Тестовая Категория",
		Tags:            []string{"тест", "бенчмарк", "redis"},
		Images:          []string{"https://example.com/image1.jpg"},
		MetaKeywords:    "тест, бенчмарк, redis",
		MetaDescription: "Тестовое описание для бенчмарка",
		ContentHTML:     "<p>Тестовый HTML контент</p>",
	}

	testURL := "https://example.com/test-article"

	b.Run("Put", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			url := testURL + string(rune(i))
			_ = cache.Set(url, testContent, 30*time.Minute)
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Сначала записываем данные
		_ = cache.Set(testURL, testContent, 30*time.Minute)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cache.Get(testURL)
		}
	})

	b.Run("Get_NotFound", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			url := testURL + "-notfound-" + string(rune(i))
			_, _ = cache.Get(url)
		}
	})
}

// TestGetCacheKey тестирует генерацию ключа кэша
func TestGetCacheKey(t *testing.T) {
	cache := &ContentCache{
		prefix: "content:",
	}

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple URL",
			url:      "https://example.com/article",
			expected: fmt.Sprintf("content:%x", md5.Sum([]byte("https://example.com/article"))),
		},
		{
			name:     "URL with query",
			url:      "https://example.com/article?id=123",
			expected: fmt.Sprintf("content:%x", md5.Sum([]byte("https://example.com/article?id=123"))),
		},
		{
			name:     "empty URL",
			url:      "",
			expected: fmt.Sprintf("content:%x", md5.Sum([]byte(""))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.getCacheKey(tt.url)
			if result != tt.expected {
				t.Errorf("getCacheKey(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
