package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
)

// cleanUTF8String очищает строку от некорректных UTF-8 последовательностей
func cleanUTF8String(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	// Заменяем некорректные UTF-8 последовательности на безопасные символы
	return strings.ToValidUTF8(s, "�")
}

// SaveNewsContent сохраняет полный контент новости
func SaveNewsContent(db *sql.DB, newsID int64, fullText, author, category string, tags, images []string,
	metaKeywords, metaDescription string, metaData map[string]string, contentHTML string) error {

	// Очищаем все строковые поля от некорректных UTF-8 последовательностей
	fullText = cleanUTF8String(fullText)
	author = cleanUTF8String(author)
	category = cleanUTF8String(category)
	metaKeywords = cleanUTF8String(metaKeywords)
	metaDescription = cleanUTF8String(metaDescription)
	contentHTML = cleanUTF8String(contentHTML)

	// Очищаем массивы строк
	for i, tag := range tags {
		tags[i] = cleanUTF8String(tag)
	}
	for i, image := range images {
		images[i] = cleanUTF8String(image)
	}

	// Преобразуем tags и images в массивы PostgreSQL
	// Используем pq.Array для правильного форматирования
	tagsArray := tags
	if tagsArray == nil {
		tagsArray = []string{}
	}

	imagesArray := images
	if imagesArray == nil {
		imagesArray = []string{}
	}

	// Преобразуем metaData в JSON
	metaDataJSON := "{}"
	if len(metaData) > 0 {
		jsonBytes, err := json.Marshal(metaData)
		if err == nil {
			metaDataJSON = string(jsonBytes)
		}
	}

	query := `
		UPDATE news 
		SET full_text = $1,
			author = $2,
			category = $3,
			tags = $4,
			images = $5,
			meta_keywords = $6,
			meta_description = $7,
			meta_data = $8::JSONB,
			content_html = $9,
			scraped_at = NOW(),
			scrape_status = 'success',
			scrape_error = NULL,
			updated_at = NOW()
		WHERE id = $10
	`

	_, err := db.Exec(query, fullText, author, category, pq.Array(tagsArray), pq.Array(imagesArray),
		metaKeywords, metaDescription, metaDataJSON, contentHTML, newsID)

	if err != nil {
		return fmt.Errorf("ошибка сохранения контента новости: %w", err)
	}

	return nil
}

// MarkNewsScrapeFailed отмечает новость как не удавшуюся при парсинге
func MarkNewsScrapeFailed(db *sql.DB, newsID int64, errorMsg string) error {
	query := `
		UPDATE news 
		SET scrape_status = 'failed',
			scrape_error = $1,
			scraped_at = NOW(),
			updated_at = NOW()
		WHERE id = $2
	`

	_, err := db.Exec(query, errorMsg, newsID)
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса парсинга: %w", err)
	}

	return nil
}

// GetNewsForScraping возвращает список новостей, которые нужно распарсить
func GetNewsForScraping(db *sql.DB, limit int) ([]NewsForScraping, error) {
	query := `
		SELECT id, link, published_at
		FROM news
		WHERE (scrape_status IS NULL OR scrape_status = 'pending' OR scrape_status = 'failed')
		  AND published_at > NOW() - INTERVAL '7 days' -- только новости за последние 7 дней
		ORDER BY id DESC
		LIMIT $1
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения новостей для парсинга: %w", err)
	}
	defer rows.Close()

	var newsList []NewsForScraping
	for rows.Next() {
		var news NewsForScraping
		err := rows.Scan(&news.ID, &news.Link, &news.PublishedAt)
		if err != nil {
			continue
		}
		newsList = append(newsList, news)
	}

	return newsList, nil
}

// NewsForScraping представляет новость для парсинга
type NewsForScraping struct {
	ID          int64
	Link        string
	PublishedAt time.Time
}

