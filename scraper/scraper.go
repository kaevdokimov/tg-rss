package scraper

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"tg-rss/monitoring"

	"github.com/PuerkitoBio/goquery"
)

var scraperLogger = monitoring.NewLogger("Scraper")

// NewsContent содержит полный контент новости со страницы
type NewsContent struct {
	FullText      string            // Полный текст новости
	Author        string            // Автор статьи
	Category      string            // Категория новости
	Tags          []string          // Теги/ключевые слова
	Images        []string          // URL изображений из статьи
	PublishedAt   *time.Time        // Дата публикации со страницы (может отличаться от RSS)
	MetaKeywords  string            // Мета-тег keywords
	MetaDescription string          // Мета-тег description
	MetaData      map[string]string // Дополнительные метаданные
	ContentHTML   string            // HTML контента статьи (для будущего анализа)
}

// ScrapeNewsContent парсит страницу новости и извлекает полный контент
func ScrapeNewsContent(url string) (*NewsContent, error) {
	scraperLogger.Debug("Начинаем парсинг страницы: %s", url)

	// Создаем HTTP клиент с таймаутом
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Загружаем страницу
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки страницы: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("неверный статус код: %d", resp.StatusCode)
	}

	// Парсим HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %w", err)
	}

	content := &NewsContent{
		MetaData: make(map[string]string),
		Tags:     []string{},
		Images:   []string{},
	}

	// Извлекаем метаданные
	extractMetaData(doc, content)

	// Извлекаем полный текст статьи
	extractFullText(doc, content)

	// Извлекаем автора
	extractAuthor(doc, content)

	// Извлекаем категорию
	extractCategory(doc, content)

	// Извлекаем теги
	extractTags(doc, content)

	// Извлекаем изображения
	extractImages(doc, content)

	// Извлекаем дату публикации
	extractPublishedDate(doc, content)

	// Сохраняем HTML контента для будущего анализа
	extractContentHTML(doc, content)

	scraperLogger.Debug("Парсинг завершен: текст=%d символов, изображений=%d, тегов=%d",
		len(content.FullText), len(content.Images), len(content.Tags))

	return content, nil
}

// extractMetaData извлекает метаданные из <meta> тегов
func extractMetaData(doc *goquery.Document, content *NewsContent) {
	// Keywords
	doc.Find("meta[name='keywords'], meta[property='keywords']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("content"); exists {
			content.MetaKeywords = val
		}
	})

	// Description
	doc.Find("meta[name='description'], meta[property='description'], meta[property='og:description']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("content"); exists && content.MetaDescription == "" {
			content.MetaDescription = val
		}
	})

	// Дополнительные метаданные (Open Graph, Twitter Cards и т.д.)
	doc.Find("meta[property^='og:'], meta[name^='twitter:']").Each(func(i int, s *goquery.Selection) {
		prop, _ := s.Attr("property")
		name, _ := s.Attr("name")
		val, exists := s.Attr("content")
		if exists {
			key := prop
			if key == "" {
				key = name
			}
			if key != "" {
				content.MetaData[key] = val
			}
		}
	})
}

// extractFullText извлекает полный текст статьи
func extractFullText(doc *goquery.Document, content *NewsContent) {
	// Популярные селекторы для контента статьи
	selectors := []string{
		"article .article-content",
		"article .content",
		"article .post-content",
		"article .entry-content",
		"article .article-body",
		"article .news-content",
		"article .text",
		"article",
		".article-content",
		".content",
		".post-content",
		".entry-content",
		".article-body",
		".news-content",
		"#article-content",
		"#content",
		"[itemprop='articleBody']",
		"[role='article']",
	}

	var textParts []string

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			// Удаляем скрипты и стили
			s.Find("script, style, noscript").Remove()
			
			// Извлекаем текст
			text := strings.TrimSpace(s.Text())
			if len(text) > 100 { // Минимальная длина для валидного контента
				textParts = append(textParts, text)
			}
		})

		if len(textParts) > 0 {
			break
		}
	}

	// Если не нашли по селекторам, пробуем найти по структурированным данным
	if len(textParts) == 0 {
		doc.Find("p").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if len(text) > 50 {
				textParts = append(textParts, text)
			}
		})
	}

	content.FullText = strings.Join(textParts, "\n\n")
	content.FullText = strings.TrimSpace(content.FullText)
}

