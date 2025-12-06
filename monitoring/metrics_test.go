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
		t.Errorf("Ожидался RSSPollsTotal=1, получено %d", metrics.RSSPollsTotal)
	}
	if metrics.RSSItemsProcessed != 1 {
		t.Errorf("Ожидался RSSItemsProcessed=1, получено %d", metrics.RSSItemsProcessed)
	}
	if metrics.KafkaMessagesProduced != 1 {
		t.Errorf("Ожидался KafkaMessagesProduced=1, получено %d", metrics.KafkaMessagesProduced)
	}
	if metrics.KafkaMessagesConsumed != 1 {
		t.Errorf("Ожидался KafkaMessagesConsumed=1, получено %d", metrics.KafkaMessagesConsumed)
	}
	if metrics.TelegramMessagesSent != 1 {
		t.Errorf("Ожидался TelegramMessagesSent=1, получено %d", metrics.TelegramMessagesSent)
	}
	if metrics.TelegramCommandsTotal != 1 {
		t.Errorf("Ожидался TelegramCommandsTotal=1, получено %d", metrics.TelegramCommandsTotal)
	}
	if metrics.DBQueriesTotal != 1 {
		t.Errorf("Ожидался DBQueriesTotal=1, получено %d", metrics.DBQueriesTotal)
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
		t.Errorf("Ожидался RSSPollsErrors=1, получено %d", metrics.RSSPollsErrors)
	}
	if metrics.KafkaErrors != 1 {
		t.Errorf("Ожидался KafkaErrors=1, получено %d", metrics.KafkaErrors)
	}
	if metrics.TelegramMessagesErrors != 1 {
		t.Errorf("Ожидался TelegramMessagesErrors=1, получено %d", metrics.TelegramMessagesErrors)
	}
	if metrics.DBQueriesErrors != 1 {
		t.Errorf("Ожидался DBQueriesErrors=1, получено %d", metrics.DBQueriesErrors)
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
		t.Errorf("Ожидался RSSPollsTotal=10, получено %d", metrics.RSSPollsTotal)
	}
	if metrics.KafkaMessagesProduced != 10 {
		t.Errorf("Ожидался KafkaMessagesProduced=10, получено %d", metrics.KafkaMessagesProduced)
	}
}

func TestMetricsLastUpdate(t *testing.T) {
	Reset()

	time.Sleep(10 * time.Millisecond)
	IncrementRSSPolls()

	metrics := GetMetrics()
	if metrics.LastUpdate.IsZero() {
		t.Error("Ожидалось, что LastUpdate будет установлено")
	}
}
