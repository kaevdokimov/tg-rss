package bot

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"tg-rss/db"
	"tg-rss/monitoring"
	"tg-rss/redis"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var newsLogger = monitoring.NewLogger("NewsProcessor")

// Константы для управления памятью
const (
	MaxPendingNewsPerUser = 100            // Максимум новостей в очереди на пользователя
	PendingNewsTTL        = 24 * time.Hour // TTL для новостей в очереди
	CleanupInterval       = 1 * time.Hour  // Интервал очистки старых записей
)

// PendingNews представляет новость, ожидающую отправки
type PendingNews struct {
	NewsID      int64
	SourceID    int64
	SourceName  string
	SourceUrl   string
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
	AddedAt     time.Time // Когда добавлена в очередь (для TTL)
}

// NewsProcessor обрабатывает новости из Redis и записывает в БД
type NewsProcessor struct {
	db                *sql.DB
	bot               *tgbotapi.BotAPI
	globalRateLimiter *GlobalRateLimiter
	pendingNews       map[int64][]PendingNews // очередь новостей по пользователям
	pendingMutex      sync.Mutex
	sendInterval      time.Duration
	// Кэш подписок для снижения запросов к БД
	subscriptionsCache map[int64][]interface{} // кэш подписок по source_id
	cacheMutex         sync.RWMutex
	lastCacheUpdate    time.Time
	cacheDuration      time.Duration
}

// NewNewsProcessor создает новый обработчик новостей
func NewNewsProcessor(db *sql.DB, bot *tgbotapi.BotAPI) *NewsProcessor {
	// Глобальный rate limiter: минимум 50ms между сообщениями (20 сообщений/секунду)
	// Это дает запас от лимита Telegram в 30 сообщений/секунду
	globalLimiter := NewGlobalRateLimiter(DefaultRateLimitInterval)

	// Инициализация кэша подписок
	cache := make(map[int64][]interface{})

	np := &NewsProcessor{
		db:                 db,
		bot:                bot,
		globalRateLimiter:  globalLimiter,
		pendingNews:        make(map[int64][]PendingNews),
		sendInterval:       DefaultSendInterval, // отправка раз в 15 минут
		subscriptionsCache: cache,
		cacheDuration:      SubscriptionsCacheTTL, // обновление кэша каждые 10 минут
	}

	// Запускаем периодическую отправку накопленных новостей
	go np.startPeriodicSending()

	// Запускаем периодическую очистку старых записей
	go np.startPeriodicCleanup()

	return np
}

