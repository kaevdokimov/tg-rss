package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"tg-rss/config"

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
	consumer    sarama.ConsumerGroup
	newsTopic   string
	notifyTopic string
	groupID     string
}

// NewProducer создает новый Kafka producer
func NewProducer(kafkaConfig *config.KafkaConfig) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForLocal // Оптимизация: локальное подтверждение вместо WaitForAll
	config.Producer.Retry.Max = 3 // Снижаем количество повторных попыток
	config.Producer.Compression = sarama.CompressionSnappy // Сжатие для экономии трафика
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Частая отправка мелких батчей
	config.Producer.Flush.Messages = 100 // Отправка при накоплении 100 сообщений

	producer, err := sarama.NewSyncProducer(kafkaConfig.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka producer: %v", err)
	}

	return &Producer{
		producer:    producer,
		newsTopic:   kafkaConfig.NewsTopic,
		notifyTopic: kafkaConfig.NotifyTopic,
	}, nil
}

// NewConsumer создает новый Kafka consumer с группой
func NewConsumer(kafkaConfig *config.KafkaConfig) (*Consumer, error) {
	log.Printf("Создание consumer с brokers: %v", kafkaConfig.Brokers)

	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Fetch.Min = 1 // Минимальный размер батча для экономии памяти
	config.Consumer.Fetch.Max = 1048576 // 1MB максимум
	config.Consumer.MaxWaitTime = 500 * time.Millisecond // Таймаут ожидания сообщений
	config.Consumer.MaxProcessingTime = 100 * time.Millisecond // Время обработки

	// Создаем consumer group
	consumerGroup, err := sarama.NewConsumerGroup(kafkaConfig.Brokers, "tg-bot-group", config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka consumer group: %v", err)
	}

	return &Consumer{
		consumer:    consumerGroup,
		newsTopic:   kafkaConfig.NewsTopic,
		notifyTopic: kafkaConfig.NotifyTopic,
		groupID:     "tg-bot-group",
	}, nil
}

// SendNewsNotification отправляет уведомление о новости в Kafka
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
		return fmt.Errorf("ошибка отправки сообщения в Kafka: %v", err)
	}

	log.Printf("Сообщение отправлено в Kafka: topic=%s, partition=%d, offset=%d", p.notifyTopic, partition, offset)
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
		return fmt.Errorf("ошибка отправки новости в Kafka: %v", err)
	}

	log.Printf("Новость отправлена в Kafka: topic=%s, partition=%d, offset=%d", p.newsTopic, partition, offset)
	return nil
}

// Close закрывает producer
func (p *Producer) Close() error {
	return p.producer.Close()
}

// Close закрывает consumer
func (c *Consumer) Close() error {
	if c.consumer != nil {
		return c.consumer.Close()
	}
	return nil
}

// consumerHandler реализует sarama.ConsumerGroupHandler
// для обработки сообщений из Kafka
type consumerHandler struct {
	handler func(any) error
}

// Setup вызывается перед началом потребления
func (h *consumerHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Println("Настройка consumer group")
	return nil
}

// Cleanup вызывается после завершения потребления
func (h *consumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("Завершение consumer group")
	return nil
}

// ConsumeClaim обрабатывает сообщения из партиции
func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var data any
		var err error

		switch message.Topic {
		case "news-items":
			var newsItem NewsItem
			err = json.Unmarshal(message.Value, &newsItem)
			data = newsItem
		case "news-notifications":
			var notification NewsNotification
			err = json.Unmarshal(message.Value, &notification)
			data = notification
		}

		if err != nil {
			log.Printf("Ошибка десериализации сообщения из топика %s: %v", message.Topic, err)
			session.MarkMessage(message, "")
			continue
		}

		// Обрабатываем сообщение
		if err := h.handler(data); err != nil {
			log.Printf("Ошибка обработки сообщения: %v", err)
		}

		// Подтверждаем обработку сообщения
		session.MarkMessage(message, "")
	}
	return nil
}

// StartConsuming начинает потребление сообщений из Kafka
func (c *Consumer) StartConsuming(handler func(interface{}) error) error {
	topics := []string{c.newsTopic, c.notifyTopic}

	// Создаем обработчик для consumer group
	h := &consumerHandler{
		handler: handler,
	}

	// Запускаем бесконечный цикл для обработки перебалансировок
	go func() {
		for {
			// Эта функция будет пересоздавать сессию при необходимости
			err := c.consumer.Consume(context.Background(), topics, h)
			if err != nil {
				log.Printf("Ошибка потребления из Kafka: %v", err)
				time.Sleep(5 * time.Second) // Задержка перед повторной попыткой
			}

			// Проверяем, не закрыт ли consumer
			if err == sarama.ErrClosedConsumerGroup {
				return
			}

			time.Sleep(1 * time.Second)
		}
	}()

	return nil
}
