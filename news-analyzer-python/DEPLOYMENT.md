# Деплой News Analyzer

## Обзор

Python-сервис `news-analyzer` интегрирован в существующий docker-compose проект и автоматически запускается вместе с остальными сервисами.

## Быстрый старт

### 1. Подготовка конфигурации

Убедитесь, что в корне проекта есть файл `.env` с настройками БД:
```env
POSTGRES_HOST=db
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=news_bot
TZ=Europe/Moscow
```

### 2. Запуск всех сервисов

```bash
make up
# или
docker-compose --env-file .env up -d
```

Это запустит:
- `bot` - Go-приложение
- `db` - PostgreSQL
- `kafka` - Kafka брокер
- `zookeeper` - Zookeeper
- `news-analyzer` - Python-сервис анализа новостей

### 3. Проверка работы

```bash
# Проверить логи
make analyzer-logs

# Проверить подключение к БД
make analyzer-test

# Запустить анализ вручную
make analyzer-run
```

## Конфигурация

### Переменные окружения

В `docker-compose.yml` можно настроить следующие переменные для `news-analyzer`:

```yaml
environment:
  # Расписание запуска (формат: "HH:MM,HH:MM", по умолчанию: 12:00 и 19:00)
  ANALYZER_SCHEDULE_TIMES: "12:00,19:00"
  
  # Запустить анализ сразу при старте (для тестирования)
  RUN_ON_STARTUP: "false"
  
  # Уровень логирования
  ANALYZER_LOG_LEVEL: "INFO"
  
  # Telegram для отправки отчетов
  TELEGRAM_API_KEY: "${TELEGRAM_API_KEY}"  # Тот же токен, что и для основного бота
  TELEGRAM_CHAT_ID: "234501916"  # ID чата для отправки (по умолчанию AdminChatID)
```

### Расписание запуска

Формат расписания: `"HH:MM,HH:MM"` (несколько времен через запятую)

**По умолчанию:** два запуска в день в **12:00 и 19:00**

Примеры:
- `"12:00,19:00"` - два раза в день (12:00 и 19:00) - **по умолчанию**
- `"08:00,14:00,20:00"` - три раза в день
- `"00:00"` - один раз в день в полночь
- `"09:30,18:30"` - два раза в день в 09:30 и 18:30

### Интеграция с Telegram

После каждого анализа отчет автоматически отправляется в Telegram бот.

**Настройка:**
1. Убедитесь, что `TELEGRAM_API_KEY` установлен (используется тот же токен, что и для основного бота)
2. Установите `TELEGRAM_CHAT_ID` - ID чата для отправки (можно получить через @userinfobot)
3. По умолчанию используется AdminChatID из бота (234501916)

**Формат отчета:**
- Заголовок с датой и временем анализа
- Общая статистика (количество новостей, тем)
- Топ-5 тем с ключевыми словами и примерами заголовков
- Форматирование в Markdown для удобного чтения

### Конфигурационный файл

По умолчанию используется `config.yaml.example`. Для кастомизации:

1. Создайте `news-analyzer-python/config.yaml` на основе `config.yaml.example`
2. Настройте параметры анализа
3. Пересоберите образ: `make analyzer-build`

Или смонтируйте файл как volume в `docker-compose.yml`:
```yaml
volumes:
  - ./news-analyzer-python/config.yaml:/app/config.yaml:ro
```

## Управление сервисом

### Команды Makefile

```bash
# Логи
make analyzer-logs

# Консоль в контейнере
make analyzer-console

# Запустить анализ вручную
make analyzer-run

# Тест подключения к БД
make analyzer-test

# Пересобрать образ
make analyzer-build

# Перезапустить сервис
make analyzer-restart

# Остановить сервис
make analyzer-stop

# Запустить сервис
make analyzer-start
```

### Docker команды

```bash
# Логи
docker-compose logs -f news-analyzer

# Выполнить команду в контейнере
docker exec -it news-analyzer python run_daily.py
docker exec -it news-analyzer python test_connection.py

# Консоль
docker exec -it news-analyzer bash

# Пересобрать
docker-compose build news-analyzer

# Перезапустить
docker-compose restart news-analyzer
```

## Автоматический запуск

Сервис автоматически запускает анализ по расписанию, указанному в `ANALYZER_CRON_SCHEDULE`.

Механизм работы:
1. При старте контейнера запускается `docker-entrypoint.sh`
2. Скрипт парсит расписание и запускает цикл проверки времени
3. Когда наступает время запуска, выполняется `run_daily.py`
4. Результаты сохраняются в `storage/reports/`

## Персистентность данных

Отчеты и логи сохраняются в Docker volumes:
- `analyzer_reports` - JSON отчеты
- `analyzer_logs` - логи работы

Для доступа к данным:
```bash
# Найти volume
docker volume ls | grep analyzer

# Просмотреть содержимое (требуется root)
docker run --rm -v analyzer_reports:/data alpine ls -la /data
```

## Мониторинг

### Healthcheck

Контейнер имеет healthcheck, который проверяет подключение к БД:
```bash
docker inspect news-analyzer | grep -A 10 Health
```

### Логи

Логи доступны через:
```bash
make analyzer-logs
# или
docker-compose logs -f news-analyzer
```

Логи также сохраняются в:
- Консоль (stdout/stderr)
- `storage/logs/news_analyzer.log` (внутри контейнера)
- `storage/logs/cron.log` (если используется cron)

## Отладка

### Проблемы с подключением к БД

```bash
# Проверить подключение
make analyzer-test

# Проверить переменные окружения
docker exec -it news-analyzer env | grep POSTGRES
```

### Проблемы с NLTK данными

```bash
# Переустановить NLTK данные
docker exec -it news-analyzer python setup_nltk.py
```

### Проблемы с конфигурацией

```bash
# Проверить наличие config.yaml
docker exec -it news-analyzer ls -la config.yaml

# Просмотреть конфигурацию
docker exec -it news-analyzer cat config.yaml
```

### Запуск в режиме отладки

```bash
# Запустить с выводом всех логов
docker-compose up news-analyzer

# Или установить RUN_ON_STARTUP=true для немедленного запуска
docker-compose up -d -e RUN_ON_STARTUP=true news-analyzer
```

## Обновление

```bash
# Остановить сервис
make analyzer-stop

# Пересобрать образ
make analyzer-build

# Запустить сервис
make analyzer-start
```

## Производственное развертывание

### Рекомендации

1. **Безопасность**:
   - Не храните пароли в `.env` в git
   - Используйте секреты Docker или внешние системы управления секретами
   - Ограничьте доступ к volumes с отчетами

2. **Мониторинг**:
   - Настройте алерты на ошибки в логах
   - Мониторьте использование ресурсов
   - Отслеживайте успешность выполнения анализа

3. **Резервное копирование**:
   - Регулярно сохраняйте volumes с отчетами
   - Настройте автоматическое копирование в S3 или другой storage

4. **Масштабирование**:
   - Для больших объемов данных увеличьте `mem_limit` и `cpus`
   - Рассмотрите возможность запуска нескольких экземпляров для разных временных окон

### Оптимизация ресурсов

В `docker-compose.yml` можно настроить ограничения:
```yaml
mem_limit: 512m      # Максимум памяти
mem_reservation: 256m # Резервируемая память
cpus: 0.5            # Количество CPU
```

Для больших объемов данных увеличьте эти значения.
