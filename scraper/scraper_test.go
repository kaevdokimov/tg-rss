package scraper

import (
	"strings"
	"testing"
	"time"
)

func TestRemoveNullBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string without null bytes",
			input:    "normal string",
			expected: "normal string",
		},
		{
			name:     "string with null bytes",
			input:    "string\x00with\x00null\x00bytes",
			expected: "stringwithnullbytes",
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
		{
			name:     "null bytes at beginning and end",
			input:    "\x00start\x00middle\x00end\x00",
			expected: "startmiddleend",
		},
		{
			name:     "HTML content with null bytes",
			input:    "<p>Hello\x00world</p>\x00<script>alert('test')</script>",
			expected: "<p>Helloworld</p><script>alert('test')</script>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeNullBytes(tt.input)
			if result != tt.expected {
				t.Errorf("removeNullBytes(%q) = %q, expected %q", tt.input, result, tt.expected)
			}

			// Проверяем, что в результате нет null байтов
			if strings.Contains(result, "\x00") {
				t.Errorf("Result still contains null bytes: %q", result)
			}
		})
	}
}

func TestScrapeLentaRu(t *testing.T) {
	// Реальный URL новости с lenta.ru (взято из RSS)
	testURL := "https://lenta.ru/news/2026/01/08/vitse-prezident-ssha-nazval-zahvachennoe-sudno-marinera-falshivym-rossiyskim-tankerom/"

	// Отключаем сжатие для дебага (если нужно раскомментировать)
	// SetDisableCompression(true)
	// defer SetDisableCompression(false)

	content, err := ScrapeNewsContent(testURL)
	if err != nil {
		t.Logf("Ошибка при парсинге %s: %v", testURL, err)
		// Не фейлим тест, если страница недоступна или URL неправильный
		return
	}

	// Проверяем, что контент не является сжатыми данными
	if len(content.FullText) > 0 {
		// Проверяем, что текст не содержит бинарных символов (кроме пробелов и переносов)
		binaryChars := 0
		for _, r := range content.FullText {
			if r < 32 && r != 9 && r != 10 && r != 13 {
				binaryChars++
			}
		}

		if binaryChars > len(content.FullText)/10 { // Если больше 10% бинарных символов
			t.Errorf("Текст содержит слишком много бинарных символов (%d из %d), возможно данные не распакованы",
				binaryChars, len(content.FullText))
			t.Logf("Пример текста: %q", content.FullText[:min(200, len(content.FullText))])
		} else {
			t.Logf("Успешно распарсили страницу, длина текста: %d символов", len(content.FullText))
		}
	}

	// Проверяем основные поля
	if content.Author != "" {
		t.Logf("Автор: %s", content.Author)
	}
	if content.Category != "" {
		t.Logf("Категория: %s", content.Category)
	}
	if len(content.Tags) > 0 {
		t.Logf("Теги: %v", content.Tags)
	}
	if len(content.Images) > 0 {
		t.Logf("Изображений: %d", len(content.Images))
	}
	if content.PublishedAt != nil {
		t.Logf("Дата публикации: %s", content.PublishedAt.Format(time.RFC3339))
	}
}

func TestCompressionDetection(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "gzip data",
			data:     []byte{0x1f, 0x8b}, // Gzip magic bytes
			expected: true,
		},
		{
			name:     "deflate data",
			data:     []byte{0x78, 0x9c}, // ZLIB header
			expected: true,
		},
		{
			name:     "brotli data",
			data:     []byte{0xce, 0xb2, 0xcf, 0x81}, // Brotli magic bytes
			expected: true,
		},
		{
			name:     "plain text",
			data:     []byte("Hello world"),
			expected: false,
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(string(tt.data))
			result := isCompressedData(reader)
			if result != tt.expected {
				t.Errorf("isCompressedData() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
