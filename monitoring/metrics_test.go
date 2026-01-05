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
	IncrementRedisMessagesProduced()
	IncrementRedisMessagesConsumed()
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
	if metrics.RedisMessagesProduced != 1 {
		t.Errorf("Ожидался RedisMessagesProduced=1, получено %d", metrics.RedisMessagesProduced)
	}
	if metrics.RedisMessagesConsumed != 1 {
		t.Errorf("Ожидался RedisMessagesConsumed=1, получено %d", metrics.RedisMessagesConsumed)
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
	IncrementRedisErrors()
	IncrementTelegramMessagesErrors()
	IncrementDBQueriesErrors()

	metrics := GetMetrics()

	if metrics.RSSPollsErrors != 1 {
		t.Errorf("Ожидался RSSPollsErrors=1, получено %d", metrics.RSSPollsErrors)
	}
	if metrics.RedisErrors != 1 {
		t.Errorf("Ожидался RedisErrors=1, получено %d", metrics.RedisErrors)
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
			IncrementRedisMessagesProduced()
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
	if metrics.RedisMessagesProduced != 10 {
		t.Errorf("Ожидался RedisMessagesProduced=10, получено %d", metrics.RedisMessagesProduced)
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
