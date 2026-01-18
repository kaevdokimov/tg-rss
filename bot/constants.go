package bot

import "time"

// Константы для работы с Telegram API
const (
	// Rate limiting
	DefaultRateLimitInterval = 50 * time.Millisecond // 20 сообщений/секунду
	MaxRateLimitInterval     = 60 * time.Second      // максимальный интервал при rate limiting
	MinRateLimitInterval     = 50 * time.Millisecond // минимальный интервал

	// Лимиты Telegram API
	MaxMessageLength = 4096 // максимальная длина сообщения

	// Таймауты
	RedisInitTimeout     = 5 * time.Second  // таймаут инициализации Redis
	HTTPTimeout          = 10 * time.Second // таймаут HTTP запросов для RSS
	ContentScraperDelay  = 1 * time.Minute  // задержка первого запуска content scraper

	// Интервалы
	DefaultSendInterval      = 15 * time.Minute // интервал отправки новостей
	SubscriptionsCacheTTL    = 10 * time.Minute // TTL кэша подписок
	SourcesCacheTTL          = 30 * time.Minute // TTL кэша источников
	RedisCacheTTL            = 30 * time.Minute // TTL Redis кэша контента

	// Лимиты времени
	MaxNewsAge = 24 * time.Hour // максимальный возраст новости

	// Retry логика
	MaxRetries             = 5                 // максимальное количество попыток
	BaseRetryDelay         = 1 * time.Second   // базовая задержка для exponential backoff
	MaxRetryDelay          = 32 * time.Second  // максимальная задержка
	ContentScraperMaxDelay = 1 * time.Second   // максимальная задержка в content scraper

	// Параллельная обработка
	MaxWorkers        = 6 // максимальное количество воркеров
	DefaultBatchSize  = 50 // размер батча для обработки
	DefaultConcurrent = 3  // количество параллельных запросов

	// Ограничения
	MaxContentSize = 2 * 1024 * 1024 // 2MB максимальный размер контента
)