package migrations

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// FixEscapedCharacters исправляет экранированные символы в заголовках новостей
// Удаляет обратные слеши перед символами, которые не должны быть экранированы
func FixEscapedCharacters(db *sql.DB) error {
	log.Println("Начинаем исправление экранированных символов в заголовках новостей...")

	// Исправляем дефисы: заменяем \- на -
	result, err := db.Exec(`
		UPDATE news 
		SET title = REPLACE(title, '\-', '-')
		WHERE title LIKE '%\-%'
	`)
	if err != nil {
		return fmt.Errorf("ошибка при исправлении дефисов: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("Исправлено дефисов: %d записей", rowsAffected)

	// Исправляем точки: заменяем \. на . (если они были экранированы)
	result, err = db.Exec(`
		UPDATE news 
		SET title = REPLACE(title, '\.', '.')
		WHERE title LIKE '%\.%'
	`)
	if err != nil {
		return fmt.Errorf("ошибка при исправлении точек: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("Исправлено точек: %d записей", rowsAffected)

	// Исправляем только те символы, которые точно не должны быть экранированы
	// Дефис и точка уже исправлены выше
	// Остальные символы (*, _, [, ], (, )) должны оставаться экранированными для Markdown

	// Обновляем updated_at для всех измененных записей
	_, err = db.Exec(`
		UPDATE news 
		SET updated_at = NOW()
		WHERE title LIKE '%\%' ESCAPE '\'
	`)
	if err != nil {
		log.Printf("Предупреждение: не удалось обновить updated_at: %v", err)
	}

	// Проверяем, остались ли еще экранированные символы
	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM news 
		WHERE title LIKE '%\%' ESCAPE '\'
	`).Scan(&count)
	if err != nil {
		log.Printf("Предупреждение: не удалось проверить оставшиеся экранированные символы: %v", err)
	} else {
		log.Printf("Осталось записей с экранированными символами (которые должны быть экранированы): %d", count)
	}

	log.Println("Исправление экранированных символов завершено")
	return nil
}

// UnescapeMarkdown удаляет экранирование для символов, которые не должны быть экранированы
func UnescapeMarkdown(text string) string {
	// Удаляем экранирование для дефисов и точек
	text = strings.ReplaceAll(text, `\-`, `-`)
	text = strings.ReplaceAll(text, `\.`, `.`)
	return text
}
