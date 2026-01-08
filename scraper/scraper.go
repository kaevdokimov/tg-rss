package scraper

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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
		MaxIdleConnsPerHost: 20,  // Увеличили с 10 до 20
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
// Использует библиотеку go-readability (порт Mozilla Readability.js) для качественного извлечения контента
func ScrapeNewsContent(articleURL string) (*NewsContent, error) {
	scraperLogger.Debug("Начинаем парсинг страницы: %s", articleURL)

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
			scraperLogger.Debug("Попытка %d/%d не удалась для %s: %v. Повтор через %v", attempt+1, maxRetries, articleURL, err, delay)
			time.Sleep(delay)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("ошибка загрузки страницы после %d попыток: %w", maxRetries, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("неверный статус код после %d попыток: %d", maxRetries, resp.StatusCode)
		}
	}
	defer resp.Body.Close()

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
		defer gzipReader.Close()
		reader = gzipReader
		scraperLogger.Debug("Распаковываем данные с использованием gzip")

	case "deflate":
		flateReader := flate.NewReader(resp.Body)
		defer flateReader.Close()
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
		scraperLogger.Warn("Неизвестный тип сжатия: %s, пробуем обработать как обычные данные", contentEncoding)
		// Пробуем определить сжатие автоматически
		if isCompressedData(resp.Body) {
			scraperLogger.Debug("Обнаружены сжатые данные без указания Content-Encoding, пробуем распаковать")
			// Читаем первые байты для определения типа сжатия
			peekReader := &peekReader{reader: resp.Body}
			if peekReader.isGzip() {
				gzipReader, err := gzip.NewReader(peekReader)
				if err == nil {
					defer gzipReader.Close()
					reader = gzipReader
					scraperLogger.Debug("Автоматически распаковали как gzip")
				} else {
					reader = peekReader
				}
			} else if peekReader.isDeflate() {
				flateReader := flate.NewReader(peekReader)
				defer flateReader.Close()
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
		scraperLogger.Warn("Контент страницы %s был обрезан из-за превышения лимита %d байт", articleURL, maxContentSize)
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

// extractFullText извлекает полный текст статьи, убирая лишнее
// DEPRECATED: Используется go-readability, эта функция больше не вызывается
// Оставлена для справки или как fallback
func extractFullText(doc *goquery.Document, content *NewsContent) {
	// Популярные селекторы для контента статьи (в порядке приоритета)
	selectors := []string{
		"article .article-content",
		"article .article-body",
		"article .post-content",
		"article .entry-content",
		"article .content",
		"article .news-content",
		"article .text-content",
		"[itemprop='articleBody']",
		".article-content",
		".article-body",
		".post-content",
		".entry-content",
		".news-content",
		"#article-content",
		"#article-body",
		"article main",
		"main article",
		"[role='article']",
		"article",
	}

	var articleContent *goquery.Selection

	// Ищем контейнер статьи
	for _, selector := range selectors {
		selection := doc.Find(selector).First()
		if selection.Length() > 0 {
			// Проверяем, что это действительно контент статьи (достаточно текста)
			text := strings.TrimSpace(selection.Clone().Find("script, style, noscript, nav, header, footer, aside, .ad, .advertisement, .comments, .social, .share").Remove().Text())
			if len(text) > 200 { // Минимальная длина для валидной статьи
				articleContent = selection
				break
			}
		}
	}

	// Если не нашли специфичный контейнер, пробуем найти по структурированным данным
	if articleContent == nil || articleContent.Length() == 0 {
		articleContent = doc.Find("[itemprop='articleBody'], article, main").First()
	}

	// Если все еще не нашли, используем body как последний вариант
	if articleContent == nil || articleContent.Length() == 0 {
		articleContent = doc.Find("body")
	}

	if articleContent.Length() == 0 {
		content.FullText = ""
		return
	}

	// Клонируем элемент, чтобы не изменять оригинальный DOM
	cleanContent := articleContent.Clone()

	// Удаляем все лишние элементы, которые не являются частью статьи
	cleanContent.Find(`
		script, style, noscript,
		nav, header, footer, aside,
		.ad, .advertisement, .ads, .advert,
		.comments, .comment-section, .comments-section,
		.social, .social-share, .share, .share-buttons,
		.menu, .navigation, .navbar, .nav-menu,
		.breadcrumb, .breadcrumbs,
		.related, .related-articles, .related-posts,
		.subscribe, .newsletter,
		.author-box, .author-info,
		.tags, .tag-list,
		iframe, embed, object,
		[class*="ad"], [class*="advert"], [id*="ad"], [id*="advert"],
		[class*="comment"], [id*="comment"],
		[class*="social"], [id*="social"],
		[class*="share"], [id*="share"],
		[class*="menu"], [id*="menu"],
		[class*="nav"], [id*="nav"],
		[class*="footer"], [id*="footer"],
		[class*="header"], [id*="header"],
		[class*="sidebar"], [id*="sidebar"]
	`).Remove()

	// Извлекаем только параграфы и заголовки из статьи
	var textParts []string
	
	// Ищем параграфы внутри контента статьи
	cleanContent.Find("p, h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		
		// Фильтруем короткие и нерелевантные тексты
		if len(text) < 30 {
			return
		}
		
		// Пропускаем тексты, которые выглядят как навигация или реклама
		if isNonContentText(text) {
			return
		}
		
		textParts = append(textParts, text)
	})

	// Если параграфов мало, пробуем извлечь текст напрямую, но более аккуратно
	if len(textParts) < 3 {
		// Разбиваем на строки и фильтруем
		fullText := strings.TrimSpace(cleanContent.Text())
		lines := strings.Split(fullText, "\n")
		
		var filteredLines []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 50 && !isNonContentText(line) {
				filteredLines = append(filteredLines, line)
			}
		}
		
		if len(filteredLines) > len(textParts) {
			textParts = filteredLines
		}
	}

	// Объединяем части текста
	content.FullText = strings.Join(textParts, "\n\n")
	content.FullText = strings.TrimSpace(content.FullText)

	// Очищаем от лишних пробелов и переносов
	content.FullText = removeNullBytes(cleanText(content.FullText))
}

