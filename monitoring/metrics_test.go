package monitoring

import (
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	Reset()

	// Тестируем инкременты
	IncrementRSSPolls()
	IncrementRSSItemsProcessed()
	IncrementKafkaMessagesProduced()
	IncrementKafkaMessagesConsumed()
	IncrementTelegramMessagesSent()
	IncrementTelegramCommands()
	IncrementDBQueries()

	metrics := GetMetrics()

	if metrics.RSSPollsTotal != 1 {
		t.Errorf("Expected RSSPollsTotal=1, got %d", metrics.RSSPollsTotal)
	}
	if metrics.RSSItemsProcessed != 1 {
		t.Errorf("Expected RSSItemsProcessed=1, got %d", metrics.RSSItemsProcessed)
	}
	if metrics.KafkaMessagesProduced != 1 {
		t.Errorf("Expected KafkaMessagesProduced=1, got %d", metrics.KafkaMessagesProduced)
	}
	if metrics.KafkaMessagesConsumed != 1 {
		t.Errorf("Expected KafkaMessagesConsumed=1, got %d", metrics.KafkaMessagesConsumed)
	}
	if metrics.TelegramMessagesSent != 1 {
		t.Errorf("Expected TelegramMessagesSent=1, got %d", metrics.TelegramMessagesSent)
	}
	if metrics.TelegramCommandsTotal != 1 {
		t.Errorf("Expected TelegramCommandsTotal=1, got %d", metrics.TelegramCommandsTotal)
	}
	if metrics.DBQueriesTotal != 1 {
		t.Errorf("Expected DBQueriesTotal=1, got %d", metrics.DBQueriesTotal)
	}
}

func TestMetricsErrors(t *testing.T) {
	Reset()

	IncrementRSSPollsErrors()
	IncrementKafkaErrors()
	IncrementTelegramMessagesErrors()
	IncrementDBQueriesErrors()

	metrics := GetMetrics()

	if metrics.RSSPollsErrors != 1 {
		t.Errorf("Expected RSSPollsErrors=1, got %d", metrics.RSSPollsErrors)
	}
	if metrics.KafkaErrors != 1 {
		t.Errorf("Expected KafkaErrors=1, got %d", metrics.KafkaErrors)
	}
	if metrics.TelegramMessagesErrors != 1 {
		t.Errorf("Expected TelegramMessagesErrors=1, got %d", metrics.TelegramMessagesErrors)
	}
	if metrics.DBQueriesErrors != 1 {
		t.Errorf("Expected DBQueriesErrors=1, got %d", metrics.DBQueriesErrors)
	}
}

func TestMetricsConcurrency(t *testing.T) {
	Reset()

	// Тестируем конкурентный доступ
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			IncrementRSSPolls()
			IncrementKafkaMessagesProduced()
			done <- true
		}()
	}

	// Ждем завершения всех горутин
	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := GetMetrics()
	if metrics.RSSPollsTotal != 10 {
		t.Errorf("Expected RSSPollsTotal=10, got %d", metrics.RSSPollsTotal)
	}
	if metrics.KafkaMessagesProduced != 10 {
		t.Errorf("Expected KafkaMessagesProduced=10, got %d", metrics.KafkaMessagesProduced)
	}
}

func TestMetricsLastUpdate(t *testing.T) {
	Reset()

	time.Sleep(10 * time.Millisecond)
	IncrementRSSPolls()

	metrics := GetMetrics()
	if metrics.LastUpdate.IsZero() {
		t.Error("Expected LastUpdate to be set")
	}
}
