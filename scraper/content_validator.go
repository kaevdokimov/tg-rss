package scraper

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	// monitoring functions will be implemented locally
)

// ContentValidator валидирует и санитизирует scraped контент
type ContentValidator struct{}

// NewContentValidator создает новый валидатор контента
func NewContentValidator() *ContentValidator {
	return &ContentValidator{}
}

// ValidateAndSanitizeContent валидирует и санитизирует контент новости
func (cv *ContentValidator) ValidateAndSanitizeContent(content *NewsContent) error {
	if content == nil {
		return ErrInvalidContent
	}

	// Валидируем и санитизируем заголовок
	content.FullText = cv.sanitizeText(content.FullText)
	if err := cv.validateText(content.FullText, "full_text", maxContentSize); err != nil {
		// monitoring.IncrementContentValidationErrors("full_text")
		return err
	}

	// Валидируем и санитизируем автора
	content.Author = cv.sanitizeText(content.Author)
	if content.Author != "" {
		if err := cv.validateText(content.Author, "author", 255); err != nil {
			// monitoring.IncrementContentValidationErrors("author")
			return err
		}
	}

	// Валидируем и санитизируем категорию
	content.Category = cv.sanitizeText(content.Category)
	if content.Category != "" {
		if err := cv.validateText(content.Category, "category", 255); err != nil {
			// monitoring.IncrementContentValidationErrors("category")
			return err
		}
	}

	// Валидируем и санитизируем HTML контент
	content.ContentHTML = cv.sanitizeHTML(content.ContentHTML)
	if content.ContentHTML != "" {
		if err := cv.validateText(content.ContentHTML, "content_html", maxContentSize); err != nil {
			// monitoring.IncrementContentValidationErrors("content_html")
			return err
		}
	}

	// Валидируем и санитизируем мета-данные
	content.MetaKeywords = cv.sanitizeText(content.MetaKeywords)
	content.MetaDescription = cv.sanitizeText(content.MetaDescription)

	if content.MetaKeywords != "" {
		if err := cv.validateText(content.MetaKeywords, "meta_keywords", 1024); err != nil {
			// monitoring.IncrementContentValidationErrors("meta_keywords")
			return err
		}
	}

	if content.MetaDescription != "" {
		if err := cv.validateText(content.MetaDescription, "meta_description", 1024); err != nil {
			// monitoring.IncrementContentValidationErrors("meta_description")
			return err
		}
	}

	// Валидируем URL'ы изображений
	validImages := make([]string, 0, len(content.Images))
	for _, imgURL := range content.Images {
		if cv.validateImageURL(imgURL) {
			validImages = append(validImages, imgURL)
			// Валидный URL изображения - добавляем в список
		}
		// Недействительный URL изображения - пропускаем без ошибок
	}
	content.Images = validImages

	// Валидируем теги
	validTags := make([]string, 0, len(content.Tags))
	for _, tag := range content.Tags {
		sanitizedTag := cv.sanitizeText(tag)
		if sanitizedTag != "" && len(sanitizedTag) <= 100 {
			validTags = append(validTags, sanitizedTag)
			// Валидный тег - добавляем в список
		}
		// Недействительный тег (пустой или слишком длинный) - пропускаем
	}
	content.Tags = validTags

	// Валидируем мета-данные
	if content.MetaData != nil {
		validMetaData := make(map[string]string)
		for key, value := range content.MetaData {
			if cv.validateMetaKey(key) && cv.validateMetaValue(value) {
			validMetaData[key] = cv.sanitizeText(value)
			// Валидная мета-данная - добавляем в список
		}
		// Недействительная мета-данная - пропускаем
		}
		content.MetaData = validMetaData
	}

	// monitoring.IncrementContentValidations()
	return nil
}

// sanitizeText санитизирует текст от потенциально опасных символов
func (cv *ContentValidator) sanitizeText(text string) string {
	if text == "" {
		return ""
	}

	// HTML escape
	text = html.EscapeString(text)

	// Удаляем null байты
	text = strings.ReplaceAll(text, "\x00", "")

	// Удаляем другие опасные символы
	text = strings.Map(func(r rune) rune {
		// Удаляем control characters кроме \n, \r, \t
		if r < 32 && r != 9 && r != 10 && r != 13 {
			return -1
		}
		return r
	}, text)

	return strings.TrimSpace(text)
}

