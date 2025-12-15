# Сохранение исторических данных анализа

## 📊 Обзор

Теперь все результаты анализа автоматически сохраняются в PostgreSQL для последующего использования как исторических данных. Это позволяет:

- 📈 Анализировать тренды во времени
- 🔍 Сравнивать результаты разных периодов
- 📉 Строить графики и статистику
- 🔄 Восстанавливать данные при потере файлов

## 🗄️ Структура таблицы

Таблица `news_analysis` содержит следующие поля:

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | SERIAL PRIMARY KEY | Уникальный идентификатор |
| `analysis_date` | TIMESTAMP | Дата и время анализа (UNIQUE) |
| `total_news` | INTEGER | Количество проанализированных новостей |
| `narratives_count` | INTEGER | Количество найденных тем |
| `narratives` | JSONB | Полные данные нарративов в формате JSON |
| `created_at` | TIMESTAMP | Время создания записи в БД |

### Индексы

- `idx_news_analysis_date` - для быстрого поиска по дате анализа
- `idx_news_analysis_created_at` - для сортировки по времени создания

## 🔧 Автоматическое создание таблицы

Таблица создается автоматически при первом запуске анализа, если её еще нет в БД. Метод `ensure_analysis_table_exists()` проверяет существование таблицы и создает её при необходимости.

## 💾 Сохранение данных

Данные сохраняются автоматически после каждого анализа в `run_daily.py`:

```python
# Сохраняем результат анализа в БД для исторических данных
db.ensure_analysis_table_exists()
analysis_id = db.save_analysis_result(
    analysis_date=analysis_date,
    total_news=len(news_items),
    narratives=narratives
)
```

### Особенности сохранения

- **UPSERT логика**: Если анализ с такой же датой уже существует, запись обновляется
- **JSONB формат**: Нарративы сохраняются в формате JSONB для эффективного хранения и запросов
- **Автоматическая транзакция**: Все операции выполняются в транзакции с rollback при ошибке

## 📖 Получение исторических данных

### Через Python API

```python
from src.db import Database
from src.config import load_settings
from datetime import datetime, timedelta

settings = load_settings()
db = Database(settings.get_db_connection_string())
db.connect()

# Получить последние 10 результатов
results = db.get_analysis_results(limit=10)

# Получить результаты за период
start_date = datetime(2025, 12, 1)
end_date = datetime(2025, 12, 15)
results = db.get_analysis_results(start_date=start_date, end_date=end_date)

# Получить последний результат
latest = db.get_latest_analysis_result()
```

### Через скрипт

```bash
# Последние 10 результатов
docker exec -it news-analyzer python scripts/get_analysis_history.py

# Последние 20 результатов
docker exec -it news-analyzer python scripts/get_analysis_history.py --limit 20

# Результаты с определенной даты
docker exec -it news-analyzer python scripts/get_analysis_history.py --start 2025-12-01

# Результаты за период
docker exec -it news-analyzer python scripts/get_analysis_history.py --start 2025-12-01 --end 2025-12-15

# Вывод в формате JSON
docker exec -it news-analyzer python scripts/get_analysis_history.py --json
```

### Через SQL

```sql
-- Все результаты
SELECT * FROM news_analysis ORDER BY analysis_date DESC;

-- Результаты за последние 7 дней
SELECT * FROM news_analysis 
WHERE analysis_date >= NOW() - INTERVAL '7 days'
ORDER BY analysis_date DESC;

-- Статистика по дням
SELECT 
    DATE(analysis_date) as date,
    COUNT(*) as analyses_count,
    AVG(total_news) as avg_news,
    AVG(narratives_count) as avg_narratives
FROM news_analysis
GROUP BY DATE(analysis_date)
ORDER BY date DESC;

-- Поиск по ключевым словам в нарративах
SELECT * FROM news_analysis
WHERE narratives::text LIKE '%политика%'
ORDER BY analysis_date DESC;
```

## 📊 Примеры использования

### Анализ трендов

```python
# Получить все результаты за месяц
from datetime import datetime, timedelta

start_date = datetime.now() - timedelta(days=30)
results = db.get_analysis_results(start_date=start_date)

# Анализ изменения количества новостей
news_counts = [r.total_news for r in results]
avg_news = sum(news_counts) / len(news_counts) if news_counts else 0
print(f"Среднее количество новостей за месяц: {avg_news:.1f}")
```

### Сравнение периодов

```python
# Результаты за первую неделю декабря
week1_start = datetime(2025, 12, 1)
week1_end = datetime(2025, 12, 7)
week1_results = db.get_analysis_results(start_date=week1_start, end_date=week1_end)

# Результаты за вторую неделю декабря
week2_start = datetime(2025, 12, 8)
week2_end = datetime(2025, 12, 14)
week2_results = db.get_analysis_results(start_date=week2_start, end_date=week2_end)

# Сравнение
week1_avg_news = sum(r.total_news for r in week1_results) / len(week1_results)
week2_avg_news = sum(r.total_news for r in week2_results) / len(week2_results)
print(f"Неделя 1: {week1_avg_news:.1f} новостей в среднем")
print(f"Неделя 2: {week2_avg_news:.1f} новостей в среднем")
```

### Поиск тем по ключевым словам

```python
# Получить все результаты
all_results = db.get_analysis_results()

# Найти все упоминания определенной темы
keyword = "экономика"
for result in all_results:
    for narrative in result.narratives:
        if keyword.lower() in ' '.join(narrative.get('keywords', [])).lower():
            print(f"Найдено в анализе от {result.analysis_date}: {narrative}")
```

## 🔍 Миграция

Если таблица еще не создана, можно выполнить миграцию вручную:

```bash
# Через psql
psql -h localhost -U postgres -d news_bot -f db/migrations/create_news_analysis_table.sql

# Или через docker
docker exec -i db psql -U postgres -d news_bot < news-analyzer-python/db/migrations/create_news_analysis_table.sql
```

## ⚠️ Важные замечания

1. **Уникальность по дате**: Каждая дата анализа может иметь только одну запись. При повторном анализе в тот же день запись обновляется.

2. **Размер данных**: JSONB поля могут занимать значительное место. Рекомендуется периодически очищать старые данные (старше года).

3. **Производительность**: Индексы оптимизируют запросы по дате. Для сложных запросов по JSONB используйте GIN индексы.

4. **Резервное копирование**: Данные в БД автоматически включаются в бэкапы PostgreSQL.

## 🧹 Очистка старых данных

```sql
-- Удалить результаты старше года
DELETE FROM news_analysis 
WHERE analysis_date < NOW() - INTERVAL '1 year';

-- Или оставить только последние 100 записей
DELETE FROM news_analysis
WHERE id NOT IN (
    SELECT id FROM news_analysis 
    ORDER BY analysis_date DESC 
    LIMIT 100
);
```

## 📈 Мониторинг

```sql
-- Размер таблицы
SELECT 
    pg_size_pretty(pg_total_relation_size('news_analysis')) as total_size,
    pg_size_pretty(pg_relation_size('news_analysis')) as table_size,
    pg_size_pretty(pg_indexes_size('news_analysis')) as indexes_size;

-- Количество записей
SELECT COUNT(*) FROM news_analysis;

-- Статистика по датам
SELECT 
    DATE(analysis_date) as date,
    COUNT(*) as count
FROM news_analysis
GROUP BY DATE(analysis_date)
ORDER BY date DESC;
```