// ProcessNewsItem обрабатывает новость из Redis
func (np *NewsProcessor) ProcessNewsItem(newsItem redis.NewsItem) error {
	// Парсим время публикации
	publishedAt, err := time.Parse("2006-01-02 15:04:05", newsItem.PublishedAt)
	if err != nil {
		newsLogger.Warn("Ошибка парсинга времени", "error", err)
		publishedAt = time.Now()
	}

	// Сохранение новости в БД
	query := `INSERT INTO news (title, description, link, published_at, source_id) 
			  VALUES ($1, $2, $3, $4, $5) 
			  ON CONFLICT (link) DO UPDATE SET title = $1, description = $2, published_at = $4
			  RETURNING id`

	monitoring.IncrementDBQueries()
	var newsID int64
	err = np.db.QueryRow(query, newsItem.Title, newsItem.Description, newsItem.Link, publishedAt, newsItem.SourceID).Scan(&newsID)
	if err != nil {
		monitoring.IncrementDBQueriesErrors()
		return err
	}

	newsLogger.Debug("Новость сохранена в БД",
		"news_id", newsID,
		"title", newsItem.Title)

	// Проверяем, является ли новость новой (не старше MaxNewsAge)
	// Это предотвращает отправку старых новостей при первом запуске или перезапуске
	if time.Since(publishedAt) > MaxNewsAge {
		newsLogger.Debug("Пропускаем старую новость (старше 24ч)",
			"title", newsItem.Title,
			"published_at", publishedAt)
		return nil
	}

	// Получение списка пользователей, подписанных на источник (с кэшированием)
	subscriptions, err := np.getSubscriptionsCached(newsItem.SourceID)
	if err != nil {
		newsLogger.Error("Ошибка при получении подписок", "error", err)
		return err
	}

	// Получаем URL источника для форматирования
	var sourceUrl string
	err = np.db.QueryRow("SELECT url FROM sources WHERE id = $1", newsItem.SourceID).Scan(&sourceUrl)
	if err != nil {
		newsLogger.Warn("Не удалось получить URL источника",
			"source_id", newsItem.SourceID,
			"error", err)
		sourceUrl = ""
	}

	// Проверяем, не отправляли ли уже эту новость кому-то (глобальная дедупликация)
	// Используем news_id вместо source_id + link для дедупликации по контенту
	newsAlreadySent, err := np.isNewsAlreadySentGlobally(newsID)
	if err != nil {
		newsLogger.Error("Ошибка при проверке глобальной отправки новости", "error", err)
		return err
	}
	if newsAlreadySent {
		newsLogger.Debug("Новость уже была отправлена глобально", "title", newsItem.Title)
		return nil
	}

	// Добавляем новость в очередь для каждого подписанного пользователя
	np.pendingMutex.Lock()
	defer np.pendingMutex.Unlock()

	addedToQueue := 0

	// Собираем все chatID для батч-проверки
	chatIDs := make([]int64, len(subscriptions))
	for i, subscription := range subscriptions {
		chatIDs[i] = subscription.ChatId
	}

	// Батч-проверка: проверяем для всех пользователей за один запрос
	sentToUsers, err := db.IsNewsSentToUsers(np.db, chatIDs, newsID)
	if err != nil {
		newsLogger.Error("Ошибка при батч-проверке отправленных новостей", "error", err)
		// Fallback: продолжаем, но не добавляем новости
		return err
	}

	// Добавляем новости в очередь только для тех, кому еще не отправляли
	for _, subscription := range subscriptions {
		// Проверяем результат батч-запроса
		if sent, exists := sentToUsers[subscription.ChatId]; exists && sent {
			newsLogger.Debug("Новость уже была отправлена пользователю",
				"user_id", subscription.ChatId,
				"title", newsItem.Title)
			continue
		}

		// Добавляем новость в очередь пользователя
		pending := PendingNews{
			NewsID:      newsID,
			SourceID:    newsItem.SourceID,
			SourceName:  newsItem.SourceName,
			SourceUrl:   sourceUrl,
			Title:       newsItem.Title,
			Description: newsItem.Description,
			Link:        newsItem.Link,
			PublishedAt: publishedAt,
			AddedAt:     time.Now(), // Устанавливаем время добавления
		}

		// Проверяем лимит на количество новостей для пользователя
		userQueue := np.pendingNews[subscription.ChatId]
		if len(userQueue) >= MaxPendingNewsPerUser {
			newsLogger.Warn("Очередь новостей пользователя переполнена, пропускаем",
				"user_id", subscription.ChatId,
				"queue_size", len(userQueue),
				"max_size", MaxPendingNewsPerUser)
			continue
		}

		np.pendingNews[subscription.ChatId] = append(userQueue, pending)
		addedToQueue++
		newsLogger.Debug("Новость добавлена в очередь для пользователя",
			"user_id", subscription.ChatId,
			"title", newsItem.Title,
			"queue_size", len(np.pendingNews[subscription.ChatId]))
	}

	if addedToQueue > 0 {
		newsLogger.Info("Новость добавлена в очередь для пользователей",
			"title", newsItem.Title,
			"users_count", addedToQueue)
	} else if len(subscriptions) > 0 {
		newsLogger.Warn("Новость не была добавлена в очередь (возможно, уже отправлена всем подписчикам)",
			"title", newsItem.Title)
	} else {
		newsLogger.Debug("Новость не была добавлена в очередь (нет подписчиков на источник)",
			"title", newsItem.Title,
			"source_id", newsItem.SourceID)
	}

	return nil
}

// isNewsAlreadySentGlobally проверяет, была ли новость уже отправлена кому-либо
func (np *NewsProcessor) isNewsAlreadySentGlobally(newsID int64) (bool, error) {
	var count int
	err := np.db.QueryRow(`
		SELECT COUNT(*)
		FROM messages
		WHERE news_id = $1
	`, newsID).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("ошибка при проверке глобальной отправки новости: %w", err)
	}

	return count > 0, nil
}

// startPeriodicSending запускает периодическую отправку накопленных новостей
func (np *NewsProcessor) startPeriodicSending() {
	newsLogger.Info("Запуск периодической отправки новостей с интервалом",
		"interval", np.sendInterval)

	// Создаем тикер для периодической отправки
	ticker := time.NewTicker(np.sendInterval)
	defer ticker.Stop()

	// Первая отправка через интервал (15 минут)
	// Это позволяет накопить новости за период
	go func() {
		time.Sleep(np.sendInterval)
		np.sendPendingNews()
	}()

	// Затем отправляем по расписанию каждые 15 минут
	for range ticker.C {
		np.sendPendingNews()

		// Обновляем метрики размера очередей
		np.pendingMutex.Lock()
		totalQueueSize := int64(0)
		for _, queue := range np.pendingNews {
			totalQueueSize += int64(len(queue))
		}
		np.pendingMutex.Unlock()

		monitoring.UpdateQueueSize("pending_news", totalQueueSize)
	}
}

