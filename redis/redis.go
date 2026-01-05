package redis

import (
	"context"
	"encoding/json"
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

// Close закрывает соединения
func (p *Producer) Close() error {
	return p.client.Close()
}

func (c *Consumer) Close() error {
	return c.client.Close()
}
