# Быстрый старт

## Шаг 1: Установка зависимостей

```bash
# Создайте виртуальное окружение
python3 -m venv venv
source venv/bin/activate  # Linux/Mac
# или venv\Scripts\activate  # Windows

# Установите зависимости
pip install -r requirements.txt

# Загрузите данные NLTK
python setup_nltk.py
```

## Шаг 2: Настройка конфигурации

```bash
# Скопируйте примеры конфигурации
cp env.example .env
cp config.yaml.example config.yaml
```

Отредактируйте `.env` файл, указав параметры подключения к вашей БД:
```env
POSTGRES_HOST=db          # или localhost для локальной разработки
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=news_bot
```

## Шаг 3: Тестирование подключения

```bash
python test_connection.py
```

Если все работает, вы увидите:
- ✅ Подключение к БД успешно
- ✅ Получено N новостей

## Шаг 4: Запуск анализа

```bash
python run_daily.py
```

Результаты будут сохранены в `storage/reports/report_YYYY-MM-DD.json`.

## Структура отчета

Отчет в JSON содержит:
- `analysis_date` - дата анализа
- `total_news` - общее количество новостей
- `narratives_count` - количество найденных тем
- `narratives` - массив тем, каждая содержит:
  - `cluster_id` - ID кластера
  - `size` - количество новостей в теме
  - `keywords` - ключевые слова
  - `titles` - примеры заголовков
  - `news_count` - количество новостей

## Настройка параметров

Основные параметры настраиваются в `config.yaml`:
- `time_window_hours` - окно анализа (по умолчанию 24 часа)
- `min_cluster_size` - минимальный размер кластера (по умолчанию 5)
- `top_narratives` - количество топ-тем в отчете (по умолчанию 5)

## Автоматический запуск (cron)

Для ежедневного запуска в 00:00 добавьте в crontab:
```bash
0 0 * * * cd /path/to/news-analyzer-python && /path/to/venv/bin/python run_daily.py >> /path/to/logs/cron.log 2>&1
```
