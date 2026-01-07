package redis

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"tg-rss/config"

	"github.com/go-redis/redis/v8"
)

type NewsItem struct {
	SourceID    int64  `json:"source_id"`
	SourceName  string `json:"source_name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
	PublishedAt string `json:"published_at"`
}

type NewsNotification struct {
	ChatID      int64  `json:"chat_id"`
	NewsID      int64  `json:"news_id"`
	SourceID    int64  `json:"source_id"`
	SourceName  string `json:"source_name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
	PublishedAt string `json:"published_at"`
}

// CachedNewsContent представляет контент новости для кэширования в Redis
type CachedNewsContent struct {
	FullText        string            `json:"full_text"`
	Author          string            `json:"author"`
	Category        string            `json:"category"`
	Tags            []string          `json:"tags"`
	Images          []string          `json:"images"`
	PublishedAt     *time.Time        `json:"published_at,omitempty"`
	MetaKeywords    string            `json:"meta_keywords"`
	MetaDescription string            `json:"meta_description"`
	MetaData        map[string]string `json:"meta_data"`
	ContentHTML     string            `json:"content_html"`
}

type Producer struct {
	client        *redis.Client
	newsChannel   string
	notifyChannel string
}

type Consumer struct {
	client        *redis.Client
	newsChannel   string
	notifyChannel string
}

// NewProducer создает новый Redis producer
func NewProducer(redisConfig *config.RedisConfig) (*Producer, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     10, // connection pooling
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Printf("Redis producer подключен к %s", redisConfig.Addr)

	return &Producer{
		client:        client,
		newsChannel:   redisConfig.NewsChannel,
		notifyChannel: redisConfig.NotifyChannel,
	}, nil
}

// PublishNews отправляет новость в Redis pub/sub
func (p *Producer) PublishNews(news NewsItem) error {
	data, err := json.Marshal(news)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return p.client.Publish(ctx, p.newsChannel, data).Err()
}

// PublishNotification отправляет уведомление о новости
func (p *Producer) PublishNotification(notification NewsNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return p.client.Publish(ctx, p.notifyChannel, data).Err()
}

// NewConsumer создает новый Redis consumer
func NewConsumer(redisConfig *config.RedisConfig) (*Consumer, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Printf("Redis consumer подключен к %s", redisConfig.Addr)

	return &Consumer{
		client:        client,
		newsChannel:   redisConfig.NewsChannel,
		notifyChannel: redisConfig.NotifyChannel,
	}, nil
}

// SubscribeNews подписывается на новости
func (c *Consumer) SubscribeNews(handler func(NewsItem) error) error {
	pubsub := c.client.Subscribe(context.Background(), c.newsChannel)
	defer pubsub.Close()

	log.Printf("Redis consumer подписан на канал: %s", c.newsChannel)

	for {
		msg, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			log.Printf("Ошибка получения сообщения из Redis: %v", err)
			return err
		}

		var news NewsItem
		if err := json.Unmarshal([]byte(msg.Payload), &news); err != nil {
			log.Printf("Ошибка десериализации новости: %v", err)
			continue // skip invalid messages
		}

		if err := handler(news); err != nil {
			log.Printf("Ошибка обработки новости: %v", err)
			continue
		}
	}
}

// SubscribeNotifications подписывается на уведомления
func (c *Consumer) SubscribeNotifications(handler func(NewsNotification) error) error {
	pubsub := c.client.Subscribe(context.Background(), c.notifyChannel)
	defer pubsub.Close()

	log.Printf("Redis consumer подписан на канал уведомлений: %s", c.notifyChannel)

	for {
		msg, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			log.Printf("Ошибка получения уведомления из Redis: %v", err)
			return err
		}

		var notification NewsNotification
		if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
			log.Printf("Ошибка десериализации уведомления: %v", err)
			continue // skip invalid messages
		}

		if err := handler(notification); err != nil {
			log.Printf("Ошибка обработки уведомления: %v", err)
			continue
		}
	}
}

// ContentCache представляет кэш для скрапированного контента
type ContentCache struct {
	client *redis.Client
	prefix string
}

// NewContentCache создает новый кэш контента
func NewContentCache(redisConfig *config.RedisConfig) (*ContentCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     20, // больше соединений для кэша
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Printf("Redis content cache подключен к %s", redisConfig.Addr)

	return &ContentCache{
		client: client,
		prefix: "content:",
	}, nil
}

// getCacheKey генерирует ключ кэша для URL
func (cc *ContentCache) getCacheKey(url string) string {
	// Используем простой хэш URL как ключ
	return cc.prefix + fmt.Sprintf("%x", md5.Sum([]byte(url)))
}

// Get получает контент из кэша
func (cc *ContentCache) Get(url string) (*CachedNewsContent, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := cc.getCacheKey(url)
	data, err := cc.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Ошибка чтения из Redis кэша: %v", err)
		}
		return nil, false
	}

	var content CachedNewsContent
	if err := json.Unmarshal([]byte(data), &content); err != nil {
		log.Printf("Ошибка десериализации контента из кэша: %v", err)
		return nil, false
	}

	return &content, true
}

// Set сохраняет контент в кэш
func (cc *ContentCache) Set(url string, content *CachedNewsContent, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := cc.getCacheKey(url)
	data, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("ошибка сериализации контента: %w", err)
	}

	return cc.client.Set(ctx, key, data, ttl).Err()
}

// Delete удаляет контент из кэша
func (cc *ContentCache) Delete(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := cc.getCacheKey(url)
	return cc.client.Del(ctx, key).Err()
}

// Close закрывает соединения
func (p *Producer) Close() error {
	return p.client.Close()
}

func (c *Consumer) Close() error {
	return c.client.Close()
}

func (cc *ContentCache) Close() error {
	return cc.client.Close()
}

// GetStats возвращает статистику использования кэша (для отладки)
func (cc *ContentCache) GetStats() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	info, err := cc.client.Info(ctx, "keyspace").Result()
	if err != nil {
		return nil, err
	}

	// Парсим информацию о БД
	stats := make(map[string]interface{})
	stats["info"] = info

	return stats, nil
}