// extractAuthor извлекает автора статьи
func extractAuthor(doc *goquery.Document, content *NewsContent) {
	selectors := []string{
		"[itemprop='author']",
		".author",
		".article-author",
		".post-author",
		"meta[name='author']",
		"meta[property='article:author']",
	}

	for _, selector := range selectors {
		if strings.HasPrefix(selector, "meta") {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				if val, exists := s.Attr("content"); exists {
					content.Author = strings.TrimSpace(val)
				}
			})
		} else {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				if text != "" {
					content.Author = text
				}
			})
		}

		if content.Author != "" {
			break
		}
	}
}

// extractCategory извлекает категорию новости
func extractCategory(doc *goquery.Document, content *NewsContent) {
	selectors := []string{
		"[itemprop='articleSection']",
		".category",
		".article-category",
		".post-category",
		".breadcrumb a:last-child",
		"meta[property='article:section']",
	}

	for _, selector := range selectors {
		if strings.HasPrefix(selector, "meta") {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				if val, exists := s.Attr("content"); exists {
					content.Category = strings.TrimSpace(val)
				}
			})
		} else {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				if text != "" {
					content.Category = text
				}
			})
		}

		if content.Category != "" {
			break
		}
	}
}

// extractTags извлекает теги/ключевые слова
func extractTags(doc *goquery.Document, content *NewsContent) {
	// Из мета-тегов
	if content.MetaKeywords != "" {
		keywords := strings.Split(content.MetaKeywords, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				content.Tags = append(content.Tags, kw)
			}
		}
	}

	// Из элементов страницы
	selectors := []string{
		".tags a",
		".tag a",
		".keywords a",
		"[rel='tag']",
		".article-tags a",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				// Проверяем, нет ли уже такого тега
				found := false
				for _, tag := range content.Tags {
					if strings.EqualFold(tag, text) {
						found = true
						break
					}
				}
				if !found {
					content.Tags = append(content.Tags, text)
				}
			}
		})
	}
}

// extractImages извлекает URL изображений из статьи
func extractImages(doc *goquery.Document, content *NewsContent) {
	// Ищем изображения в контенте статьи
	selectors := []string{
		"article img",
		".article-content img",
		".content img",
		"[itemprop='articleBody'] img",
	}

	seen := make(map[string]bool)

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			src, exists := s.Attr("src")
			if !exists {
				src, exists = s.Attr("data-src") // lazy loading
			}
			if exists && src != "" && !seen[src] {
				// Преобразуем относительные URL в абсолютные
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				} else if strings.HasPrefix(src, "/") {
					// Нужен базовый URL, но для простоты оставляем как есть
				}
				content.Images = append(content.Images, src)
				seen[src] = true
			}
		})
	}
}

// extractPublishedDate извлекает дату публикации со страницы
func extractPublishedDate(doc *goquery.Document, content *NewsContent) {
	selectors := []string{
		"meta[property='article:published_time']",
		"meta[name='publishdate']",
		"meta[name='pubdate']",
		"[itemprop='datePublished']",
		".published",
		".article-date",
		".post-date",
		"time[datetime]",
	}

	for _, selector := range selectors {
		if strings.HasPrefix(selector, "meta") {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				if val, exists := s.Attr("content"); exists {
					if t, err := parseDate(val); err == nil {
						content.PublishedAt = &t
					}
				}
			})
		} else {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				// Пробуем атрибут datetime
				if datetime, exists := s.Attr("datetime"); exists {
					if t, err := parseDate(datetime); err == nil {
						content.PublishedAt = &t
					}
				} else {
					// Пробуем текст
					text := strings.TrimSpace(s.Text())
					if t, err := parseDate(text); err == nil {
						content.PublishedAt = &t
					}
				}
			})
		}

		if content.PublishedAt != nil {
			break
		}
	}
}

// extractContentHTML сохраняет HTML контента для будущего анализа
func extractContentHTML(doc *goquery.Document, content *NewsContent) {
	selectors := []string{
		"article .article-content",
		"article .content",
		"article .post-content",
		"article",
		"[itemprop='articleBody']",
	}

	for _, selector := range selectors {
		html, err := doc.Find(selector).First().Html()
		if err == nil && html != "" && len(html) > 100 {
			content.ContentHTML = html
			break
		}
	}
}

// parseDate пытается распарсить дату в различных форматах
func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02.01.2006 15:04",
		"02.01.2006",
		"January 2, 2006",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("не удалось распарсить дату: %s", dateStr)
}