// sanitizeHTML санитизирует HTML контент
func (cv *ContentValidator) sanitizeHTML(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	// Удаляем потенциально опасные теги и атрибуты
	dangerousPatterns := []string{
		`<script[^>]*>.*?</script>`,
		`<style[^>]*>.*?</style>`,
		`<iframe[^>]*>.*?</iframe>`,
		`<object[^>]*>.*?</object>`,
		`<embed[^>]*>.*?</embed>`,
		`<form[^>]*>.*?</form>`,
		`<input[^>]*>`,
		`<button[^>]*>.*?</button>`,
		`on\w+="[^"]*"`, // JavaScript event handlers
		`javascript:`,
		`vbscript:`,
		`data:`,
	}

	for _, pattern := range dangerousPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		htmlContent = re.ReplaceAllString(htmlContent, "")
	}

	// Удаляем null байты
	htmlContent = strings.ReplaceAll(htmlContent, "\x00", "")

	return htmlContent
}

// validateText валидирует текст на соответствие ограничениям
func (cv *ContentValidator) validateText(text, fieldName string, maxLength int) error {
	if text == "" {
		return nil // Пустой текст допустим
	}

	if !utf8.ValidString(text) {
		return NewContentValidationError(fieldName, "invalid UTF-8 encoding")
	}

	if len(text) > maxLength {
		return NewContentValidationError(fieldName, "exceeds maximum length")
	}

	// Проверяем на наличие слишком длинных последовательностей одинаковых символов
	if cv.hasRepeatedChars(text, 100) {
		return NewContentValidationError(fieldName, "contains too many repeated characters")
	}

	return nil
}

// validateImageURL валидирует URL изображения
func (cv *ContentValidator) validateImageURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Проверяем схему
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// Проверяем хост
	if u.Host == "" {
		return false
	}

	// Проверяем расширение файла (должно быть изображением)
	validExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"}
	path := strings.ToLower(u.Path)
	for _, ext := range validExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	// Также принимаем URL без расширения (динамические изображения)
	return len(path) > 0
}

// validateMetaKey валидирует ключ мета-данных
func (cv *ContentValidator) validateMetaKey(key string) bool {
	if key == "" || len(key) > 100 {
		return false
	}

	// Ключ должен содержать только буквы, цифры, дефисы и подчеркивания
	validKey := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return validKey.MatchString(key)
}

// validateMetaValue валидирует значение мета-данных
func (cv *ContentValidator) validateMetaValue(value string) bool {
	if len(value) > 1000 {
		return false
	}

	// Проверяем на наличие опасных символов
	dangerousChars := []string{"<", ">", "&", "\"", "'", "\x00"}
	for _, char := range dangerousChars {
		if strings.Contains(value, char) {
			return false
		}
	}

	return true
}

// hasRepeatedChars проверяет наличие длинных последовательностей повторяющихся символов
func (cv *ContentValidator) hasRepeatedChars(text string, threshold int) bool {
	if len(text) < threshold {
		return false
	}

	currentChar := text[0]
	count := 1

	for i := 1; i < len(text); i++ {
		if text[i] == currentChar {
			count++
			if count >= threshold {
				return true
			}
		} else {
			currentChar = text[i]
			count = 1
		}
	}

	return false
}

// ContentValidationError ошибка валидации контента
type ContentValidationError struct {
	Field   string
	Message string
}

func NewContentValidationError(field, message string) *ContentValidationError {
	return &ContentValidationError{
		Field:   field,
		Message: message,
	}
}

func (e *ContentValidationError) Error() string {
	return "content validation failed for " + e.Field + ": " + e.Message
}

// ErrInvalidContent ошибка для недопустимого контента
var ErrInvalidContent = NewContentValidationError("content", "invalid content")

// Глобальный валидатор контента
var GlobalContentValidator = NewContentValidator()