// sendPendingNews отправляет все накопленные новости пользователям списком
func (np *NewsProcessor) sendPendingNews() {
	np.pendingMutex.Lock()

	// Копируем очередь и очищаем оригинал
	pendingCopy := make(map[int64][]PendingNews)
	for chatId, news := range np.pendingNews {
		if len(news) > 0 {
			pendingCopy[chatId] = make([]PendingNews, len(news))
			copy(pendingCopy[chatId], news)
		}
	}
	np.pendingNews = make(map[int64][]PendingNews)

	np.pendingMutex.Unlock()

	if len(pendingCopy) == 0 {
		newsLogger.Debug("Нет накопленных новостей для отправки")
		return
	}

	// Подсчитываем общее количество новостей
	totalNews := 0
	for _, newsList := range pendingCopy {
		totalNews += len(newsList)
	}

	newsLogger.Info("Начинаем отправку накопленных новостей",
		"users_count", len(pendingCopy),
		"total_news", totalNews)

	messagesSent := 0
	errorCount := 0

	// Отправляем новости каждому пользователю
	for chatId, newsList := range pendingCopy {
		if len(newsList) == 0 {
			continue
		}

		// Проверяем глобальный rate limit перед отправкой
		if !np.globalRateLimiter.AllowGlobal() {
			newsLogger.Debug("Глобальный rate limit, пропускаем отправку списка новостей пользователю",
				"user_id", chatId)
			// Возвращаем новости обратно в очередь
			np.pendingMutex.Lock()
			np.pendingNews[chatId] = append(np.pendingNews[chatId], newsList...)
			np.pendingMutex.Unlock()
			continue
		}

		// Отправляем новости частями, если они не помещаются в одно сообщение
		// Telegram имеет лимит MaxMessageLength символов на сообщение
		message := ""
		newsIndex := 0
		sentNewsCount := 0
		allSentSuccessfully := true

		for newsIndex < len(newsList) {
			news := newsList[newsIndex]
			// Нумерация продолжается между сообщениями
			formattedNews := formatMessage(newsIndex+1, news.Title, news.PublishedAt, news.SourceName, news.Link)

			// Проверяем, не превысит ли добавление этой новости лимит
			if len(message)+len(formattedNews) > MaxMessageLength {
				// Если это первая новость и она сама превышает лимит, отправляем её отдельно
				if len(message) == 0 {
					// Обрезаем форматированную новость, если она слишком длинная
					if len(formattedNews) > MaxMessageLength {
						truncatedNews := formattedNews[:MaxMessageLength]
						lastNewline := strings.LastIndex(truncatedNews, "\n")
						if lastNewline > 0 {
							formattedNews = truncatedNews[:lastNewline]
						} else {
							formattedNews = truncatedNews
						}
					}
					message = formattedNews
					newsIndex++
				} else {
					// Текущее сообщение готово, отправляем его
					message = strings.TrimRight(message, "\n")
					if !np.sendNewsMessage(chatId, message, newsList[sentNewsCount:newsIndex]) {
						allSentSuccessfully = false
						break
					}
					sentNewsCount = newsIndex
					message = ""
					// Не увеличиваем newsIndex, чтобы добавить текущую новость в следующее сообщение
				}
			} else {
				message += formattedNews
				newsIndex++
			}
		}

		// Отправляем оставшиеся новости, если есть
		if allSentSuccessfully && len(message) > 0 {
			message = strings.TrimRight(message, "\n")
			if !np.sendNewsMessage(chatId, message, newsList[sentNewsCount:]) {
				allSentSuccessfully = false
			} else {
				sentNewsCount = len(newsList)
			}
		}

		// Если не все сообщения отправились успешно, возвращаем неотправленные новости в очередь
		if !allSentSuccessfully && sentNewsCount < len(newsList) {
			np.pendingMutex.Lock()
			np.pendingNews[chatId] = append(np.pendingNews[chatId], newsList[sentNewsCount:]...)
			np.pendingMutex.Unlock()
			newsLogger.Warn("Не все новости были отправлены пользователю",
				"user_id", chatId,
				"returned_to_queue", len(newsList)-sentNewsCount)
			errorCount++
		} else if sentNewsCount > 0 {
			newsLogger.Info("Список новостей отправлен пользователю",
				"news_count", sentNewsCount,
				"user_id", chatId)
			messagesSent++
		}
	}

	// Обновляем метрики
	monitoring.IncrementQueueProcessed("pending_news")
	if errorCount > 0 {
		monitoring.IncrementQueueErrors("pending_news")
	}
}

