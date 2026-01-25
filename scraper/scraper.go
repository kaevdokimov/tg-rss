package scraper

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tg-rss/cache"
	"tg-rss/monitoring"

	readability "github.com/go-shiori/go-readability"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
)

var scraperLogger = monitoring.NewLogger("Scraper")

// Глобальный оптимизированный HTTP клиент для переиспользования соединений
var httpClient = &http.Client{
	Timeout: 15 * time.Second, // Сократили таймаут с 45 до 15 секунд
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20, // Увеличили с 10 до 20
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  false, // Включаем сжатие
	},
}

// SetDisableCompression отключает сжатие для HTTP клиента (для дебага)
func SetDisableCompression(disable bool) {
	httpClient.Transport.(*http.Transport).DisableCompression = disable
}

const maxContentSize = 2 * 1024 * 1024 // Максимальный размер контента 2MB

// peekReader позволяет заглянуть вперед в поток данных для определения типа сжатия
type peekReader struct {
	reader io.Reader
	buf    []byte
	pos    int
}

func (pr *peekReader) Read(p []byte) (n int, err error) {
	if pr.pos < len(pr.buf) {
		n = copy(p, pr.buf[pr.pos:])
		pr.pos += n
		return n, nil
	}
	return pr.reader.Read(p)
}

func (pr *peekReader) peek(n int) ([]byte, error) {
	if pr.pos >= len(pr.buf) {
		buf := make([]byte, n)
		m, err := pr.reader.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		pr.buf = append(pr.buf, buf[:m]...)
	}
	return pr.buf[:min(len(pr.buf), n)], nil
}

