package redpanda

import (
	"encoding/json"
	"fmt"
	"log"
	"tg-rss/config"
	"tg-rss/rss"

	"github.com/IBM/sarama"
)

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

type NewsItem struct {
	SourceID    int64  `json:"source_id"`
	SourceName  string `json:"source_name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
	PublishedAt string `json:"published_at"`
}

type Producer struct {
	producer    sarama.SyncProducer
	newsTopic   string
	notifyTopic string
}

type Consumer struct {
	consumer    sarama.Consumer
	newsTopic   string
	notifyTopic string
}

// NewProducer создает новый Redpanda producer
func NewProducer(redpandaConfig *config.RedpandaConfig) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewSyncProducer(redpandaConfig.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Redpanda producer: %v", err)
	}

	return &Producer{
		producer:    producer,
		newsTopic:   redpandaConfig.NewsTopic,
		notifyTopic: redpandaConfig.NotifyTopic,
	}, nil
}

// NewConsumer создает новый Redpanda consumer
func NewConsumer(redpandaConfig *config.RedpandaConfig) (*Consumer, error) {
	log.Printf("Создание consumer с brokers: %v", redpandaConfig.Brokers)
	consumer, err := sarama.NewConsumer(redpandaConfig.Brokers, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Redpanda consumer: %v", err)
	}

	return &Consumer{
		consumer:    consumer,
		newsTopic:   redpandaConfig.NewsTopic,
		notifyTopic: redpandaConfig.NotifyTopic,
	}, nil
}

// SendNewsNotification отправляет уведомление о новости в Redpanda
func (p *Producer) SendNewsNotification(notification NewsNotification) error {
	message, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("ошибка сериализации уведомления: %v", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.notifyTopic,
		Value: sarama.StringEncoder(message),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения в Redpanda: %v", err)
	}

	log.Printf("Сообщение отправлено в Redpanda: topic=%s, partition=%d, offset=%d", p.notifyTopic, partition, offset)
	return nil
}

// SendNewsToSubscribers отправляет новость всем подписчикам источника
func (p *Producer) SendNewsToSubscribers(chatIDs []int64, news rss.News, sourceID int64, sourceName string) error {
	for _, chatID := range chatIDs {
		notification := NewsNotification{
			ChatID:      chatID,
			NewsID:      0, // Будет заполнено после сохранения в БД
			SourceID:    sourceID,
			SourceName:  sourceName,
			Title:       news.Title,
			Description: news.Description,
			Link:        news.Link,
			PublishedAt: news.PublishedAt.Format("2006-01-02 15:04:05"),
		}

		if err := p.SendNewsNotification(notification); err != nil {
			log.Printf("Ошибка отправки уведомления для chat_id %d: %v", chatID, err)
			continue
		}
	}
	return nil
}

// SendNewsItem отправляет новость в очередь для обработки
func (p *Producer) SendNewsItem(newsItem NewsItem) error {
	message, err := json.Marshal(newsItem)
	if err != nil {
		return fmt.Errorf("ошибка сериализации новости: %v", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.newsTopic,
		Value: sarama.StringEncoder(message),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("ошибка отправки новости в Redpanda: %v", err)
	}

	log.Printf("Новость отправлена в Redpanda: topic=%s, partition=%d, offset=%d", p.newsTopic, partition, offset)
	return nil
}

// Close закрывает producer
func (p *Producer) Close() error {
	return p.producer.Close()
}

// StartConsuming начинает потребление сообщений из Redpanda
func (c *Consumer) StartConsuming(handler func(interface{}) error) error {
	partitionConsumer, err := c.consumer.ConsumePartition(c.newsTopic, 0, sarama.OffsetNewest)
	if err != nil {
		return fmt.Errorf("ошибка создания partition consumer: %v", err)
	}
	defer partitionConsumer.Close()

	for message := range partitionConsumer.Messages() {
		// Пытаемся десериализовать как NewsItem
		var newsItem NewsItem
		if err := json.Unmarshal(message.Value, &newsItem); err == nil {
			if err := handler(newsItem); err != nil {
				log.Printf("Ошибка обработки новости: %v", err)
			}
			continue
		}

		// Пытаемся десериализовать как NewsNotification
		var notification NewsNotification
		if err := json.Unmarshal(message.Value, &notification); err == nil {
			if err := handler(notification); err != nil {
				log.Printf("Ошибка обработки уведомления: %v", err)
			}
			continue
		}

		log.Printf("Неизвестный тип сообщения в Redpanda")
	}

	return nil
}

// Close закрывает consumer
func (c *Consumer) Close() error {
	return c.consumer.Close()
}