// sendNewsMessage отправляет одно сообщение с новостями и сохраняет их в БД
func (np *NewsProcessor) sendNewsMessage(chatId int64, message string, newsList []PendingNews) bool {
	if len(message) == 0 {
		newsLogger.Warn("Пустое сообщение для пользователя, пропускаем",
			"user_id", chatId)
		return false
	}

	// Небольшая задержка между отправками
	currentInterval := np.globalRateLimiter.GetMinInterval()
	if currentInterval > 0 {
		time.Sleep(currentInterval)
	}

	// Отправляем сообщение
	msg := tgbotapi.NewMessage(chatId, message)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true

	if _, err := np.bot.Send(msg); err != nil {
		monitoring.IncrementTelegramMessagesErrors()

		// Улучшенная обработка ошибок
		if isRateLimitError(err) {
			retryAfter := extractRetryAfter(err)
			if retryAfter > 0 {
				var newInterval time.Duration
				if retryAfter > 3600 {
					newInterval = MaxRateLimitInterval
					newsLogger.Warn("Критический rate limit для пользователя",
						"user_id", chatId,
						"retry_after", retryAfter)
				} else if retryAfter > 300 {
					newInterval = 30 * time.Second
					newsLogger.Warn("Высокий rate limit для пользователя",
						"user_id", chatId,
						"retry_after", retryAfter)
				} else {
					newInterval = time.Duration(retryAfter+5) * time.Second
					if newInterval > MaxRateLimitInterval {
						newInterval = MaxRateLimitInterval
					}
					newsLogger.Warn("Rate limit для пользователя, увеличиваем глобальный интервал",
						"user_id", chatId,
						"retry_after", retryAfter,
						"new_interval", newInterval)
				}
				np.globalRateLimiter.SetMinInterval(newInterval)
			} else {
				np.globalRateLimiter.SetMinInterval(5 * time.Second)
				newsLogger.Warn("Rate limit для пользователя (время не указано), увеличиваем глобальный интервал до 5 секунд",
					"user_id", chatId)
			}
			return false
		}

		errorMsg := handleTelegramError(err)
		newsLogger.Error("Ошибка отправки списка новостей пользователю",
			"user_id", chatId,
			"error", err,
			"message", errorMsg)

		// Если ошибка связана с форматированием, пробуем отправить без Markdown
		if strings.Contains(err.Error(), "can't parse entities") || strings.Contains(err.Error(), "Bad Request") {
			newsLogger.Warn("Ошибка парсинга Markdown для пользователя, пробуем отправить без форматирования",
				"user_id", chatId)
			// Формируем простое сообщение без Markdown
			simpleMessage := ""
			for i, news := range newsList {
				// Используем функцию из пакета msg для форматирования времени
				relativeTime := formatRelativeTime(news.PublishedAt)
				// Добавляем ссылку на новость в конце строки
				simpleMessage += fmt.Sprintf("%d. %s - %s • %s\n%s\n\n",
					i+1, news.Title, news.SourceName, relativeTime, news.Link)
			}
			simpleMessage = strings.TrimRight(simpleMessage, "\n")

			simpleMsg := tgbotapi.NewMessage(chatId, simpleMessage)
			simpleMsg.DisableWebPagePreview = true
			// Не устанавливаем ParseMode, отправляем как обычный текст

			if _, sendErr := np.bot.Send(simpleMsg); sendErr != nil {
				newsLogger.Error("Ошибка отправки простого сообщения пользователю",
					"user_id", chatId,
					"error", sendErr)
				return false
			}
			// Если простое сообщение отправилось успешно, продолжаем
		} else {
			return false
		}
	}

	// Сохраняем информацию об отправке в таблицу messages
	tx, err := np.db.Begin()
	if err != nil {
		newsLogger.Error("Ошибка начала транзакции для сохранения сообщений", "error", err)
		return false
	}

	// Сохраняем все отправленные новости
	saveErrors := false
	for _, news := range newsList {
		if err := db.SaveMessage(tx, chatId, news.NewsID); err != nil {
			newsLogger.Error("Ошибка сохранения сообщения для новости",
				"news_id", news.NewsID,
				"error", err)
			saveErrors = true
			// Продолжаем сохранять остальные
		}
	}

	if err := tx.Commit(); err != nil {
		newsLogger.Error("Ошибка коммита транзакции", "error", err)
		return false
	}

	// Если были ошибки сохранения, но транзакция прошла, логируем предупреждение
	if saveErrors {
		newsLogger.Warn("Некоторые новости не были сохранены в БД для пользователя, но сообщение отправлено",
			"user_id", chatId)
	}

	monitoring.IncrementTelegramMessagesSent()
	newsLogger.Debug("Сообщение с новостями отправлено пользователю",
		"news_count", len(newsList),
		"user_id", chatId)

	// После успешной отправки постепенно уменьшаем интервал, если он был увеличен
	currentInterval = np.globalRateLimiter.GetMinInterval()
	if currentInterval > MinRateLimitInterval {
		newInterval := currentInterval * 90 / 100
		if newInterval < MinRateLimitInterval {
			newInterval = MinRateLimitInterval
		}
		np.globalRateLimiter.SetMinInterval(newInterval)
	}

	return true
}

