package bot

import (
	"strings"
	"testing"
	"time"
)

func TestFormatMessage(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name        string
		i           int
		title       string
		description string
		publishedAt time.Time
		sourceName  string
		wantContains []string
	}{
		{
			name:        "basic message",
			i:           1,
			title:       "Test News Title",
			description: "",
			publishedAt: now.Add(-30 * time.Minute),
			sourceName:  "Test Source",
			wantContains: []string{"1.", "*Test News Title*", "Test Source", "30 минут назад"},
		},
		{
			name:        "message with description",
			i:           7,
			title:       "Рэпер Гуф сравнил Долину",
			description: "Some description",
			publishedAt: now.Add(-28 * time.Minute),
			sourceName:  "Lenta.ru",
			wantContains: []string{"7.", "*Рэпер Гуф сравнил Долину*", "Lenta.ru", "28 минут назад"},
		},
		{
			name:        "message with long title",
			i:           10,
			title:       "Очень длинный заголовок новости который может быть очень длинным",
			description: "",
			publishedAt: now.Add(-1 * time.Hour),
			sourceName:  "Ria.ru",
			wantContains: []string{"10.", "Ria.ru", "1 час назад"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMessage(tt.i, tt.title, tt.description, tt.publishedAt, tt.sourceName)
			
			// Проверяем, что результат содержит все необходимые элементы
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("formatMessage() не содержит '%s'. Результат: %q", want, result)
				}
			}
			
			// Проверяем, что формат компактный (нет лишних пустых строк)
			lines := strings.Split(result, "\n")
			emptyLines := 0
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					emptyLines++
				}
			}
			// Должно быть максимум 1 пустая строка (между элементами)
			if emptyLines > 1 {
				t.Errorf("formatMessage() содержит слишком много пустых строк. Результат: %q", result)
			}
			
			// Проверяем, что формат содержит номер, заголовок, источник и время
			if !strings.Contains(result, "•") {
				t.Errorf("formatMessage() должна содержать разделитель '•' между источником и временем. Результат: %q", result)
			}
		})
	}
}

func TestFormatNewsMessage(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name        string
		title       string
		description string
		publishedAt time.Time
		sourceName  string
		wantContains []string
		wantNotContains []string
	}{
		{
			name:        "message without description",
			title:       "Test News Title",
			description: "",
			publishedAt: now.Add(-30 * time.Minute),
			sourceName:  "Test Source",
			wantContains: []string{"*Test News Title*", "Test Source", "30 минут назад"},
			wantNotContains: []string{"📰", "⏰"},
		},
		{
			name:        "message with description",
			title:       "Important News",
			description: "This is a description of the news",
			publishedAt: now.Add(-2 * time.Hour),
			sourceName:  "News Source",
			wantContains: []string{"*Important News*", "News Source", "2 часа назад", "This is a description"},
			wantNotContains: []string{"📰", "⏰"},
		},
		{
			name:        "message with long description",
			title:       "Long Description News",
			description: strings.Repeat("A", 300), // Длинное описание
			publishedAt: now.Add(-5 * time.Minute),
			sourceName:  "Source",
			wantContains: []string{"*Long Description News*", "Source", "5 минут назад"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNewsMessage(tt.title, tt.description, tt.publishedAt, tt.sourceName)
			
			// Проверяем, что результат содержит все необходимые элементы
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("formatNewsMessage() не содержит '%s'. Результат: %q", want, result)
				}
			}
			
			// Проверяем, что результат не содержит старые элементы
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(result, notWant) {
					t.Errorf("formatNewsMessage() содержит нежелательный элемент '%s'. Результат: %q", notWant, result)
				}
			}
			
			// Проверяем, что формат компактный
			if strings.Count(result, "\n\n\n") > 0 {
				t.Errorf("formatNewsMessage() содержит слишком много пустых строк. Результат: %q", result)
			}
			
			// Проверяем, что есть разделитель между источником и временем
			if !strings.Contains(result, "•") {
				t.Errorf("formatNewsMessage() должна содержать разделитель '•' между источником и временем. Результат: %q", result)
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name     string
		publishedAt time.Time
		wantContains string
	}{
		{
			name:     "just now",
			publishedAt: now.Add(-30 * time.Second),
			wantContains: "только что",
		},
		{
			name:     "minutes ago",
			publishedAt: now.Add(-28 * time.Minute),
			wantContains: "минут назад",
		},
		{
			name:     "hours ago",
			publishedAt: now.Add(-2 * time.Hour),
			wantContains: "часа назад",
		},
		{
			name:     "days ago",
			publishedAt: now.Add(-3 * 24 * time.Hour),
			wantContains: "дня назад",
		},
		{
			name:     "old news",
			publishedAt: now.Add(-10 * 24 * time.Hour),
			wantContains: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.publishedAt)
			
			if !strings.Contains(result, tt.wantContains) {
				t.Errorf("formatRelativeTime() = %q, должен содержать %q", result, tt.wantContains)
			}
			
			if result == "" {
				t.Error("formatRelativeTime() вернул пустую строку")
			}
		})
	}
}

func TestTrimDescription(t *testing.T) {
	tests := []struct {
		name      string
		desc      string
		maxLength int
		wantMax   int
		wantEnds  string
	}{
		{
			name:      "short description",
			desc:      "Short text",
			maxLength: 200,
			wantMax:   200,
			wantEnds:  "",
		},
		{
			name:      "long description",
			desc:      strings.Repeat("A", 300),
			maxLength: 200,
			wantMax:   203, // 200 + "..."
			wantEnds:  "...",
		},
		{
			name:      "description with spaces",
			desc:      strings.Repeat("word ", 100),
			maxLength: 50,
			wantMax:   53, // 50 + "..."
			wantEnds:  "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimDescription(tt.desc, tt.maxLength)
			
			if len(result) > tt.wantMax {
				t.Errorf("trimDescription() вернул строку длиной %d, максимум %d. Результат: %q", len(result), tt.wantMax, result)
			}
			
			if tt.wantEnds != "" && !strings.HasSuffix(result, tt.wantEnds) {
				t.Errorf("trimDescription() должен заканчиваться на %q, получили: %q", tt.wantEnds, result)
			}
		})
	}
}
