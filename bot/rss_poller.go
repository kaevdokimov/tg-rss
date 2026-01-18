package bot

import (
	"database/sql"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/lib/pq"
	"tg-rss/db"
	"tg-rss/monitoring"
	"tg-rss/redis"
	"tg-rss/rss"
)

var rssLogger = monitoring.NewLogger("RSS")

// Оптимизированный HTTP клиент для RSS парсинга
var rssHttpClient = &http.Client{
	Timeout: HTTPTimeout, // Короткий таймаут для RSS
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  false,
	},
}

// parseResult результат парсинга одного источника
type parseResult struct {
	source   db.Source
	newsList []rss.News
	err      error
}

// newsCandidate представляет кандидата новости для обработки
type newsCandidate struct {
	source db.Source
	item   rss.News
}

// parseSource парсит один RSS источник
func parseSource(source db.Source, tz *time.Location) parseResult {
	newsList, err := rss.ParseRSSWithClient(source.Url, tz, rssHttpClient)
	return parseResult{
		source:   source,
		newsList: newsList,
		err:      err,
	}
}

// min возвращает минимальное из двух целых чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// StartRSSPolling запускает регулярный опрос RSS-источников
func StartRSSPolling(dbConn *sql.DB, interval time.Duration, tz *time.Location, redisProducer *redis.Producer) {
	// Кэшируем источники для снижения нагрузки на БД
	// Обновляем кэш каждые SourcesCacheTTL
	var sourcesCache []db.Source
	var lastCacheUpdate time.Time
	rssLogger.Info("Запуск RSS парсера с интервалом %v", interval)

	// Создаем тикер для периодического выполнения вместо блокирующего sleep
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Запускаем первый цикл сразу, без ожидания
	firstRun := true

	// Защита от паники - логируем ошибку, но не перезапускаем рекурсивно
	defer func() {
		if r := recover(); r != nil {
			rssLogger.Error("КРИТИЧЕСКАЯ ОШИБКА в RSS парсере: %v. RSS парсинг остановлен", r)
			// Не перезапускаем автоматически, чтобы избежать утечки горутин
			// Перезапуск должен происходить на уровне оркестратора (systemd, docker, etc.)
		}
	}()

	// Первый цикл выполняется сразу
	if err := runPollingCycle(dbConn, tz, redisProducer, &sourcesCache, &lastCacheUpdate, &firstRun); err != nil {
		rssLogger.Error("Ошибка в первом цикле парсинга: %v", err)
	}

	// Последующие циклы по таймеру
	for range ticker.C {
		if err := runPollingCycle(dbConn, tz, redisProducer, &sourcesCache, &lastCacheUpdate, &firstRun); err != nil {
			rssLogger.Error("Ошибка в цикле парсинга: %v", err)
		}
	}
}

// runPollingCycle выполняет один цикл парсинга RSS
func runPollingCycle(dbConn *sql.DB, tz *time.Location, redisProducer *redis.Producer,
	sourcesCache *[]db.Source, lastCacheUpdate *time.Time, firstRun *bool) error {

	monitoring.IncrementRSSPolls()
	rssLogger.Info("Начало цикла парсинга RSS-источников")

	// Используем кэшированные источники или обновляем кэш
	var sources []db.Source
	if time.Since(*lastCacheUpdate) > SourcesCacheTTL || len(*sourcesCache) == 0 {
		rssLogger.Debug("Обновление кэша источников")
		var err error
		sources, err = fetchSources(dbConn)
		if err != nil {
			monitoring.IncrementRSSPollsErrors()
			rssLogger.Error("Ошибка при получении источников: %v", err)
			return err
		}
		*sourcesCache = sources
		*lastCacheUpdate = time.Now()
		rssLogger.Info("Кэш источников обновлен: %d источников", len(sources))
	} else {
		sources = *sourcesCache
		rssLogger.Debug("Используем кэшированные источники: %d источников", len(sources))
	}

	rssLogger.Info("Найдено активных источников: %d", len(sources))

	totalNewsFound := 0
	totalNewsSent := 0
	sourcesProcessed := 0
	sourcesWithErrors := 0

	// Оптимизация: параллельная обработка источников
	// Ограничиваем количество одновременных запросов для избежания перегрузки
	maxWorkers := min(6, runtime.NumCPU()*2) // максимум 6 воркеров или 2xCPU
	if len(sources) < maxWorkers {
		maxWorkers = len(sources)
	}

	rssLogger.Debug("Запуск параллельного парсинга с %d воркерами", maxWorkers)

	// Каналы для коммуникации между воркерами
	jobs := make(chan db.Source, len(sources))
	results := make(chan parseResult, len(sources))

	// Запускаем воркеры
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			rssLogger.Debug("Воркер %d запущен", workerID)
			for source := range jobs {
				result := parseSource(source, tz)
				results <- result
			}
			rssLogger.Debug("Воркер %d завершен", workerID)
		}(i)
	}

	// Отправляем задания воркерам
	for _, source := range sources {
		jobs <- source
	}
	close(jobs)

	// Собираем результаты
	var candidates []newsCandidate

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.err != nil {
			monitoring.IncrementRSSPollsErrors()
			sourcesWithErrors++
			rssLogger.Warn("Ошибка парсинга RSS для источника %s (%s): %v", result.source.Name, result.source.Url, result.err)
			continue
		}

		sourcesProcessed++
		totalNewsFound += len(result.newsList)
		rssLogger.Debug("Источник %s: найдено новостей %d", result.source.Name, len(result.newsList))

		for _, item := range result.newsList {
			// Пропускаем старые новости (старше MaxNewsAge)
			if time.Since(item.PublishedAt) > MaxNewsAge {
				continue
			}

			// Собираем кандидатов для батч-проверки дубликатов
			candidates = append(candidates, newsCandidate{
				source: result.source,
				item:   item,
			})
		}
	}

	// Оптимизация: батч-проверка дубликатов и отправка в Redis
	if len(candidates) > 0 {
		totalNewsSent += processCandidatesBatch(dbConn, redisProducer, candidates)
	}

	rssLogger.Info("Цикл парсинга завершен: обработано источников %d/%d, найдено новостей %d, отправлено в Redis %d, ошибок %d",
		sourcesProcessed, len(sources), totalNewsFound, totalNewsSent, sourcesWithErrors)

	// Обновляем статус первого запуска
	if *firstRun {
		*firstRun = false
		rssLogger.Info("Первый цикл парсинга выполнен. Следующие циклы будут выполняться с интервалом")
	}

	return nil
}

