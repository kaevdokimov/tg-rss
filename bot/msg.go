package bot

import (
	"fmt"
	"time"
)

// formatNewsMessage форматирует сообщение для отправки
func formatNewsMessage(title, link, description string, publishedAt time.Time) string {
	return fmt.Sprintf(
		"[*%s*](%s)\n%s\n\nОпубликовано: %s",
		title, link, description, publishedAt.Format("02.01.2006 15:04"),
	)
}

// formatMessage форматирует сообщение в списке для отправки
func formatMessage(i int, title, link, description string, publishedAt time.Time) string {
	return fmt.Sprintf(
		"%d. [*%s*](%s)\n%s\n\nОпубликовано: %s",
		i, title, link, description, publishedAt.Format("02.01.2006 15:04"),
	)
}
