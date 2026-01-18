package bot

import (
	"database/sql"
	"encoding/json"
	"sync"
	"tg-rss/db"
	"tg-rss/monitoring"
	"tg-rss/redis"
	"tg-rss/scraper"
	"time"
)

var contentScraperLogger = monitoring.NewLogger("ContentScraper")

// RedisCacheTTL определен в constants.go

// convertToCachedNewsContent конвертирует scraper.NewsContent в redis.CachedNewsContent
func convertToCachedNewsContent(content *scraper.NewsContent) *redis.CachedNewsContent {
	return &redis.CachedNewsContent{
		FullText:        content.FullText,
		Author:          content.Author,
		Category:        content.Category,
		Tags:            content.Tags,
		Images:          content.Images,
		PublishedAt:     content.PublishedAt,
		MetaKeywords:    content.MetaKeywords,
		MetaDescription: content.MetaDescription,
		MetaData:        content.MetaData,
		ContentHTML:     content.ContentHTML,
	}
}

// convertFromCachedNewsContent конвертирует redis.CachedNewsContent в scraper.NewsContent
func convertFromCachedNewsContent(cached *redis.CachedNewsContent) *scraper.NewsContent {
	return &scraper.NewsContent{
		FullText:        cached.FullText,
		Author:          cached.Author,
		Category:        cached.Category,
		Tags:            cached.Tags,
		Images:          cached.Images,
		PublishedAt:     cached.PublishedAt,
		MetaKeywords:    cached.MetaKeywords,
		MetaDescription: cached.MetaDescription,
		MetaData:        cached.MetaData,
		ContentHTML:     cached.ContentHTML,
	}
}

// ContentScraper обрабатывает фоновый парсинг страниц новостей
type ContentScraper struct {
	db         *sql.DB
	interval   time.Duration
	batchSize  int
	concurrent int                 // количество одновременных запросов
	cache      *redis.ContentCache // Redis кэш для контента
}

// NewContentScraper создает новый обработчик парсинга контента
func NewContentScraper(db *sql.DB, interval time.Duration, batchSize, concurrent int, cache *redis.ContentCache) *ContentScraper {
	return &ContentScraper{
		db:         db,
		interval:   interval,
		batchSize:  batchSize,
		concurrent: concurrent,
		cache:      cache,
	}
}

// Start запускает фоновый процесс парсинга контента
func (cs *ContentScraper) Start() {
	contentScraperLogger.Info("Запуск фонового парсера контента: интервал=%v, размер батча=%d, параллельно=%d",
		cs.interval, cs.batchSize, cs.concurrent)

	// Создаем тикер с начальной задержкой в 1 минуту
	ticker := time.NewTicker(cs.interval)
	defer ticker.Stop()

	// Первый запуск через ContentScraperDelay после старта
	go func() {
		time.Sleep(ContentScraperDelay)
		cs.scrapeBatch()
	}()

	// Затем запускаем по расписанию
	for range ticker.C {
		cs.scrapeBatch()
	}
}

// scrapeBatch обрабатывает батч новостей для парсинга
func (cs *ContentScraper) scrapeBatch() {
	contentScraperLogger.Info("Начинаем парсинг батча новостей")

	// Получаем список новостей для парсинга
	newsList, err := db.GetNewsForScraping(cs.db, cs.batchSize)
	if err != nil {
		contentScraperLogger.Error("Ошибка получения новостей для парсинга", "error", err)
		return
	}

	if len(newsList) == 0 {
		contentScraperLogger.Debug("Нет новостей для парсинга")
		return
	}

	contentScraperLogger.Info("Найдено новостей для парсинга", "news_count", len(newsList))

	// Создаем канал для ограничения параллелизма
	semaphore := make(chan struct{}, cs.concurrent)
	results := make(chan scrapeResult, len(newsList))
	var wg sync.WaitGroup

	// Запускаем парсинг параллельно
	for i, news := range newsList {
		wg.Add(1)
		go func(n db.NewsForScraping, idx int) {
			defer wg.Done()

			// Занимаем слот в семафоре
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Добавляем минимальную задержку между запросами для избежания rate limiting
			// Задержка зависит от индекса в батче, но не превышает разумных пределов
			if idx > 0 {
				delay := time.Duration(min(idx, 10)) * 100 * time.Millisecond // максимум 1 секунда задержки
				time.Sleep(delay)
			}

			cs.scrapeNews(n, results)
		}(news, i)
	}

	// Закрываем канал результатов после завершения всех горутин
	go func() {
		wg.Wait()
		close(results)
	}()

	// Ждем завершения всех горутин
	wg.Wait()

	// Собираем результаты
	successCount := 0
	failCount := 0
	for result := range results {
		if result.success {
			successCount++
		} else {
			failCount++
		}
	}

	contentScraperLogger.Info("Парсинг завершен",
		"success_count", successCount,
		"error_count", failCount)
}

