package bot

import (
	"fmt"
	"strings"
	"time"
)

// formatNewsMessage форматирует сообщение для отправки
func formatNewsMessage(title, description string, publishedAt time.Time, sourceName string) string {
	// Обрезаем описание если оно слишком длинное
	trimmedDesc := trimDescription(description, 200)

	// Форматируем относительное время
	relativeTime := formatRelativeTime(publishedAt)

	// Компактный формат: заголовок, источник и время в одну строку
	header := fmt.Sprintf("*%s*\n%s • %s", title, sourceName, relativeTime)

	if trimmedDesc == "" {
		return header
	}

	// Добавляем описание, если есть
	return fmt.Sprintf("%s\n\n%s", header, trimmedDesc)
}

// formatMessage форматирует сообщение в списке для отправки
func formatMessage(i int, title, description string, publishedAt time.Time, sourceName string, link string) string {
	// Форматируем относительное время
	relativeTime := formatRelativeTime(publishedAt)

	// Минималистичный формат: номер, заголовок со ссылкой, источник и время без отступов
	return fmt.Sprintf(
		"%d. [%s](%s)\n%s • %s\n",
		i, title, link, sourceName, relativeTime,
	)
}

// trimDescription обрезает описание до указанной длины
func trimDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}

	// Обрезаем до последнего пробела перед maxLength
	trimmed := description[:maxLength]
	lastSpace := strings.LastIndex(trimmed, " ")
	if lastSpace > maxLength*3/4 { // Если пробел находится в последней четверти
		trimmed = trimmed[:lastSpace]
	}

	return trimmed + "..."
}

// formatRelativeTime форматирует время в относительном виде
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	duration := now.Sub(t)

	if duration < time.Minute {
		return "только что"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d мин", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d ч", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d дн", days)
	} else {
		// Если больше недели, показываем дату в коротком формате
		return t.Format("02.01")
	}
}