// processCandidatesBatch обрабатывает кандидатов новостей батчем для оптимизации запросов к БД
func processCandidatesBatch(dbConn *sql.DB, redisProducer *redis.Producer, candidates []newsCandidate) int {
	if len(candidates) == 0 {
		return 0
	}

	rssLogger.Debug("Проверка батча из %d кандидатов на дубликаты", len(candidates))

	// Собираем все ссылки для проверки дубликатов (дедупликация по контенту)
	var links []string
	linkMap := make(map[string]newsCandidate) // ключ: link

	for _, candidate := range candidates {
		links = append(links, candidate.item.Link)
		linkMap[candidate.item.Link] = candidate
	}

	// Оптимизация: батч-запрос для проверки существования новостей по ссылке
	query := `
		SELECT link
		FROM news
		WHERE link = ANY($1)
	`

	rows, err := dbConn.Query(query, pq.Array(links))
	if err != nil {
		rssLogger.Error("Ошибка батч-проверки дубликатов: %v", err)
		// Fallback: обрабатываем по одной новости
		return processCandidatesSequential(dbConn, redisProducer, candidates)
	}
	defer rows.Close()

	// Собираем существующие новости
	existingNews := make(map[string]bool)
	for rows.Next() {
		var link string
		if err := rows.Scan(&link); err != nil {
			rssLogger.Warn("Ошибка чтения результата проверки дубликатов: %v", err)
			continue
		}
		existingNews[link] = true
	}

	// Обрабатываем кандидатов, пропуская дубликаты
	sent := 0
	for link, candidate := range linkMap {
		if existingNews[link] {
			rssLogger.Debug("Новость уже есть в БД, пропускаем: %s", candidate.item.Title)
			continue
		}

		monitoring.IncrementRSSItemsProcessed()

		// Создаем объект новости для отправки в Redis
		newsItem := redis.NewsItem{
			SourceID:    candidate.source.Id,
			SourceName:  candidate.source.Name,
			Title:       candidate.item.Title,
			Description: candidate.item.Description,
			Link:        candidate.item.Link,
			PublishedAt: candidate.item.PublishedAt.Format("2006-01-02 15:04:05"),
		}

		// Отправляем новость в Redis для обработки
		if err := redisProducer.PublishNews(newsItem); err != nil {
			rssLogger.Error("Ошибка отправки новости в Redis: %v", err)
			continue
		}

		sent++
		rssLogger.Info("Новость отправлена в Redis: %s (источник: %s)", candidate.item.Title, candidate.source.Name)
	}

	rssLogger.Info("Батч обработан: %d кандидатов, %d отправлено в Redis", len(candidates), sent)
	return sent
}

// processCandidatesSequential обрабатывает кандидатов последовательно (fallback функция)
func processCandidatesSequential(dbConn *sql.DB, redisProducer *redis.Producer, candidates []newsCandidate) int {
	sent := 0
	for _, candidate := range candidates {
		// Проверяем дубликат индивидуально по ссылке
		var existingNewsID int64
		err := dbConn.QueryRow(`
			SELECT id FROM news
			WHERE link = $1
		`, candidate.item.Link).Scan(&existingNewsID)

		if err == nil {
			rssLogger.Debug("Новость уже есть в БД, пропускаем: %s", candidate.item.Title)
			continue // Новость уже есть
		} else if err != sql.ErrNoRows {
			rssLogger.Warn("Ошибка проверки дубликата: %v", err)
			continue
		}

		monitoring.IncrementRSSItemsProcessed()

		newsItem := redis.NewsItem{
			SourceID:    candidate.source.Id,
			SourceName:  candidate.source.Name,
			Title:       candidate.item.Title,
			Description: candidate.item.Description,
			Link:        candidate.item.Link,
			PublishedAt: candidate.item.PublishedAt.Format("2006-01-02 15:04:05"),
		}

		if err := redisProducer.PublishNews(newsItem); err != nil {
			monitoring.IncrementRedisErrors()
			rssLogger.Error("Ошибка отправки новости в Redis: %v", err)
			continue
		}

		monitoring.IncrementRedisMessagesProduced()
		sent++
		rssLogger.Info("Новость отправлена в Redis: %s (источник: %s)", candidate.item.Title, candidate.source.Name)
	}
	return sent
}

// fetchSources получает список источников из БД
func fetchSources(dbConn *sql.DB) ([]db.Source, error) {
	rows, err := dbConn.Query("SELECT id, name, url FROM sources WHERE status = $1", db.Active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []db.Source
	for rows.Next() {
		var id int
		var name, url string
		if err := rows.Scan(&id, &name, &url); err != nil {
			return nil, err
		}
		sources = append(sources, db.Source{
			Id:   int64(id),
			Name: name,
			Url:  url,
		})
	}
	return sources, nil
}
