package bot

import (
	"fmt"
	"regexp"
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

// escapeMarkdown экранирует специальные символы Markdown
func escapeMarkdown(text string) string {
	// Экранируем специальные символы Markdown: * _ [ ] ( ) ~ ` > # + - = | { } . !
	re := regexp.MustCompile(`([*_\[\]()~` + "`" + `>#+\-=|{}.!])`)
	return re.ReplaceAllString(text, `\$1`)
}

// formatMessage форматирует сообщение в списке для отправки
func formatMessage(i int, title string, publishedAt time.Time, sourceName string, newsLink string) string {
	// Форматируем относительное время
	relativeTime := formatRelativeTime(publishedAt)

	// Экранируем специальные символы в заголовке и названии источника
	// Но не экранируем ссылку, так как она уже в правильном формате
	escapedTitle := escapeMarkdown(title)
	escapedSourceName := escapeMarkdown(sourceName)
	escapedRelativeTime := escapeMarkdown(relativeTime)

	// Минималистичный формат: заголовок обычным текстом, ссылка на новость через иконку
	// 🔗 - заметная иконка для ссылки на новость
	return fmt.Sprintf(
		"%d. %s   [%s](%s) • %s\n",
		i, escapedTitle, escapedSourceName, newsLink, escapedRelativeTime,
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