type scrapeResult struct {
	success bool
}

// scrapeNews парсит одну новость
func (cs *ContentScraper) scrapeNews(news db.NewsForScraping, results chan<- scrapeResult) {
	contentScraperLogger.Debug("Парсинг новости",
		"news_id", news.ID,
		"url", news.Link)

	// Сначала проверяем Redis кэш
	var content *scraper.NewsContent
	if cs.cache != nil {
		if cached, found := cs.cache.Get(news.Link); found {
			contentScraperLogger.Debug("Контент новости найден в Redis кэше",
				"news_id", news.ID)
			content = convertFromCachedNewsContent(cached)
		}
	}

	// Если не найдено в кэше, парсим страницу
	if content == nil {
		var err error
		content, err = scraper.ScrapeNewsContent(news.Link)
		if err != nil {
			contentScraperLogger.Warn("Ошибка парсинга новости",
				"news_id", news.ID,
				"error", err)
			// Сохраняем ошибку
			if saveErr := db.MarkNewsScrapeFailed(cs.db, news.ID, err.Error()); saveErr != nil {
				contentScraperLogger.Error("Ошибка сохранения статуса ошибки для новости",
					"news_id", news.ID,
					"error", saveErr)
			}
			results <- scrapeResult{success: false}
			return
		}

		// Сохраняем в Redis кэш
		if cs.cache != nil {
			cachedContent := convertToCachedNewsContent(content)
			if err := cs.cache.Set(news.Link, cachedContent, RedisCacheTTL); err != nil {
				contentScraperLogger.Warn("Ошибка сохранения в Redis кэш для новости",
					"news_id", news.ID,
					"error", err)
			} else {
				contentScraperLogger.Debug("Контент новости сохранен в Redis кэш",
					"news_id", news.ID)
			}
		}
	}

	// Преобразуем metaData в map[string]string для сохранения
	metaDataMap := make(map[string]string)
	if content.MetaData != nil {
		metaDataMap = content.MetaData
	}

	// Сохраняем контент
	err := db.SaveNewsContent(
		cs.db,
		news.ID,
		content.FullText,
		content.Author,
		content.Category,
		content.Tags,
		content.Images,
		content.MetaKeywords,
		content.MetaDescription,
		metaDataMap,
		content.ContentHTML,
	)

	if err != nil {
		contentScraperLogger.Error("Ошибка сохранения контента новости",
			"news_id", news.ID,
			"error", err)
		if saveErr := db.MarkNewsScrapeFailed(cs.db, news.ID, err.Error()); saveErr != nil {
			contentScraperLogger.Error("Ошибка сохранения статуса ошибки для новости",
				"news_id", news.ID,
				"error", saveErr)
		}
		results <- scrapeResult{success: false}
		return
	}

	contentScraperLogger.Debug("Контент новости ID=%d успешно сохранен: текст=%d символов, изображений=%d, тегов=%d",
		news.ID, len(content.FullText), len(content.Images), len(content.Tags))

	results <- scrapeResult{success: true}
}

// GetNewsContentJSON возвращает контент новости в формате JSON для Python анализа
func GetNewsContentJSON(db *sql.DB, newsID int64) (string, error) {
	query := `
		SELECT 
			id, title, description, link, published_at,
			full_text, author, category, tags, images,
			meta_keywords, meta_description, meta_data, content_html,
			scraped_at, scrape_status
		FROM news
		WHERE id = $1
	`

	var news struct {
		ID              int64
		Title           string
		Description     string
		Link            string
		PublishedAt     time.Time
		FullText        sql.NullString
		Author          sql.NullString
		Category        sql.NullString
		Tags            []string
		Images          []string
		MetaKeywords    sql.NullString
		MetaDescription sql.NullString
		MetaData        sql.NullString
		ContentHTML     sql.NullString
		ScrapedAt       sql.NullTime
		ScrapeStatus    sql.NullString
	}

	err := db.QueryRow(query, newsID).Scan(
		&news.ID, &news.Title, &news.Description, &news.Link, &news.PublishedAt,
		&news.FullText, &news.Author, &news.Category, &news.Tags, &news.Images,
		&news.MetaKeywords, &news.MetaDescription, &news.MetaData, &news.ContentHTML,
		&news.ScrapedAt, &news.ScrapeStatus,
	)

	if err != nil {
		return "", err
	}

	// Преобразуем в JSON
	jsonBytes, err := json.Marshal(news)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
