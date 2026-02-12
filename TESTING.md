# Руководство по тестированию

Документация по запуску и написанию тестов для проекта TG-RSS.

## Оглавление

- [Типы тестов](#типы-тестов)
- [Пакеты без тестов](#пакеты-без-тестов)
- [Запуск тестов](#запуск-тестов)
- [Написание тестов](#написание-тестов)
- [Покрытие кода](#покрытие-кода)
- [CI/CD](#cicd)
- [Best Practices](#best-practices)

## Типы тестов

### Unit тесты

Тестируют отдельные функции и методы в изоляции.

**Расположение**: `*_test.go` рядом с тестируемым файлом

**Примеры**:
- `api/handlers_test.go` - валидация API
- `bot/circuit_breaker_test.go` - circuit breaker логика
- `cache/cache_test.go` - кэширование
- `bot/rate_limiter_test.go` - rate limiting
- `monitoring/logger_test.go` - логирование

### Integration тесты

Тестируют взаимодействие компонентов с реальными зависимостями.

**Примеры**:
- `db/db_test.go` - работа с PostgreSQL
- `scraper/scraper_test.go` - HTTP запросы к реальным сайтам

### Benchmark тесты

Измеряют производительность критичных участков кода.

**Примеры**:
- `redis/cache_benchmark_test.go` - производительность Redis
- `cache/cache_test.go` - производительность in-memory кэша

### E2E тесты

> В разработке. Планируется тестирование полного цикла работы бота.

## Пакеты без тестов

Некоторые пакеты не содержат `*_test.go` и сознательно исключены из обязательного покрытия:

| Пакет | Причина |
|-------|---------|
| `db/migrations` | Миграции — это SQL-скрипты и одноразовый код применения схемы; проверяются при деплое и ручном прогоне. |
| `middleware` | Тонкая обёртка над стандартным HTTP; логика покрывается косвенно через тесты API. |
| `scripts` | Вспомогательные утилиты (отладка, хуки); не входят в основной бинарник. |

При добавлении новой бизнес-логики в эти пакеты рекомендуется вынести её в тестируемый слой или добавить unit-тесты.

## Запуск тестов

### Все тесты

```bash
# Запустить все тесты
go test ./...

# С подробным выводом
go test -v ./...

# С race detector
go test -race ./...
```

### Конкретный пакет

```bash
# Тесты API
go test ./api/...

# Тесты бота
go test ./bot/...

# Тесты БД
go test ./db/...

# Тесты кэша
go test ./cache/...
```

### Конкретный тест

```bash
# Запуск одного теста
go test -run TestValidateURL ./api/...

# С подробным выводом
go test -v -run TestValidateURL ./api/...
```

### Unit тесты (без integration)

```bash
# Пропустить долгие тесты
go test -short ./...
```

В коде отмечаем integration тесты:
```go
func TestDatabaseIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Пропускаем integration тест в short режиме")
    }
    // тест
}
```

### Benchmark тесты

```bash
# Запустить benchmarks
go test -bench=. ./...

# С выделением памяти
go test -bench=. -benchmem ./...

# Конкретный benchmark
go test -bench=BenchmarkCache ./cache/...
```

### С покрытием

```bash
# Покрытие всего проекта
go test -coverprofile=coverage.out ./...

# Просмотр покрытия в браузере
go tool cover -html=coverage.out

# Покрытие по пакетам
go test -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -func=coverage.out
```

## Написание тестов

### Структура теста

```go
package mypackage

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFunction(t *testing.T) {
    // Arrange - подготовка данных
    input := "test"
    expected := "result"
    
    // Act - выполнение
    result := MyFunction(input)
    
    // Assert - проверка
    assert.Equal(t, expected, result)
}
```

### Таблично-ориентированные тесты

```go
func TestValidateURL(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "Valid HTTP URL",
            url:     "http://example.com",
            wantErr: false,
        },
        {
            name:    "Empty URL",
            url:     "",
            wantErr: true,
            errMsg:  "URL не может быть пустым",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateURL(tt.url)
            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Тестирование с mock'ами

Для изоляции внешних зависимостей используем mock'и:

```go
// Определяем интерфейс (будущая архитектура)
type NewsRepository interface {
    Save(news *News) error
    Find(id int64) (*News, error)
}

// Mock реализация
type MockNewsRepository struct {
    SaveFunc func(news *News) error
    FindFunc func(id int64) (*News, error)
}

func (m *MockNewsRepository) Save(news *News) error {
    return m.SaveFunc(news)
}

func (m *MockNewsRepository) Find(id int64) (*News, error) {
    return m.FindFunc(id)
}

// Использование в тесте
func TestNewsService(t *testing.T) {
    mockRepo := &MockNewsRepository{
        SaveFunc: func(news *News) error {
            return nil // Успешное сохранение
        },
    }
    
    service := NewNewsService(mockRepo)
    err := service.ProcessNews(testNews)
    assert.NoError(t, err)
}
```

### Конкурентные тесты

```go
func TestConcurrentAccess(t *testing.T) {
    cache := NewCache(5 * time.Minute)
    
    var wg sync.WaitGroup
    numGoroutines := 100
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            // Параллельная запись и чтение
            cache.Set("key", id)
            cache.Get("key")
        }(i)
    }
    
    wg.Wait()
    
    // Проверяем, что кэш в рабочем состоянии
    assert.NotPanics(t, func() {
        cache.Size()
    })
}
```

### Integration тесты с testcontainers

```go
// Планируется использование testcontainers для БД
func TestDatabaseIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Пропускаем integration тест")
    }
    
    // TODO: Использовать testcontainers для PostgreSQL
    // ctx := context.Background()
    // postgres, err := testcontainers.GenericContainer(ctx, ...)
}
```

### Benchmark тесты

```go
func BenchmarkCacheSet(b *testing.B) {
    c := NewCache(5 * time.Minute)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        c.Set("key", "value")
    }
}

func BenchmarkCacheConcurrent(b *testing.B) {
    c := NewCache(5 * time.Minute)
    
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            c.Set(fmt.Sprintf("key%d", i), i)
            i++
        }
    })
}
```

## Покрытие кода

### Текущее покрытие

По состоянию на последний коммит:

| Пакет | Покрытие | Статус |
|-------|----------|--------|
| `api` | ~40% | 🟡 Требуется улучшение |
| `bot` | ~30% | 🟡 Требуется улучшение |
| `cache` | ~90% | 🟢 Хорошо |
| `config` | ~70% | 🟢 Хорошо |
| `db` | ~25% | 🔴 Низкое |
| `monitoring` | ~80% | 🟢 Хорошо |
| `scraper` | ~50% | 🟡 Требуется улучшение |

**Цель**: 70-80% покрытие для всех критичных пакетов

### Генерация отчета

```bash
# Создать отчет покрытия
go test -coverprofile=coverage.out -covermode=atomic ./...

