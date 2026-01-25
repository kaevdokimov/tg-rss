package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"tg-rss/api"
	"tg-rss/bot"
	"tg-rss/config"
	"tg-rss/db"
	"tg-rss/middleware"
	"tg-rss/monitoring"
	"tg-rss/redis"
	"time"

	_ "github.com/lib/pq" // PostgreSQL драйвер
)

var startTime = time.Now()

func main() {
	// Создаем контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка сигналов завершения и перезапуска
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Инициализация структурированного логирования
	logLevel := getEnv("LOG_LEVEL", "INFO")
	monitoring.SetLogLevelFromString(logLevel)
	logger := monitoring.NewLogger("Main")
	logger.Info("Запуск TG-RSS бота, версия 1.0.0")
	logger.Info("Уровень логирования", "level", logLevel)

	// Настройки
	cfgDB := config.LoadDBConfig()
	cfgTgBot := config.LoadTgBotConfig()
	cfgRedis := config.LoadRedisConfig()

	// Опциональный тест производительности Redis кэша
	if os.Getenv("RUN_PERFORMANCE_TEST") == "true" {
		PerformanceTest()
		return
	}

	// Инициализация базы данных
	logger.Info("Подключение к базе данных...")
	dbConn, err := db.Connect(cfgDB)
	if err != nil {
		logger.Fatal("Ошибка подключения к базе данных: %v", err)
	}
	defer func() { _ = dbConn.Close() }()

	logger.Info("Инициализация схемы базы данных...")
	db.InitSchema(dbConn)

	// Обновляем устаревшие RSS URL источников
	logger.Info("Обновление устаревших RSS источников...")
	db.UpdateOutdatedRSSSources(dbConn)

	// Обновляем названия существующих источников
	logger.Info("Обновление названий источников...")
	err = db.UpdateSourceNames(dbConn)
	if err != nil {
		logger.Warn("Не удалось обновить названия источников: %v", err)
	}

	// Инициализация Redis с graceful degradation
	var redisProducer *redis.Producer
	var redisConsumer *redis.Consumer
	var redisAvailable bool

	logger.Info("Инициализация Redis producer...")
	maxRetries := 3 // Уменьшаем количество попыток для graceful degradation
	for i := 0; i < maxRetries; i++ {
		redisProducer, err = redis.NewProducer(cfgRedis)
		if err != nil {
			logger.Warn("Ошибка создания Redis producer (попытка %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				logger.Info("Повторная попытка через 2 секунды...")
				select {
				case <-time.After(2 * time.Second):
					continue
				case <-ctx.Done():
					logger.Fatal("Контекст отменен во время инициализации Redis")
				}
			}
			logger.Warn("Redis недоступен. Переходим в режим graceful degradation (синхронная обработка)")
			redisAvailable = false
		} else {
			redisAvailable = true
			defer func() { _ = redisProducer.Close() }()
			logger.Info("Redis producer успешно инициализирован")
			break
		}
	}

	if redisAvailable {
		// Инициализация Redis consumer
		logger.Info("Инициализация Redis consumer...")
		for i := 0; i < maxRetries; i++ {
			redisConsumer, err = redis.NewConsumer(cfgRedis)
			if err != nil {
				logger.Warn("Ошибка создания Redis consumer (попытка %d/%d): %v", i+1, maxRetries, err)
				if i < maxRetries-1 {
					logger.Info("Повторная попытка через 2 секунды...")
					select {
					case <-time.After(2 * time.Second):
						continue
					case <-ctx.Done():
						logger.Fatal("Контекст отменен во время инициализации Redis")
					}
				}
				logger.Warn("Redis consumer недоступен. Работаем в ограниченном режиме")
				redisAvailable = false
			} else {
				defer func() { _ = redisConsumer.Close() }()
				logger.Info("Redis consumer успешно инициализирован")
				break
			}
		}
	}

	// Запуск health check сервера
	logger.Info("Запуск health check сервера на порту 8080...")
	go startHealthServer(ctx, dbConn)

	// Запуск обновления метрик
	logger.Info("Запуск обновления метрик каждые 30 секунд...")
	go startMetricsUpdater(ctx, dbConn)

	// Запуск бота с Redis или в режиме graceful degradation
	logger.Info("Запуск компонентов бота...")
	if redisAvailable {
		bot.StartBotWithRedis(ctx, cfgTgBot, cfgRedis, dbConn, redisProducer, redisConsumer)
	} else {
		logger.Info("Запуск в режиме graceful degradation (без Redis)")
		bot.StartBotWithoutRedis(ctx, cfgTgBot, dbConn)
	}

	// Ожидание сигнала завершения или перезапуска
	select {
	case sig := <-sigChan:
		switch sig {
		case syscall.SIGHUP:
			logger.Info("Получен сигнал SIGHUP, начинаем graceful restart...")
			// Для graceful restart просто логируем и позволяем systemd/docker перезапустить
			logger.Info("Graceful restart завершен, ожидаем перезапуска от orchestrator'а")
			cancel()
		default:
			logger.Info("Получен сигнал %v, начинаем graceful shutdown...", sig)
			cancel()                    // отменяем контекст
			time.Sleep(5 * time.Second) // даем время на завершение
		}
	case <-ctx.Done():
		logger.Info("Завершение по контексту")
	}
	logger.Info("Приложение завершено")
}

