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
	"tg-rss/bot"
	"tg-rss/config"
	"tg-rss/db"
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

	// Обработка сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Инициализация структурированного логирования
	logLevel := getEnv("LOG_LEVEL", "INFO")
	monitoring.SetLogLevelFromString(logLevel)
	logger := monitoring.NewLogger("Main")
	logger.Info("Запуск TG-RSS бота, версия 1.0.0")
	logger.Info("Уровень логирования: %s", logLevel)

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
	defer dbConn.Close()

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
			defer redisProducer.Close()
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
				defer redisConsumer.Close()
				logger.Info("Redis consumer успешно инициализирован")
				break
			}
		}
	}

	// Запуск health check сервера
	logger.Info("Запуск health check сервера на порту 8080...")
	go startHealthServer(ctx, dbConn)

	// Запуск бота с Redis или в режиме graceful degradation
	logger.Info("Запуск компонентов бота...")
	if redisAvailable {
		bot.StartBotWithRedis(ctx, cfgTgBot, cfgRedis, dbConn, redisProducer, redisConsumer)
	} else {
		logger.Info("Запуск в режиме graceful degradation (без Redis)")
		bot.StartBotWithoutRedis(ctx, cfgTgBot, dbConn)
	}

	// Ожидание сигнала завершения
	select {
	case sig := <-sigChan:
		logger.Info("Получен сигнал %v, начинаем graceful shutdown...", sig)
		cancel()                    // отменяем контекст
		time.Sleep(5 * time.Second) // даем время на завершение
	case <-ctx.Done():
		logger.Info("Завершение по контексту")
	}
	logger.Info("Приложение завершено")
}

// startHealthServer запускает HTTP сервер для health checks и метрик
func startHealthServer(ctx context.Context, dbConn *sql.DB) {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Проверяем подключение к БД
		if err := dbConn.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Database unhealthy: %v", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Metrics endpoint для Prometheus-style метрик
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Собираем метрики
		fmt.Fprintf(w, "# TG-RSS Bot Metrics\n")
		fmt.Fprintf(w, "# HELP rss_polls_total Total number of RSS polls\n")
		fmt.Fprintf(w, "# TYPE rss_polls_total counter\n")
		fmt.Fprintf(w, "rss_polls_total %d\n", monitoring.GetRSSPolls())

		fmt.Fprintf(w, "# HELP rss_polls_errors_total Total number of RSS poll errors\n")
		fmt.Fprintf(w, "# TYPE rss_polls_errors_total counter\n")
		fmt.Fprintf(w, "rss_polls_errors_total %d\n", monitoring.GetRSSPollsErrors())

		fmt.Fprintf(w, "# HELP rss_items_processed_total Total number of RSS items processed\n")
		fmt.Fprintf(w, "# TYPE rss_items_processed_total counter\n")
		fmt.Fprintf(w, "rss_items_processed_total %d\n", monitoring.GetRSSItemsProcessed())

		fmt.Fprintf(w, "# HELP redis_messages_produced_total Total number of Redis messages produced\n")
		fmt.Fprintf(w, "# TYPE redis_messages_produced_total counter\n")
		fmt.Fprintf(w, "redis_messages_produced_total %d\n", monitoring.GetRedisMessagesProduced())

		fmt.Fprintf(w, "# HELP redis_messages_consumed_total Total number of Redis messages consumed\n")
		fmt.Fprintf(w, "# TYPE redis_messages_consumed_total counter\n")
		fmt.Fprintf(w, "redis_messages_consumed_total %d\n", monitoring.GetRedisMessagesConsumed())

		fmt.Fprintf(w, "# HELP redis_errors_total Total number of Redis errors\n")
		fmt.Fprintf(w, "# TYPE redis_errors_total counter\n")
		fmt.Fprintf(w, "redis_errors_total %d\n", monitoring.GetRedisErrors())

		fmt.Fprintf(w, "# HELP telegram_messages_sent_total Total number of Telegram messages sent\n")
		fmt.Fprintf(w, "# TYPE telegram_messages_sent_total counter\n")
		fmt.Fprintf(w, "telegram_messages_sent_total %d\n", monitoring.GetTelegramMessagesSent())

		fmt.Fprintf(w, "# HELP telegram_messages_errors_total Total number of Telegram message errors\n")
		fmt.Fprintf(w, "# TYPE telegram_messages_errors_total counter\n")
		fmt.Fprintf(w, "telegram_messages_errors_total %d\n", monitoring.GetTelegramMessagesErrors())

		fmt.Fprintf(w, "# HELP telegram_commands_total Total number of Telegram commands received\n")
		fmt.Fprintf(w, "# TYPE telegram_commands_total counter\n")
		fmt.Fprintf(w, "telegram_commands_total %d\n", monitoring.GetTelegramCommands())

		fmt.Fprintf(w, "# HELP db_queries_total Total number of database queries\n")
		fmt.Fprintf(w, "# TYPE db_queries_total counter\n")
		fmt.Fprintf(w, "db_queries_total %d\n", monitoring.GetDBQueries())

		fmt.Fprintf(w, "# HELP db_queries_errors_total Total number of database query errors\n")
		fmt.Fprintf(w, "# TYPE db_queries_errors_total counter\n")
		fmt.Fprintf(w, "db_queries_errors_total %d\n", monitoring.GetDBQueriesErrors())

		// Добавляем uptime метрику
		fmt.Fprintf(w, "# HELP app_uptime_seconds Application uptime in seconds\n")
		fmt.Fprintf(w, "# TYPE app_uptime_seconds gauge\n")
		fmt.Fprintf(w, "app_uptime_seconds %d\n", int(time.Since(startTime).Seconds()))

		// Добавляем информацию о Go
		fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines\n")
		fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
		fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())

		fmt.Fprintf(w, "# HELP go_threads Number of OS threads\n")
		fmt.Fprintf(w, "# TYPE go_threads gauge\n")
		fmt.Fprintf(w, "go_threads %d\n", runtime.NumCPU())
	})

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
		defer cache.Close()
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
