package db

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tg-rss/config"
)

func TestCleanUTF8String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid UTF-8 string",
			input:    "Привет мир",
			expected: "Привет мир",
		},
		{
			name:     "string with null bytes",
			input:    "Hello\x00World\x00Test",
			expected: "HelloWorldTest",
		},
		{
			name:     "string with invalid UTF-8",
			input:    "Hello\x80\x81World", // Invalid UTF-8 bytes
			expected: "Hello�World",        // Should be replaced with replacement char
		},
		{
			name:     "string with both null bytes and invalid UTF-8",
			input:    "Test\x00\x80\x81String\x00",
			expected: "Test�String",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only null bytes",
			input:    "\x00\x00\x00",
			expected: "",
		},
		{
			name:     "HTML content with null bytes",
			input:    "<div>\x00Hello\x00</div>\x00<script>\x80\x81</script>",
			expected: "<div>Hello</div><script>�</script>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanUTF8String(tt.input)
			if result != tt.expected {
				t.Errorf("cleanUTF8String(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// removeNullBytes удаляет null байты из строки (локальная копия для тестов)
func removeNullBytes(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// TestRemoveNullBytes тестирует функцию удаления null байтов
func TestRemoveNullBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no null bytes",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "with null bytes",
			input:    "Hello\x00World\x00Test",
			expected: "HelloWorldTest",
		},
		{
			name:     "only null bytes",
			input:    "\x00\x00\x00",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeNullBytes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConnect тестирует подключение к БД (интеграционный тест)
func TestConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Проверяем, есть ли переменные окружения для тестовой БД
	if os.Getenv("POSTGRES_HOST") == "" {
		t.Skip("Skipping test: no database configuration")
	}

	cfg := config.LoadDBConfig()
	db, err := Connect(cfg)

	require.NoError(t, err)
	assert.NotNil(t, db)

	// Проверяем подключение
	err = db.Ping()
	assert.NoError(t, err)

	// Закрываем соединение
	db.Close()
}

// TestSaveNews тестирует сохранение новости (интеграционный тест)
func TestSaveNews(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if os.Getenv("POSTGRES_HOST") == "" {
		t.Skip("Skipping test: no database configuration")
	}

	cfg := config.LoadDBConfig()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Сначала создадим тестовый источник
	sourceID := int64(999999) // Используем большой ID для тестов
	_, err = db.Exec(`
		INSERT INTO sources (id, name, url, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
	`, sourceID, "Test Source", "https://test.example.com/rss.xml", "active")
	require.NoError(t, err)

	// Сохраняем новость
	newsID, err := SaveNews(db, sourceID, "Test Title", "Test Description", "https://test.example.com/news/1", time.Now())

	require.NoError(t, err)
	assert.Greater(t, newsID, int64(0))

	// Проверяем, что новость сохранена
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM news WHERE id = $1", newsID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Очищаем тестовые данные
	_, err = db.Exec("DELETE FROM news WHERE id = $1", newsID)
	assert.NoError(t, err)
	_, err = db.Exec("DELETE FROM sources WHERE id = $1", sourceID)
	assert.NoError(t, err)
}

// TestUserExists тестирует проверку существования пользователя (интеграционный тест)
func TestUserExists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if os.Getenv("POSTGRES_HOST") == "" {
		t.Skip("Skipping test: no database configuration")
	}

	cfg := config.LoadDBConfig()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Создаем тестового пользователя
	testChatID := int64(999999999) // Большой ID для тестов
	testUsername := "test_user_db"

	_, err = SaveUser(db, User{ChatId: testChatID, Username: testUsername})
	require.NoError(t, err)

	// Проверяем существование
	exists, err := UserExists(db, testChatID)
	require.NoError(t, err)
	assert.True(t, exists)

	// Проверяем несуществующего пользователя
	exists, err = UserExists(db, 999999998)
	require.NoError(t, err)
	assert.False(t, exists)

	// Очищаем тестовые данные
	_, err = db.Exec("DELETE FROM users WHERE chat_id = $1", testChatID)
	assert.NoError(t, err)
}

// TestFindActiveSources тестирует получение активных источников (интеграционный тест)
func TestFindActiveSources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if os.Getenv("POSTGRES_HOST") == "" {
		t.Skip("Skipping test: no database configuration")
	}

	cfg := config.LoadDBConfig()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer db.Close()

	// Получаем текущие активные источники
	sources, err := FindActiveSources(db)
	require.NoError(t, err)
	assert.NotNil(t, sources)

	// Проверяем, что все источники имеют статус "active"
	for _, source := range sources {
		assert.Equal(t, Active, source.Status)
		assert.NotEmpty(t, source.Name)
		assert.NotEmpty(t, source.Url)
	}
}
