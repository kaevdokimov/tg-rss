package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"tg-rss/bot"
	"tg-rss/config"
	"tg-rss/db"
	"tg-rss/redis"
	"tg-rss/monitoring"
	"time"
)

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

	// Инициализация базы данных
	logger.Info("Подключение к базе данных...")
	dbConn, err := db.Connect(cfgDB)
	if err != nil {
		logger.Fatal("Ошибка подключения к базе данных: %v", err)
	}
	defer dbConn.Close()

	logger.Info("Инициализация схемы базы данных...")
	db.InitSchema(dbConn)

	// Обновляем названия существующих источников
	logger.Info("Обновление названий источников...")
	err = db.UpdateSourceNames(dbConn)
	if err != nil {
		logger.Warn("Не удалось обновить названия источников: %v", err)
	}

	// Инициализация Redis producer с retry
	logger.Info("Инициализация Redis producer...")
	var redisProducer *redis.Producer
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		redisProducer, err = redis.NewProducer(cfgRedis)
		if err != nil {
			logger.Warn("Ошибка создания Redis producer (попытка %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				logger.Info("Повторная попытка через 5 секунд...")
				time.Sleep(5 * time.Second)
				continue
			}
			logger.Fatal("Не удалось создать Redis producer после %d попыток", maxRetries)
		}
		break
	}
	defer redisProducer.Close()
	logger.Info("Redis producer успешно инициализирован")

	// Инициализация Redis consumer с retry
	logger.Info("Инициализация Redis consumer...")
	var redisConsumer *redis.Consumer
	for i := 0; i < maxRetries; i++ {
		redisConsumer, err = redis.NewConsumer(cfgRedis)
		if err != nil {
			logger.Warn("Ошибка создания Redis consumer (попытка %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				logger.Info("Повторная попытка через 5 секунд...")
				time.Sleep(5 * time.Second)
				continue
			}
			logger.Fatal("Не удалось создать Redis consumer после %d попыток", maxRetries)
		}
		break
	}
	defer redisConsumer.Close()
	logger.Info("Redis consumer успешно инициализирован")

	// Запуск health check сервера
	logger.Info("Запуск health check сервера на порту 8080...")
	go startHealthServer(ctx, dbConn)

	// Запуск бота с Redis
	logger.Info("Запуск компонентов бота...")
	bot.StartBotWithRedis(ctx, cfgTgBot, dbConn, redisProducer, redisConsumer)

	// Ожидание сигнала завершения
	select {
	case sig := <-sigChan:
		logger.Info("Получен сигнал %v, начинаем graceful shutdown...", sig)
		cancel() // отменяем контекст
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
		fmt.Fprintf(w, "rss_polls_total %d\n", monitoring.GetRSSPolls())
		fmt.Fprintf(w, "rss_polls_errors_total %d\n", monitoring.GetRSSPollsErrors())
		fmt.Fprintf(w, "rss_items_processed_total %d\n", monitoring.GetRSSItemsProcessed())
		fmt.Fprintf(w, "redis_messages_produced_total %d\n", monitoring.GetRedisMessagesProduced())
		fmt.Fprintf(w, "redis_errors_total %d\n", monitoring.GetRedisErrors())
		fmt.Fprintf(w, "telegram_messages_sent_total %d\n", monitoring.GetTelegramMessagesSent())
		fmt.Fprintf(w, "telegram_messages_errors_total %d\n", monitoring.GetTelegramMessagesErrors())
		fmt.Fprintf(w, "db_queries_total %d\n", monitoring.GetDBQueries())
		fmt.Fprintf(w, "db_queries_errors_total %d\n", monitoring.GetDBQueriesErrors())
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

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
