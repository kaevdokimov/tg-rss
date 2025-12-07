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
func formatMessage(i int, title, description string, publishedAt time.Time, sourceName string) string {
	// Форматируем относительное время
	relativeTime := formatRelativeTime(publishedAt)

	// Компактный формат: номер, заголовок, источник и время в одну строку
	return fmt.Sprintf(
		"%d. *%s*\n   %s • %s\n",
		i, title, sourceName, relativeTime,
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
		if minutes == 1 {
			return "1 минуту назад"
		} else if minutes < 5 {
			return fmt.Sprintf("%d минуты назад", minutes)
		} else {
			return fmt.Sprintf("%d минут назад", minutes)
		}
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 час назад"
		} else if hours < 5 {
			return fmt.Sprintf("%d часа назад", hours)
		} else {
			return fmt.Sprintf("%d часов назад", hours)
		}
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 день назад"
		} else if days < 5 {
			return fmt.Sprintf("%d дня назад", days)
		} else {
			return fmt.Sprintf("%d дней назад", days)
		}
	} else {
		// Если больше недели, показываем дату
		return t.Format("02.01.2006 15:04")
	}
}