// isNonContentText проверяет, является ли текст навигацией, рекламой или другим нерелевантным контентом
func isNonContentText(text string) bool {
	text = strings.ToLower(text)
	
	// Паттерны, указывающие на нерелевантный контент
	nonContentPatterns := []string{
		"читать также",
		"подпишитесь",
		"подписаться",
		"реклама",
		"advertisement",
		"cookie",
		"куки",
		"принять",
		"согласен",
		"продолжить",
		"далее",
		"следующая",
		"предыдущая",
		"главная",
		"home",
		"меню",
		"menu",
		"войти",
		"login",
		"регистрация",
		"register",
		"поиск",
		"search",
		"комментарии",
		"comments",
		"поделиться",
		"share",
		"лайк",
		"like",
		"подпис",
		"follow",
		"связанные",
		"related",
		"рекомендуем",
		"recommended",
		"новости",
		"news",
		"архив",
		"archive",
		"контакты",
		"contacts",
		"о нас",
		"about",
		"политика",
		"policy",
		"условия",
		"terms",
		"©",
		"copyright",
		"все права",
		"all rights",
		"facebook",
		"twitter",
		"instagram",
		"telegram",
		"vk.com",
		"youtube",
		"rss",
		"xml",
		"atom",
	}
	
	for _, pattern := range nonContentPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	
	// Проверяем, слишком ли короткий текст или состоит только из заглавных букв (часто реклама)
	if len(text) < 20 {
		return true
	}
	
	upperCount := 0
	for _, r := range text {
		if r >= 'А' && r <= 'Я' || r >= 'A' && r <= 'Z' {
			upperCount++
		}
	}
	if float64(upperCount)/float64(len(text)) > 0.7 && len(text) < 100 {
		return true
	}
	
	return false
}

// cleanText очищает текст от лишних пробелов и переносов
func cleanText(text string) string {
	// Удаляем множественные пробелы
	text = strings.ReplaceAll(text, "  ", " ")
	text = strings.ReplaceAll(text, "   ", " ")
	
	// Удаляем множественные переносы строк
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}
	
	// Объединяем обратно, оставляя по одному переносу между параграфами
	result := strings.Join(cleanedLines, "\n\n")
	
	// Удаляем множественные переносы в конце
	for strings.HasSuffix(result, "\n\n") {
		result = strings.TrimSuffix(result, "\n\n")
	}
	
	return result
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

// extractContentHTML сохраняет HTML контента для будущего анализа
func extractContentHTML(doc *goquery.Document, content *NewsContent) {
	// Используем те же селекторы, что и для извлечения текста
	selectors := []string{
		"article .article-content",
		"article .article-body",
		"article .post-content",
		"article .entry-content",
		"[itemprop='articleBody']",
		"article",
		"main article",
	}

	var articleContent *goquery.Selection

	// Ищем контейнер статьи
	for _, selector := range selectors {
		selection := doc.Find(selector).First()
		if selection.Length() > 0 {
			text := strings.TrimSpace(selection.Clone().Find("script, style, noscript, nav, header, footer, aside, .ad, .advertisement, .comments, .social, .share").Remove().Text())
			if len(text) > 200 {
				articleContent = selection
				break
			}
		}
	}

	if articleContent == nil || articleContent.Length() == 0 {
		articleContent = doc.Find("[itemprop='articleBody'], article, main").First()
	}

	if articleContent.Length() == 0 {
		content.ContentHTML = ""
		return
	}

	// Клонируем и очищаем от лишних элементов
	cleanContent := articleContent.Clone()
	cleanContent.Find(`
		script, style, noscript,
		nav, header, footer, aside,
		.ad, .advertisement, .ads, .advert,
		.comments, .comment-section, .comments-section,
		.social, .social-share, .share, .share-buttons,
		.menu, .navigation, .navbar, .nav-menu,
		.breadcrumb, .breadcrumbs,
		.related, .related-articles, .related-posts,
		.subscribe, .newsletter,
		iframe, embed, object,
		[class*="ad"], [class*="advert"], [id*="ad"], [id*="advert"],
		[class*="comment"], [id*="comment"],
		[class*="social"], [id*="social"],
		[class*="share"], [id*="share"],
		[class*="menu"], [id*="menu"],
		[class*="nav"], [id*="nav"],
		[class*="footer"], [id*="footer"],
		[class*="header"], [id*="header"],
		[class*="sidebar"], [id*="sidebar"]
	`).Remove()

	html, err := cleanContent.Html()
	if err == nil && html != "" && len(html) > 100 {
		content.ContentHTML = removeNullBytes(html)
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
