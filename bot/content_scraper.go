package bot

import (
	"database/sql"
	"encoding/json"
	"tg-rss/db"
	"tg-rss/monitoring"
	"tg-rss/scraper"
	"time"
)

var contentScraperLogger = monitoring.NewLogger("ContentScraper")

// ContentScraper обрабатывает фоновый парсинг страниц новостей
type ContentScraper struct {
	db         *sql.DB
	interval   time.Duration
	batchSize  int
	concurrent int // количество одновременных запросов
}

// NewContentScraper создает новый обработчик парсинга контента
func NewContentScraper(db *sql.DB, interval time.Duration, batchSize, concurrent int) *ContentScraper {
	return &ContentScraper{
		db:         db,
		interval:   interval,
		batchSize:  batchSize,
		concurrent: concurrent,
	}
}

// Start запускает фоновый процесс парсинга контента
func (cs *ContentScraper) Start() {
	contentScraperLogger.Info("Запуск фонового парсера контента: интервал=%v, размер батча=%d, параллельно=%d",
		cs.interval, cs.batchSize, cs.concurrent)

	// Первый запуск через 1 минуту после старта
	time.Sleep(1 * time.Minute)
	cs.scrapeBatch()

	// Затем запускаем по расписанию
	ticker := time.NewTicker(cs.interval)
	defer ticker.Stop()

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
		contentScraperLogger.Error("Ошибка получения новостей для парсинга: %v", err)
		return
	}

	if len(newsList) == 0 {
		contentScraperLogger.Debug("Нет новостей для парсинга")
		return
	}

	contentScraperLogger.Info("Найдено %d новостей для парсинга", len(newsList))

	// Создаем канал для ограничения параллелизма
	semaphore := make(chan struct{}, cs.concurrent)
	results := make(chan scrapeResult, len(newsList))

	// Запускаем парсинг параллельно с задержкой между запросами
	for i, news := range newsList {
		semaphore <- struct{}{} // занимаем слот
		go func(n db.NewsForScraping, idx int) {
			defer func() { <-semaphore }() // освобождаем слот

			// Добавляем задержку между запросами для избежания rate limiting
			if idx > 0 {
				time.Sleep(time.Duration(idx%cs.concurrent) * 200 * time.Millisecond)
			}

			cs.scrapeNews(n, results)
		}(news, i)
	}

	// Ждем завершения всех горутин
	for i := 0; i < cs.concurrent; i++ {
		semaphore <- struct{}{}
	}

	close(results)

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

	contentScraperLogger.Info("Парсинг завершен: успешно=%d, ошибок=%d", successCount, failCount)
}

type scrapeResult struct {
	success bool
}

// scrapeNews парсит одну новость
func (cs *ContentScraper) scrapeNews(news db.NewsForScraping, results chan<- scrapeResult) {
	contentScraperLogger.Debug("Парсинг новости ID=%d: %s", news.ID, news.Link)

	// Парсим страницу
	content, err := scraper.ScrapeNewsContent(news.Link)
	if err != nil {
		contentScraperLogger.Warn("Ошибка парсинга новости ID=%d: %v", news.ID, err)
		// Сохраняем ошибку
		if saveErr := db.MarkNewsScrapeFailed(cs.db, news.ID, err.Error()); saveErr != nil {
			contentScraperLogger.Error("Ошибка сохранения статуса ошибки для новости ID=%d: %v", news.ID, saveErr)
		}
		results <- scrapeResult{success: false}
		return
	}

	// Преобразуем metaData в map[string]string для сохранения
	metaDataMap := make(map[string]string)
	if content.MetaData != nil {
		metaDataMap = content.MetaData
	}

	// Сохраняем контент
	err = db.SaveNewsContent(
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
		contentScraperLogger.Error("Ошибка сохранения контента новости ID=%d: %v", news.ID, err)
		if saveErr := db.MarkNewsScrapeFailed(cs.db, news.ID, err.Error()); saveErr != nil {
			contentScraperLogger.Error("Ошибка сохранения статуса ошибки для новости ID=%d: %v", news.ID, saveErr)
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