// getSubscriptionsCached возвращает подписки для источника с кэшированием
func (np *NewsProcessor) getSubscriptionsCached(sourceID int64) ([]db.Subscription, error) {
	np.cacheMutex.RLock()
	if cachedSubs, exists := np.subscriptionsCache[sourceID]; exists &&
		time.Since(np.lastCacheUpdate) < np.cacheDuration {
		np.cacheMutex.RUnlock()
		// Преобразуем interface{} обратно в db.Subscription
		subscriptions := make([]db.Subscription, len(cachedSubs))
		for i, sub := range cachedSubs {
			if s, ok := sub.(db.Subscription); ok {
				subscriptions[i] = s
			}
		}
		return subscriptions, nil
	}
	np.cacheMutex.RUnlock()

	// Кэш устарел или не существует, обновляем
	np.cacheMutex.Lock()
	defer np.cacheMutex.Unlock()

	// Проверяем еще раз, вдруг другой горутина уже обновила
	if cachedSubs, exists := np.subscriptionsCache[sourceID]; exists &&
		time.Since(np.lastCacheUpdate) < np.cacheDuration {
		// Преобразуем interface{} обратно в db.Subscription
		subscriptions := make([]db.Subscription, len(cachedSubs))
		for i, sub := range cachedSubs {
			if s, ok := sub.(db.Subscription); ok {
				subscriptions[i] = s
			}
		}
		return subscriptions, nil
	}

	monitoring.IncrementDBQueries()
	subscriptions, err := db.GetSubscriptions(np.db, sourceID)
	if err != nil {
		monitoring.IncrementDBQueriesErrors()
		return nil, err
	}

	// Обновляем кэш для всех источников (оптимизация)
	allSubscriptions := make(map[int64][]interface{})
	for _, sub := range subscriptions {
		allSubscriptions[sub.SourceId] = append(allSubscriptions[sub.SourceId], sub)
	}

	np.subscriptionsCache = allSubscriptions
	np.lastCacheUpdate = time.Now()

	newsLogger.Debug("Кэш подписок обновлен",
		"sources_count", len(allSubscriptions))
	return subscriptions, nil
}

// startPeriodicCleanup запускает периодическую очистку старых новостей из очереди
func (np *NewsProcessor) startPeriodicCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		np.cleanupOldNews()
	}
}

// cleanupOldNews удаляет устаревшие новости из очереди
func (np *NewsProcessor) cleanupOldNews() {
	np.pendingMutex.Lock()
	defer np.pendingMutex.Unlock()

	now := time.Now()
	totalRemoved := 0
	usersAffected := 0

	for chatID, newsQueue := range np.pendingNews {
		// Фильтруем только актуальные новости (не старше TTL)
		validNews := make([]PendingNews, 0, len(newsQueue))
		removed := 0

		for _, news := range newsQueue {
			if now.Sub(news.AddedAt) < PendingNewsTTL {
				validNews = append(validNews, news)
			} else {
				removed++
			}
		}

		if removed > 0 {
			np.pendingNews[chatID] = validNews
			totalRemoved += removed
			usersAffected++

			newsLogger.Debug("Очищены устаревшие новости из очереди",
				"user_id", chatID,
				"removed", removed,
				"remaining", len(validNews))
		}

		// Удаляем пустые очереди
		if len(validNews) == 0 {
			delete(np.pendingNews, chatID)
		}
	}

	if totalRemoved > 0 {
		newsLogger.Info("Периодическая очистка завершена",
			"removed_news", totalRemoved,
			"users_affected", usersAffected,
			"remaining_users", len(np.pendingNews))
	}
}