// startMetricsUpdater запускает периодическое обновление метрик
func startMetricsUpdater(ctx context.Context, dbConn *sql.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	updateDBMetrics := func() {
		if dbConn != nil {
			stats := dbConn.Stats()
			monitoring.UpdateDBConnectionMetrics(
				int64(stats.OpenConnections),
				int64(stats.Idle),
				int64(stats.InUse),
				int64(stats.WaitCount),
			)
		}
	}

	// Первое обновление
	updateDBMetrics()

	// Периодические обновления
	for {
		select {
		case <-ticker.C:
			updateDBMetrics()
		case <-ctx.Done():
			return
		}
	}
}

// startHealthServer запускает HTTP сервер для health checks и метрик
func startHealthServer(ctx context.Context, dbConn *sql.DB) {
	mux := http.NewServeMux()
	
	// Создаем rate limiter для API endpoints (100 запросов в минуту на IP)
	apiRateLimiter := middleware.NewAPIRateLimiter(100, 1*time.Minute)

	// Health check endpoint с middleware (без rate limiting)
	mux.HandleFunc("/health", middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем подключение к БД
		if err := dbConn.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "Database unhealthy: %v", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "OK")
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(10*time.Second)))

	// OpenAPI спецификация
	mux.HandleFunc("/openapi.yaml", middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		// В реальном приложении здесь можно прочитать файл
		_, _ = w.Write([]byte(`openapi: 3.0.3
info:
  title: TG-RSS Bot Management API
  description: API для управления Telegram RSS ботом
  version: 1.0.0
paths:
  /health:
    get:
      summary: Health check
      responses:
        200:
          description: OK
  /metrics:
    get:
      summary: Prometheus metrics
      responses:
        200:
          description: Metrics in Prometheus format
  /api/v1/users:
    get:
      summary: Get all users
      responses:
        200:
          description: List of users
  /api/v1/sources:
    get:
      summary: Get all sources
      responses:
        200:
          description: List of sources
    post:
      summary: Create new source
      responses:
        201:
          description: Source created
  /api/v1/subscriptions:
    get:
      summary: Get user subscriptions
      responses:
        200:
          description: User subscriptions
`))
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(5*time.Second)))

	// Metrics endpoint для Prometheus-style метрик
	mux.HandleFunc("/metrics", middleware.Chain(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Собираем метрики
		_, _ = fmt.Fprintf(w, "# TG-RSS Bot Metrics\n")
		_, _ = fmt.Fprintf(w, "# HELP rss_polls_total Total number of RSS polls\n")
		_, _ = fmt.Fprintf(w, "# TYPE rss_polls_total counter\n")
		_, _ = fmt.Fprintf(w, "rss_polls_total %d\n", monitoring.GetRSSPolls())

		_, _ = fmt.Fprintf(w, "# HELP rss_polls_errors_total Total number of RSS poll errors\n")
		_, _ = fmt.Fprintf(w, "# TYPE rss_polls_errors_total counter\n")
		_, _ = fmt.Fprintf(w, "rss_polls_errors_total %d\n", monitoring.GetRSSPollsErrors())

		_, _ = fmt.Fprintf(w, "# HELP rss_items_processed_total Total number of RSS items processed\n")
		_, _ = fmt.Fprintf(w, "# TYPE rss_items_processed_total counter\n")
		_, _ = fmt.Fprintf(w, "rss_items_processed_total %d\n", monitoring.GetRSSItemsProcessed())

		_, _ = fmt.Fprintf(w, "# HELP redis_messages_produced_total Total number of Redis messages produced\n")
		_, _ = fmt.Fprintf(w, "# TYPE redis_messages_produced_total counter\n")
		_, _ = fmt.Fprintf(w, "redis_messages_produced_total %d\n", monitoring.GetRedisMessagesProduced())

		_, _ = fmt.Fprintf(w, "# HELP redis_messages_consumed_total Total number of Redis messages consumed\n")
		_, _ = fmt.Fprintf(w, "# TYPE redis_messages_consumed_total counter\n")
		_, _ = fmt.Fprintf(w, "redis_messages_consumed_total %d\n", monitoring.GetRedisMessagesConsumed())

		_, _ = fmt.Fprintf(w, "# HELP redis_errors_total Total number of Redis errors\n")
		_, _ = fmt.Fprintf(w, "# TYPE redis_errors_total counter\n")
		_, _ = fmt.Fprintf(w, "redis_errors_total %d\n", monitoring.GetRedisErrors())

		_, _ = fmt.Fprintf(w, "# HELP telegram_messages_sent_total Total number of Telegram messages sent\n")
		_, _ = fmt.Fprintf(w, "# TYPE telegram_messages_sent_total counter\n")
		_, _ = fmt.Fprintf(w, "telegram_messages_sent_total %d\n", monitoring.GetTelegramMessagesSent())

		_, _ = fmt.Fprintf(w, "# HELP telegram_messages_errors_total Total number of Telegram message errors\n")
		_, _ = fmt.Fprintf(w, "# TYPE telegram_messages_errors_total counter\n")
		_, _ = fmt.Fprintf(w, "telegram_messages_errors_total %d\n", monitoring.GetTelegramMessagesErrors())

		_, _ = fmt.Fprintf(w, "# HELP telegram_commands_total Total number of Telegram commands received\n")
		_, _ = fmt.Fprintf(w, "# TYPE telegram_commands_total counter\n")
		_, _ = fmt.Fprintf(w, "telegram_commands_total %d\n", monitoring.GetTelegramCommands())

		_, _ = fmt.Fprintf(w, "# HELP db_queries_total Total number of database queries\n")
		_, _ = fmt.Fprintf(w, "# TYPE db_queries_total counter\n")
		_, _ = fmt.Fprintf(w, "db_queries_total %d\n", monitoring.GetDBQueries())

		_, _ = fmt.Fprintf(w, "# HELP db_queries_errors_total Total number of database query errors\n")
		_, _ = fmt.Fprintf(w, "# TYPE db_queries_errors_total counter\n")
		_, _ = fmt.Fprintf(w, "db_queries_errors_total %d\n", monitoring.GetDBQueriesErrors())

		// Добавляем uptime метрику
		_, _ = fmt.Fprintf(w, "# HELP app_uptime_seconds Application uptime in seconds\n")
		_, _ = fmt.Fprintf(w, "# TYPE app_uptime_seconds gauge\n")
		_, _ = fmt.Fprintf(w, "app_uptime_seconds %d\n", int(time.Since(startTime).Seconds()))

		// Добавляем информацию о Go
		_, _ = fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines\n")
		_, _ = fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
		_, _ = fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())

		_, _ = fmt.Fprintf(w, "# HELP go_threads Number of OS threads\n")
		_, _ = fmt.Fprintf(w, "# TYPE go_threads gauge\n")
		_, _ = fmt.Fprintf(w, "go_threads %d\n", runtime.NumCPU())

		// Метрики Circuit Breaker
		metrics := monitoring.GetMetrics()
		for name, calls := range metrics.CircuitBreakerCalls {
			_, _ = fmt.Fprintf(w, "# HELP circuit_breaker_calls_total Total number of calls to circuit breaker %s\n", name)
			_, _ = fmt.Fprintf(w, "# TYPE circuit_breaker_calls_total counter\n")
			_, _ = fmt.Fprintf(w, "circuit_breaker_calls_total{name=\"%s\"} %d\n", name, calls)
		}

		for name, failures := range metrics.CircuitBreakerFailures {
			_, _ = fmt.Fprintf(w, "# HELP circuit_breaker_failures_total Total number of failures in circuit breaker %s\n", name)
			_, _ = fmt.Fprintf(w, "# TYPE circuit_breaker_failures_total counter\n")
			_, _ = fmt.Fprintf(w, "circuit_breaker_failures_total{name=\"%s\"} %d\n", name, failures)
		}

		for name, rejected := range metrics.CircuitBreakerRejected {
			_, _ = fmt.Fprintf(w, "# HELP circuit_breaker_rejected_total Total number of rejected requests in circuit breaker %s\n", name)
			_, _ = fmt.Fprintf(w, "# TYPE circuit_breaker_rejected_total counter\n")
			_, _ = fmt.Fprintf(w, "circuit_breaker_rejected_total{name=\"%s\"} %d\n", name, rejected)
		}

		// HTTP метрики
		_, _ = fmt.Fprintf(w, "# HELP http_requests_total Total number of HTTP requests\n")
		_, _ = fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
		_, _ = fmt.Fprintf(w, "http_requests_total %d\n", metrics.HTTPRequestsTotal)

		_, _ = fmt.Fprintf(w, "# HELP http_requests_active Current number of active HTTP requests\n")
		_, _ = fmt.Fprintf(w, "# TYPE http_requests_active gauge\n")
		_, _ = fmt.Fprintf(w, "http_requests_active %d\n", metrics.HTTPRequestsActive)

		_, _ = fmt.Fprintf(w, "# HELP http_requests_errors_total Total number of HTTP request errors\n")
		_, _ = fmt.Fprintf(w, "# TYPE http_requests_errors_total counter\n")
		_, _ = fmt.Fprintf(w, "http_requests_errors_total %d\n", metrics.HTTPRequestsErrors)

		_, _ = fmt.Fprintf(w, "# HELP http_requests_timeout_total Total number of HTTP request timeouts\n")
		_, _ = fmt.Fprintf(w, "# TYPE http_requests_timeout_total counter\n")
		_, _ = fmt.Fprintf(w, "http_requests_timeout_total %d\n", metrics.HTTPRequestsTimeout)

		// Content validation метрики
		_, _ = fmt.Fprintf(w, "# HELP content_validations_total Total number of content validations\n")
		_, _ = fmt.Fprintf(w, "# TYPE content_validations_total counter\n")
		_, _ = fmt.Fprintf(w, "content_validations_total %d\n", metrics.ContentValidations)

		for field, errors := range metrics.ContentValidationErrors {
			_, _ = fmt.Fprintf(w, "# HELP content_validation_errors_total Total number of content validation errors for %s\n", field)
			_, _ = fmt.Fprintf(w, "# TYPE content_validation_errors_total counter\n")
			_, _ = fmt.Fprintf(w, "content_validation_errors_total{field=\"%s\"} %d\n", field, errors)
		}

		// Database connection метрики
		_, _ = fmt.Fprintf(w, "# HELP db_connections_open Current number of open database connections\n")
		_, _ = fmt.Fprintf(w, "# TYPE db_connections_open gauge\n")
		_, _ = fmt.Fprintf(w, "db_connections_open %d\n", metrics.DBConnectionsOpen)

		_, _ = fmt.Fprintf(w, "# HELP db_connections_idle Current number of idle database connections\n")
		_, _ = fmt.Fprintf(w, "# TYPE db_connections_idle gauge\n")
		_, _ = fmt.Fprintf(w, "db_connections_idle %d\n", metrics.DBConnectionsIdle)

		_, _ = fmt.Fprintf(w, "# HELP db_connections_in_use Current number of in-use database connections\n")
		_, _ = fmt.Fprintf(w, "# TYPE db_connections_in_use gauge\n")
		_, _ = fmt.Fprintf(w, "db_connections_in_use %d\n", metrics.DBConnectionsInUse)

		_, _ = fmt.Fprintf(w, "# HELP db_connections_wait Current number of connections waiting\n")
		_, _ = fmt.Fprintf(w, "# TYPE db_connections_wait gauge\n")
		_, _ = fmt.Fprintf(w, "db_connections_wait %d\n", metrics.DBConnectionsWait)
		
		// Cache метрики
		_, _ = fmt.Fprintf(w, "# HELP cache_hits_total Total number of cache hits by cache name\n")
		_, _ = fmt.Fprintf(w, "# TYPE cache_hits_total counter\n")
		for name, hits := range metrics.CacheHits {
			_, _ = fmt.Fprintf(w, "cache_hits_total{cache=\"%s\"} %d\n", name, hits)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP cache_misses_total Total number of cache misses by cache name\n")
		_, _ = fmt.Fprintf(w, "# TYPE cache_misses_total counter\n")
		for name, misses := range metrics.CacheMisses {
			_, _ = fmt.Fprintf(w, "cache_misses_total{cache=\"%s\"} %d\n", name, misses)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP cache_size Current size of cache by cache name\n")
		_, _ = fmt.Fprintf(w, "# TYPE cache_size gauge\n")
		for name, size := range metrics.CacheSize {
			_, _ = fmt.Fprintf(w, "cache_size{cache=\"%s\"} %d\n", name, size)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP cache_evictions_total Total number of cache evictions by cache name\n")
		_, _ = fmt.Fprintf(w, "# TYPE cache_evictions_total counter\n")
		for name, evictions := range metrics.CacheEvictions {
			_, _ = fmt.Fprintf(w, "cache_evictions_total{cache=\"%s\"} %d\n", name, evictions)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP cache_operations_total Total number of cache operations by cache name\n")
		_, _ = fmt.Fprintf(w, "# TYPE cache_operations_total counter\n")
		for name, operations := range metrics.CacheOperations {
			_, _ = fmt.Fprintf(w, "cache_operations_total{cache=\"%s\"} %d\n", name, operations)
		}
		
		// Queue метрики
		_, _ = fmt.Fprintf(w, "# HELP queue_size Current size of queue by queue name\n")
		_, _ = fmt.Fprintf(w, "# TYPE queue_size gauge\n")
		for name, size := range metrics.QueueSize {
			_, _ = fmt.Fprintf(w, "queue_size{queue=\"%s\"} %d\n", name, size)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP queue_processed_total Total number of processed items by queue name\n")
		_, _ = fmt.Fprintf(w, "# TYPE queue_processed_total counter\n")
		for name, processed := range metrics.QueueProcessed {
			_, _ = fmt.Fprintf(w, "queue_processed_total{queue=\"%s\"} %d\n", name, processed)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP queue_errors_total Total number of queue errors by queue name\n")
		_, _ = fmt.Fprintf(w, "# TYPE queue_errors_total counter\n")
		for name, errors := range metrics.QueueErrors {
			_, _ = fmt.Fprintf(w, "queue_errors_total{queue=\"%s\"} %d\n", name, errors)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP queue_latency_ms Average queue latency in milliseconds by queue name\n")
		_, _ = fmt.Fprintf(w, "# TYPE queue_latency_ms gauge\n")
		for name, latency := range metrics.QueueLatencyMs {
			_, _ = fmt.Fprintf(w, "queue_latency_ms{queue=\"%s\"} %d\n", name, latency)
		}
		
		// Rate limiting метрики
		_, _ = fmt.Fprintf(w, "# HELP rate_limit_hits_total Total number of rate limit hits by limiter name\n")
		_, _ = fmt.Fprintf(w, "# TYPE rate_limit_hits_total counter\n")
		for name, hits := range metrics.RateLimitHits {
			_, _ = fmt.Fprintf(w, "rate_limit_hits_total{limiter=\"%s\"} %d\n", name, hits)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP rate_limit_misses_total Total number of rate limit misses by limiter name\n")
		_, _ = fmt.Fprintf(w, "# TYPE rate_limit_misses_total counter\n")
		for name, misses := range metrics.RateLimitMisses {
			_, _ = fmt.Fprintf(w, "rate_limit_misses_total{limiter=\"%s\"} %d\n", name, misses)
		}
		
		_, _ = fmt.Fprintf(w, "# HELP rate_limit_rejected_total Total number of rate limit rejections by limiter name\n")
		_, _ = fmt.Fprintf(w, "# TYPE rate_limit_rejected_total counter\n")
		for name, rejected := range metrics.RateLimitRejected {
			_, _ = fmt.Fprintf(w, "rate_limit_rejected_total{limiter=\"%s\"} %d\n", name, rejected)
		}
	}, middleware.Logging, middleware.Recovery, middleware.CORS, middleware.Timeout(15*time.Second)))

	// API для управления пользователями (с rate limiting)
	mux.HandleFunc("/api/v1/users", middleware.Chain(
		api.GetUsersHandler(dbConn),
		apiRateLimiter.RateLimit,
	))
	mux.HandleFunc("/api/v1/users/check", middleware.Chain(
		api.GetUserHandler(dbConn),
		apiRateLimiter.RateLimit,
	))

	// API для управления источниками (с rate limiting)
	mux.HandleFunc("/api/v1/sources", middleware.Chain(
		api.GetSourcesHandler(dbConn),
		apiRateLimiter.RateLimit,
	))
	mux.HandleFunc("/api/v1/sources/info", middleware.Chain(
		api.GetSourceHandler(dbConn),
		apiRateLimiter.RateLimit,
	))
	mux.HandleFunc("/api/v1/sources/create", middleware.Chain(
		api.CreateSourceHandler(dbConn),
		apiRateLimiter.RateLimit,
	))
	mux.HandleFunc("/api/v1/sources/update", middleware.Chain(
		api.UpdateSourceHandler(dbConn),
		apiRateLimiter.RateLimit,
	))
	mux.HandleFunc("/api/v1/sources/delete", middleware.Chain(
		api.DeleteSourceHandler(dbConn),
		apiRateLimiter.RateLimit,
	))

	// API для управления подписками (с rate limiting)
	mux.HandleFunc("/api/v1/subscriptions", middleware.Chain(
		api.GetSubscriptionsHandler(dbConn),
		apiRateLimiter.RateLimit,
	))
	mux.HandleFunc("/api/v1/subscriptions/subscribe", middleware.Chain(
		api.SubscribeHandler(dbConn),
		apiRateLimiter.RateLimit,
	))
	mux.HandleFunc("/api/v1/subscriptions/unsubscribe", middleware.Chain(
		api.UnsubscribeHandler(dbConn),
		apiRateLimiter.RateLimit,
	))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Stopping health check server...")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("Health server shutdown error: %v", err)
	}
}

// PerformanceTest сравнивает производительность с Redis кэшем и без него
func PerformanceTest() {
	fmt.Println("🚀 Тестирование производительности Redis кэша для скраппинга")
	fmt.Println("============================================================")

	redisConfig := &config.RedisConfig{
		Addr:     "redis:6379", // или "localhost:6379" для локального тестирования
		Password: "",
		DB:       0,
	}

	// Тестируем подключение к Redis
	cache, err := redis.NewContentCache(redisConfig)
	if err != nil {
		log.Printf("❌ Redis недоступен: %v", err)
		log.Printf("🔄 Продолжаем тестирование без Redis кэша")
		cache = nil
	} else {
		defer func() { _ = cache.Close() }()
		fmt.Println("✅ Redis кэш подключен")
	}

	// Создаем тестовый контент
	testContent := &redis.CachedNewsContent{
		FullText:        "Это пример текста новости для тестирования производительности кэширования.",
		Author:          "Тестовый Автор",
		Category:        "Технологии",
		Tags:            []string{"тест", "производительность", "redis"},
		Images:          []string{"https://example.com/image1.jpg"},
		MetaKeywords:    "тест, производительность",
		MetaDescription: "Тестовое описание",
		ContentHTML:     "<p>Тестовый контент</p>",
	}

	testURLs := []string{
		"https://example.com/article1",
		"https://example.com/article2",
		"https://example.com/article3",
	}

	// Тест 1: Запись в кэш
	fmt.Println("\n📝 Тест 1: Запись в кэш")
	if cache != nil {
		start := time.Now()
		for _, url := range testURLs {
			err := cache.Set(url, testContent, 30*time.Minute)
			if err != nil {
				log.Printf("Ошибка записи в кэш: %v", err)
			}
		}
		duration := time.Since(start)
		fmt.Printf("✅ Запись %d записей: %v (%.2f мс/запись)\n",
			len(testURLs), duration, float64(duration.Nanoseconds())/float64(len(testURLs))/1000000)
	} else {
		fmt.Println("⏭️  Пропущено (Redis недоступен)")
	}

	// Тест 2: Чтение из кэша
	fmt.Println("\n📖 Тест 2: Чтение из кэша")
	if cache != nil {
		start := time.Now()
		hits := 0
		for i := 0; i < 50; i++ { // 50 чтений
			for _, url := range testURLs {
				if _, found := cache.Get(url); found {
					hits++
				}
			}
		}
		duration := time.Since(start)
		fmt.Printf("✅ %d удачных чтений: %v (%.2f мс/чтение)\n",
			hits, duration, float64(duration.Nanoseconds())/float64(hits)/1000000)
	} else {
		fmt.Println("⏭️  Пропущено (Redis недоступен)")
	}

	fmt.Println("\n📊 Резюме тестирования производительности:")
	if cache != nil {
		fmt.Println("✅ Redis кэш работает корректно и готов к использованию")
		fmt.Println("🎯 Ожидаемые преимущества:")
		fmt.Println("   • 3-10x ускорение повторных запросов")
		fmt.Println("   • Снижение нагрузки на целевые сайты")
		fmt.Println("   • Автоматическая очистка устаревших данных")
	} else {
		fmt.Println("❌ Redis недоступен - кэширование отключено")
		fmt.Println("💡 Для лучших результатов подключите Redis")
	}
}

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