func (pr *peekReader) isGzip() bool {
	data, _ := pr.peek(2)
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

func (pr *peekReader) isDeflate() bool {
	data, _ := pr.peek(1)
	return len(data) >= 1 && data[0]&0x0f == 0x08 // ZLIB header
}

func (pr *peekReader) isBrotli() bool {
	data, _ := pr.peek(4)
	if len(data) < 4 {
		return false
	}
	// Brotli magic bytes: 0xCE, 0xB2, 0xCF, 0x81 (little-endian)
	return data[0] == 0xce && data[1] == 0xb2 && data[2] == 0xcf && data[3] == 0x81
}

// isCompressedData проверяет, являются ли данные сжатыми
func isCompressedData(reader io.Reader) bool {
	peekReader := &peekReader{reader: reader}
	data, _ := peekReader.peek(4)
	if len(data) < 2 {
		return false
	}

	// Gzip magic bytes: 0x1F 0x8B
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return true
	}

	// ZLIB (deflate) magic bytes: первый байт должен быть 0x78 (метод сжатия deflate)
	// Второй байт содержит уровень сжатия
	if len(data) >= 2 && data[0] == 0x78 && (data[1] == 0x01 || data[1] == 0x5e || data[1] == 0x9c || data[1] == 0xda) {
		return true
	}

	// Brotli magic bytes: 0xCE 0xB2 0xCF 0x81 (little-endian)
	if len(data) >= 4 && data[0] == 0xce && data[1] == 0xb2 && data[2] == 0xcf && data[3] == 0x81 {
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeNullBytes удаляет null байты из строки
func removeNullBytes(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// NewsContent содержит полный контент новости со страницы
type NewsContent struct {
	FullText        string            // Полный текст новости
	Author          string            // Автор статьи
	Category        string            // Категория новости
	Tags            []string          // Теги/ключевые слова
	Images          []string          // URL изображений из статьи
	PublishedAt     *time.Time        // Дата публикации со страницы (может отличаться от RSS)
	MetaKeywords    string            // Мета-тег keywords
	MetaDescription string            // Мета-тег description
	MetaData        map[string]string // Дополнительные метаданные
	ContentHTML     string            // HTML контента статьи (для будущего анализа)
}

// ScrapeNewsContent парсит страницу новости и извлекает полный контент
// Использует библиотеку go-readability (порт Mozilla Readability.js) для качественного извлечения контента
func ScrapeNewsContent(articleURL string) (*NewsContent, error) {
	scraperLogger.Debug("Начинаем парсинг страницы", "url", articleURL)

	// Проверяем кэш
	cacheKey := fmt.Sprintf("%x", md5.Sum([]byte(articleURL)))
	if cached, found := cache.ContentCache.Get(cacheKey); found {
		scraperLogger.Debug("Возвращаем контент из кэша", "url", articleURL)
		return cached.(*NewsContent), nil
	}

	var resp *http.Response
	var err error
	maxRetries := 3 // Увеличили количество попыток

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Создаем контекст с таймаутом для каждого запроса
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		req, reqErr := http.NewRequestWithContext(ctx, "GET", articleURL, nil)
		if reqErr != nil {
			return nil, fmt.Errorf("ошибка создания запроса: %w", reqErr)
		}

		// Добавляем заголовки для лучшей совместимости с сайтами
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NewsBot/1.0; +https://github.com/kaevdokimov/tg-rss)")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err = httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		if attempt < maxRetries-1 {
			// Экспоненциальная задержка между попытками
			delay := time.Duration(attempt+1) * time.Second
			scraperLogger.Debug("Попытка не удалась",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"url", articleURL,
				"error", err,
				"retry_delay", delay)
			time.Sleep(delay)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("ошибка загрузки страницы после %d попыток: %w", maxRetries, err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("неверный статус код после %d попыток: %d", maxRetries, resp.StatusCode)
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Читаем body с поддержкой сжатия и ограничением размера
	var reader io.Reader = resp.Body
	contentEncoding := resp.Header.Get("Content-Encoding")

	// Логируем заголовки для диагностики проблем со сжатием
	scraperLogger.Debug("HTTP Response - Status: %s, Content-Encoding: %s, Content-Type: %s, Content-Length: %s",
		resp.Status, contentEncoding, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"))

	switch strings.ToLower(contentEncoding) {
	case "gzip":
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ошибка создания gzip reader: %w", err)
		}
		defer func() { _ = gzipReader.Close() }()
		reader = gzipReader
		scraperLogger.Debug("Распаковываем данные с использованием gzip")

	case "deflate":
		flateReader := flate.NewReader(resp.Body)
		defer func() { _ = flateReader.Close() }()
		reader = flateReader
		scraperLogger.Debug("Распаковываем данные с использованием deflate")

	case "br", "brotli":
		brReader := brotli.NewReader(resp.Body)
		reader = brReader
		scraperLogger.Debug("Распаковываем данные с использованием brotli")

	case "":
		// Нет сжатия, используем тело как есть
		scraperLogger.Debug("Данные без сжатия")

	default:
		scraperLogger.Warn("Неизвестный тип сжатия, пробуем обработать как обычные данные",
			"content_encoding", contentEncoding)
		// Пробуем определить сжатие автоматически
		if isCompressedData(resp.Body) {
			scraperLogger.Debug("Обнаружены сжатые данные без указания Content-Encoding, пробуем распаковать")
			// Читаем первые байты для определения типа сжатия
			peekReader := &peekReader{reader: resp.Body}
			if peekReader.isGzip() {
				gzipReader, err := gzip.NewReader(peekReader)
				if err == nil {
					defer func() { _ = gzipReader.Close() }()
					reader = gzipReader
					scraperLogger.Debug("Автоматически распаковали как gzip")
				} else {
					reader = peekReader
				}
			} else if peekReader.isDeflate() {
				flateReader := flate.NewReader(peekReader)
				defer func() { _ = flateReader.Close() }()
				reader = flateReader
				scraperLogger.Debug("Автоматически распаковали как deflate")
			} else if peekReader.isBrotli() {
				brReader := brotli.NewReader(peekReader)
				reader = brReader
				scraperLogger.Debug("Автоматически распаковали как brotli")
			} else {
				reader = peekReader
				scraperLogger.Warn("Не удалось определить тип сжатия, используем данные как есть")
			}
		}
	}

	// Ограничиваем размер читаемых данных для предотвращения перегрузки памяти
	limitedReader := io.LimitReader(reader, maxContentSize)

	// Читаем body в память для повторного использования
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения body: %w", err)
	}

	// Проверяем, не был ли контент обрезан из-за лимита
	if len(bodyBytes) >= maxContentSize {
		scraperLogger.Warn("Контент страницы был обрезан из-за превышения лимита",
			"url", articleURL,
			"max_size_bytes", maxContentSize)
	}

	// Парсим URL для передачи в readability
	parsedURL, err := url.Parse(articleURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга URL: %w", err)
	}

	content := &NewsContent{
		MetaData: make(map[string]string),
		Tags:     []string{},
		Images:   []string{},
	}

	// Используем go-readability для извлечения читаемого контента
	// Это порт Mozilla Readability.js, который хорошо зарекомендовал себя
	article, err := readability.FromReader(bytes.NewReader(bodyBytes), parsedURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга страницы с помощью readability: %w", err)
	}

	// Извлекаем полный текст статьи (уже очищенный от лишнего и null байтов)
	content.FullText = removeNullBytes(strings.TrimSpace(article.TextContent))

	// Сохраняем HTML контента (очищенный от null байтов)
	content.ContentHTML = removeNullBytes(article.Content)

	// Извлекаем автора
	if article.Byline != "" {
		content.Author = removeNullBytes(strings.TrimSpace(article.Byline))
	}

	// Извлекаем метаданные из статьи
	if article.Excerpt != "" {
		content.MetaDescription = removeNullBytes(article.Excerpt)
	}

	// Извлекаем изображение статьи
	if article.Image != "" {
		content.Images = append(content.Images, removeNullBytes(article.Image))
	}

	// Парсим оригинальный HTML для извлечения метаданных из head
	// (readability уже очистил контент, но нам нужны метаданные из head)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err == nil {
		// Извлекаем дополнительные метаданные
		extractMetaData(doc, content)

		// Извлекаем категорию
		extractCategory(doc, content)

		// Извлекаем теги
		extractTags(doc, content)

		// Извлекаем дополнительные изображения
		extractImages(doc, content)

		// Извлекаем дату публикации
		extractPublishedDate(doc, content)

		// Если автор не найден через readability, пробуем через метаданные
		if content.Author == "" {
			extractAuthor(doc, content)
		}
	}

	scraperLogger.Debug("Парсинг завершен: текст=%d символов, изображений=%d, тегов=%d",
		len(content.FullText), len(content.Images), len(content.Tags))

	// Валидируем и санитизируем контент
	validator := NewContentValidator()
	if err := validator.ValidateAndSanitizeContent(content); err != nil {
		scraperLogger.Warn("Валидация контента не удалась",
			"url", articleURL,
			"error", err)
		// Возвращаем ошибку, так как контент не прошел валидацию
		return nil, fmt.Errorf("content validation failed: %w", err)
	}

	// Сохраняем в кэш
	cache.ContentCache.Set(cacheKey, content)
	scraperLogger.Debug("Контент сохранен в кэш", "url", articleURL)

	return content, nil
}

// extractMetaData извлекает метаданные из <meta> тегов
func extractMetaData(doc *goquery.Document, content *NewsContent) {
	// Keywords
	doc.Find("meta[name='keywords'], meta[property='keywords']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("content"); exists {
			content.MetaKeywords = removeNullBytes(val)
		}
	})

	// Description
	doc.Find("meta[name='description'], meta[property='description'], meta[property='og:description']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("content"); exists && content.MetaDescription == "" {
			content.MetaDescription = removeNullBytes(val)
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
				content.MetaData[key] = removeNullBytes(val)
			}
		}
	})
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
					content.Author = removeNullBytes(strings.TrimSpace(val))
				}
			})
		} else {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				if text != "" {
					content.Author = removeNullBytes(text)
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
					content.Category = removeNullBytes(strings.TrimSpace(val))
				}
			})
		} else {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				if text != "" {
					content.Category = removeNullBytes(text)
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
				text = removeNullBytes(text)
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
				src = removeNullBytes(src)
				// Преобразуем относительные URL в абсолютные
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				}
				// Относительные пути (/path) оставляем как есть - нужен базовый URL для полного разрешения
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
					val = removeNullBytes(val)
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
					text := removeNullBytes(strings.TrimSpace(s.Text()))
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