# HTML отчет
go tool cover -html=coverage.out -o coverage.html

# Текстовый отчет
go tool cover -func=coverage.out

# Отправка в Codecov (в CI)
bash <(curl -s https://codecov.io/bash)
```

### Анализ покрытия

```bash
# Показать непокрытые строки
go test -cover -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep -v "100.0%"
```

## CI/CD

### GitHub Actions

Тесты автоматически запускаются в CI:

#### Workflow: `go.yml`

```yaml
- Запуск на: push, pull_request
- Go версии: 1.25.5, 1.25
- Платформы: linux/amd64, linux/arm64
- Проверки:
  - go test -race
  - golangci-lint
  - govulncheck
  - Покрытие → Codecov
```

#### Workflow: `ci-cd.yml`

```yaml
- Запуск тестов перед сборкой
- Проверка качества кода
- Сборка Docker образов только после успешных тестов
```

### Локальная проверка перед commit

```bash
# Запустить все проверки как в CI
make test-ci

# Или вручную
go test -race -coverprofile=coverage.out ./...
golangci-lint run
go vet ./...
```

## Best Practices

### Общие рекомендации

1. **Один тест - одна проверка**
   - Тест должен проверять одну конкретную вещь
   - Используйте подтесты (`t.Run`) для группировки

2. **Используйте t.Parallel()**
   ```go
   func TestMyFunction(t *testing.T) {
       t.Parallel() // Запускать параллельно
       // тест
   }
   ```

3. **Используйте assert/require из testify**
   - `assert` - продолжает выполнение после ошибки
   - `require` - останавливает тест при ошибке

4. **Именование тестов**
   - `TestFunctionName` для функций
   - `TestStructName_MethodName` для методов
   - Описательные имена подтестов

5. **Изолируйте тесты**
   - Не зависьте от порядка выполнения
   - Очищайте состояние после теста
   - Используйте `t.Cleanup()` или `defer`

### Структура тестов

```
package/
├── file.go
├── file_test.go         # Unit тесты
├── testdata/            # Тестовые данные
│   ├── input.json
│   └── expected.json
└── mocks/               # Mock'и (будущее)
    └── mock_interface.go
```

### Что тестировать

✅ **Обязательно**:
- Публичный API
- Бизнес-логику
- Граничные условия
- Обработку ошибок
- Конкурентный доступ

❌ **Не стоит**:
- Приватные методы (через публичный API)
- Сторонние библиотеки
- Тривиальные getter/setter

### Примеры хороших тестов

См. примеры в:
- `api/handlers_test.go` - валидация и обработка ошибок
- `bot/circuit_breaker_test.go` - все состояния паттерна
- `cache/cache_test.go` - конкурентность и TTL

## Инструменты

### testify

```bash
go get github.com/stretchr/testify
```

Основные пакеты:
- `assert` - assertions
- `require` - assertions с остановкой
- `mock` - mock объекты
- `suite` - test suites

### gomock (планируется)

```bash
go install github.com/golang/mock/mockgen@latest
```

### testcontainers (планируется)

```bash
go get github.com/testcontainers/testcontainers-go
```

## Дополнительные ресурсы

- [Testing package](https://pkg.go.dev/testing)
- [Testify documentation](https://pkg.go.dev/github.com/stretchr/testify)
- [Table driven tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Advanced Go Testing](https://www.youtube.com/watch?v=8hQG7QlcLBk)
