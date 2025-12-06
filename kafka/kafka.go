package kafka

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
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
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

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

// SendNewsToSubscribers отправляет новость всем подписчикам источника с проверкой на дубликаты
func (p *Producer) SendNewsToSubscribers(db *sql.DB, chatIDs []int64, news NewsItem, sourceID int64, sourceName string) error {
	// Сортируем chatIDs для детерминированного порядка
	sort.Slice(chatIDs, func(i, j int) bool { return chatIDs[i] < chatIDs[j] })

	// Получаем ID новости по ссылке или создаем новую запись
	var newsID int64
	err := db.QueryRow(
		`INSERT INTO news (title, description, link, published_at, source_id) 
		  VALUES ($1, $2, $3, $4, $5) 
		  ON CONFLICT (link) DO UPDATE SET title = $1, description = $2, published_at = $4
		  RETURNING id`,
		news.Title,
		news.Description,
		news.Link,
		time.Now(), // Используем текущее время, так как PublishedAt уже парсится в rss.ParseRSS
		sourceID,
	).Scan(&newsID)

	if err != nil {
		return fmt.Errorf("ошибка сохранения новости в БД: %v", err)
	}

	// Отправляем уведомления каждому подписчику
	for _, chatID := range chatIDs {
		// Проверяем, не отправляли ли уже эту новость пользователю
		var count int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM messages 
			 WHERE chat_id = $1 AND news_id = $2`,
			chatID, newsID,
		).Scan(&count)

		if err != nil {
			log.Printf("Ошибка проверки отправленной новости: %v", err)
			continue
		}

		if count > 0 {
			log.Printf("Новость %d уже отправлялась пользователю %d, пропускаем", newsID, chatID)
			continue
		}

		// Отправляем уведомление
		notification := NewsNotification{
			ChatID:      chatID,
			NewsID:      newsID,
			SourceID:    sourceID,
			SourceName:  sourceName,
			Title:       news.Title,
			Description: news.Description,
			Link:        news.Link,
			PublishedAt: news.PublishedAt,
		}

		if err := p.SendNewsNotification(notification); err != nil {
			log.Printf("Ошибка отправки уведомления пользователю %d: %v", chatID, err)
			continue
		}

		// Помечаем новость как отправленную
		_, err = db.Exec(
			`INSERT INTO messages (chat_id, news_id, sent_at) 
			 VALUES ($1, $2, $3) 
			 ON CONFLICT (chat_id, news_id) DO NOTHING`,
			chatID, newsID, time.Now(),
		)

		if err != nil {
			log.Printf("Ошибка пометки новости как отправленной: %v", err)
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
