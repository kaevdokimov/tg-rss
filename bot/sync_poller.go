package bot

import (
	"database/sql"
	"log"
	"time"

	"tg-rss/db"
	"tg-rss/monitoring"
	"tg-rss/redis"
	"tg-rss/rss"
)

// StartRSSPollingSync запускает синхронный опрос RSS-источников (без Redis)
func StartRSSPollingSync(dbConn *sql.DB, interval time.Duration, tz *time.Location, newsProcessor *NewsProcessor) {
	// Кэшируем источники для снижения нагрузки на БД
	const cacheDuration = SourcesCacheTTL
	var sourcesCache []db.Source
	var lastCacheUpdate time.Time

	log.Printf("Запуск синхронного RSS парсера с интервалом %v", interval)

	// Создаем тикер для периодического выполнения
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Первый цикл выполняется сразу
	runSyncPollingCycle(dbConn, tz, newsProcessor, &sourcesCache, &lastCacheUpdate, cacheDuration)

	// Последующие циклы по таймеру
	for range ticker.C {
		runSyncPollingCycle(dbConn, tz, newsProcessor, &sourcesCache, &lastCacheUpdate, cacheDuration)
	}
}

// runSyncPollingCycle выполняет один цикл синхронного парсинга RSS
func runSyncPollingCycle(dbConn *sql.DB, tz *time.Location, newsProcessor *NewsProcessor,
	sourcesCache *[]db.Source, lastCacheUpdate *time.Time, cacheDuration time.Duration) {

	monitoring.IncrementRSSPolls()

	// Используем кэшированные источники или обновляем кэш
	var sources []db.Source
	if time.Since(*lastCacheUpdate) > cacheDuration || len(*sourcesCache) == 0 {
		var err error
		sources, err = db.FindActiveSources(dbConn)
		if err != nil {
			monitoring.IncrementRSSPollsErrors()
			log.Printf("Ошибка при получении источников: %v", err)
			return
		}
		*sourcesCache = sources
		*lastCacheUpdate = time.Now()
	} else {
		sources = *sourcesCache
	}

	totalNewsFound := 0
	totalNewsProcessed := 0
	sourcesProcessed := 0
	sourcesWithErrors := 0

	// Обрабатываем каждый источник синхронно
	for _, source := range sources {
		var newsList []rss.News

		// Используем circuit breaker для защиты от сбоев
		err := GetRSSCircuitBreaker().Call(func() error {
			var parseErr error
			newsList, parseErr = rss.ParseRSSWithClient(source.Url, tz, rssHttpClient)
			return parseErr
		})

		if err != nil {
			monitoring.IncrementRSSPollsErrors()
			sourcesWithErrors++
			if _, ok := err.(*CircuitBreakerError); ok {
				log.Printf("Circuit breaker открыт для источника %s: %v", source.Name, err)
			} else {
				log.Printf("Ошибка парсинга RSS для источника %s: %v", source.Name, err)
			}
			continue
		}

		sourcesProcessed++
		totalNewsFound += len(newsList)

		// Обрабатываем каждую новость
		for _, item := range newsList {
			// Пропускаем старые новости
			if time.Since(item.PublishedAt) > MaxNewsAge {
				continue
			}

			// Создаем новость для обработки
			newsItem := redis.NewsItem{
				SourceID:    source.Id,
				SourceName:  source.Name,
				Title:       item.Title,
				Description: item.Description,
				Link:        item.Link,
				PublishedAt: item.PublishedAt.Format("2006-01-02 15:04:05"),
			}

			// Обрабатываем новость напрямую (без Redis очереди)
			if err := newsProcessor.ProcessNewsItem(newsItem); err != nil {
				log.Printf("Ошибка обработки новости %s: %v", item.Title, err)
			} else {
				totalNewsProcessed++
			}
		}
	}

	log.Printf("Синхронный цикл парсинга завершен: обработано источников %d/%d, найдено новостей %d, обработано %d, ошибок %d",
		sourcesProcessed, len(sources), totalNewsFound, totalNewsProcessed, sourcesWithErrors)
}
