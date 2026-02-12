# Руководство по устранению неполадок

Систематическое руководство по диагностике и решению распространенных проблем.

## Оглавление

- [Быстрая диагностика](#быстрая-диагностика)
- [Проблемы с запуском](#проблемы-с-запуском)
- [Проблемы с ботом](#проблемы-с-ботом)
- [Проблемы с базой данных](#проблемы-с-базой-данных)
- [Проблемы с Redis](#проблемы-с-redis)
- [Проблемы с производительностью](#проблемы-с-производительностью)
- [Проблемы с RSS](#проблемы-с-rss)
- [Проблемы с Docker](#проблемы-с-docker)
- [Логирование и мониторинг](#логирование-и-мониторинг)

## Быстрая диагностика

### Проверочный список

```bash
# 1. Проверка статуса контейнеров
docker-compose ps

# 2. Health check
curl http://localhost:8080/health

# 3. Логи последних ошибок
docker-compose logs --tail=50 news-bot | grep -i error

# 4. Метрики
curl http://localhost:8080/metrics | grep error

# 5. Подключение к БД
docker exec -it tg-rss-db psql -U postgres -d news_bot -c "SELECT COUNT(*) FROM news;"

# 6. Проверка Redis
docker exec -it tg-rss-redis redis-cli ping
```

### Системные требования

```bash
# Проверка доступной памяти
free -h

# Проверка места на диске
df -h

# Проверка CPU
top -bn1 | grep "Cpu(s)"

# Проверка открытых файлов
lsof -p $(pgrep -f news-bot)
```

## Проблемы с запуском

### Контейнеры не запускаются

**Симптомы**:
- `docker-compose up` завершается с ошибкой
- Контейнеры в статусе `Exited`

**Диагностика**:
```bash
# Проверить логи всех сервисов
docker-compose logs

# Проверить конкретный контейнер
docker-compose logs news-bot
docker-compose logs tg-rss-db
```

**Решения**:

1. **Порты заняты**
   ```bash
   # Проверить занятые порты
   netstat -tulpn | grep :8080
   netstat -tulpn | grep :5432
   
   # Освободить порт или изменить в docker-compose.yml
   # Убить процесс
   sudo kill -9 <PID>
   ```

2. **Недостаточно памяти**
   ```bash
   # Проверить память
   free -h
   
   # Увеличить swap или оптимизировать настройки
   sudo fallocate -l 2G /swapfile
   sudo chmod 600 /swapfile
   sudo mkswap /swapfile
   sudo swapon /swapfile
   ```

3. **Docker daemon не запущен**
   ```bash
   # Запустить Docker
   sudo systemctl start docker
   sudo systemctl enable docker
   ```

### Ошибка "Cannot connect to Docker daemon"

**Решение**:
```bash
# Добавить пользователя в группу docker
sudo usermod -aG docker $USER

# Перелогиниться или выполнить
newgrp docker

# Проверить статус Docker
sudo systemctl status docker
```

### Ошибки при сборке образа

**Симптомы**:
- `docker-compose build` завершается с ошибкой
- Ошибки загрузки зависимостей

**Решения**:

1. **Очистить кэш Docker**
   ```bash
   docker-compose build --no-cache
   docker system prune -a
   ```

2. **Проблемы с сетью**
   ```bash
   # Проверить DNS
   cat /etc/resolv.conf
   
   # Временно использовать Google DNS
   echo "nameserver 8.8.8.8" | sudo tee /etc/resolv.conf
   ```

## Проблемы с ботом

### Бот не отвечает на команды

**Диагностика**:
```bash
# Проверить логи бота
docker-compose logs -f news-bot

# Проверить токен
docker-compose exec news-bot env | grep TELEGRAM_API_KEY

# Проверить подключение к Telegram API
curl -X GET "https://api.telegram.org/bot<YOUR_TOKEN>/getMe"
```

**Решения**:

1. **Неверный токен**
   - Проверить токен в `.env`
   - Получить новый токен от @BotFather
   - Перезапустить контейнер:
     ```bash
     docker-compose restart news-bot
     ```

2. **Бот заблокирован пользователем**
   - Разблокировать бота в Telegram
   - Отправить /start заново

3. **Circuit breaker открыт**
   ```bash
   # Проверить метрики circuit breaker
   curl http://localhost:8080/metrics | grep circuit_breaker
   
   # Подождать timeout восстановления (30-120 сек)
   ```

### Новости не приходят

**Диагностика**:
```bash
# Проверить последний опрос RSS
docker-compose logs news-bot | grep "RSS"

# Проверить подписки в БД
docker exec -it tg-rss-db psql -U postgres -d news_bot -c "SELECT * FROM subscriptions;"

# Проверить активные источники
docker exec -it tg-rss-db psql -U postgres -d news_bot -c "SELECT * FROM sources WHERE status='active';"

# Проверить последние новости
docker exec -it tg-rss-db psql -U postgres -d news_bot -c "SELECT title, published_at FROM news ORDER BY published_at DESC LIMIT 10;"
```

**Решения**:

1. **Нет активных подписок**
   ```bash
   # Подписаться через бота
   # Отправить команду /subscribe
   ```

2. **RSS источники недоступны**
   ```bash
   # Проверить доступность источника
   curl -I https://example.com/feed.xml
   
   # Проверить статус источников
   curl http://localhost:8080/api/v1/sources
   ```

3. **Новости уже были отправлены**
   - Это нормально - дубликаты не отправляются
   - Проверить таблицу messages

### Rate limiting ошибки

**Симптомы**:
```
Error: Too Many Requests: retry after X seconds
```

**Решения**:

1. **Telegram API rate limit**
   - Бот автоматически обрабатывает через адаптивный rate limiter
   - Подождите указанное время
   - Проверьте метрики:
     ```bash
     curl http://localhost:8080/metrics | grep telegram
     ```

2. **API rate limit**
   ```bash
   # Проверить количество запросов
   curl http://localhost:8080/metrics | grep http_requests
   
   # Настроить лимиты в main.go
   # apiRateLimiter := middleware.NewAPIRateLimiter(100, 1*time.Minute)
   ```

## Проблемы с базой данных

### Не удается подключиться к БД

**Диагностика**:
```bash
# Проверить контейнер БД
docker-compose ps tg-rss-db

# Проверить логи
docker-compose logs tg-rss-db

# Попробовать подключиться вручную
docker exec -it tg-rss-db psql -U postgres -d news_bot
```

**Решения**:

1. **БД не запущена**
   ```bash
   docker-compose up -d tg-rss-db
   docker-compose logs -f tg-rss-db
   ```

2. **Неверные credentials**
   - Проверить `.env` файл
   - Убедиться, что `POSTGRES_PASSWORD` установлен
   - Пересоздать контейнер БД (ВНИМАНИЕ: потеря данных!):
     ```bash
     docker-compose down -v
     docker-compose up -d
     ```

3. **Connection pool исчерпан**
   ```bash
   # Проверить метрики подключений
   curl http://localhost:8080/metrics | grep db_connections
   
   # Увеличить MaxOpenConns в db/db.go
   ```

### Медленные запросы

**Диагностика**:
```bash
# Включить логирование медленных запросов в PostgreSQL
docker exec -it tg-rss-db psql -U postgres -d news_bot -c \
  "ALTER SYSTEM SET log_min_duration_statement = 1000;"

# Перезагрузить PostgreSQL
docker-compose restart tg-rss-db

# Анализ таблиц
docker exec -it tg-rss-db psql -U postgres -d news_bot -c "ANALYZE;"
```

**Решения**:

1. **Отсутствуют индексы**
   ```sql
   -- Проверить существующие индексы
   SELECT * FROM pg_indexes WHERE tablename IN ('news', 'subscriptions', 'messages');
   
   -- Создать недостающие индексы (уже должны быть в InitSchema)
   ```

2. **Большой объем данных**
   ```bash
   # Очистить старые данные
   docker exec -it tg-rss-db psql -U postgres -d news_bot -c \
     "DELETE FROM news WHERE published_at < NOW() - INTERVAL '30 days';"
   
   # VACUUM для освобождения места
   docker exec -it tg-rss-db psql -U postgres -d news_bot -c "VACUUM FULL;"
   ```

### Ошибки миграций

**Симптомы**:
- Таблицы не созданы
- Отсутствуют колонки

**Решения**:
```bash
# Проверить схему
docker exec -it tg-rss-db psql -U postgres -d news_bot -c "\dt"

# Пересоздать схему (ВНИМАНИЕ: потеря данных!)
docker exec -it tg-rss-db psql -U postgres -d news_bot -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Перезапустить бота для применения InitSchema
docker-compose restart news-bot
```

## Проблемы с Redis

### Redis недоступен

**Диагностика**:
```bash
# Проверить статус
docker-compose ps tg-rss-redis

# Проверить подключение
docker exec -it tg-rss-redis redis-cli ping

# Проверить логи
docker-compose logs tg-rss-redis
```

**Решения**:

1. **Redis не запущен**
   ```bash
   docker-compose up -d tg-rss-redis
   ```

2. **Graceful degradation**
   - Бот автоматически переходит в режим без Redis
   - Проверить логи:
     ```bash
     docker-compose logs news-bot | grep "graceful degradation"
     ```

### Память Redis заполнена

**Диагностика**:
```bash
# Проверить использование памяти
docker exec -it tg-rss-redis redis-cli INFO memory

# Проверить количество ключей
docker exec -it tg-rss-redis redis-cli DBSIZE
```

**Решения**:
```bash
# Очистить кэш (безопасно - с TTL восстановится)
docker exec -it tg-rss-redis redis-cli FLUSHDB

# Или увеличить maxmemory в docker-compose.yml
# redis:
#   command: redis-server --maxmemory 256mb
```

## Проблемы с производительностью

### Высокое использование CPU

**Диагностика**:
```bash
# Проверить CPU
docker stats

# Профилирование Go приложения
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

**Решения**:

1. **Слишком частый опрос RSS**
   ```bash
   # Увеличить TIMEOUT в .env
   TIMEOUT=120  # вместо 60
   
   docker-compose restart news-bot
   ```

2. **Много горутин**
   ```bash
   # Проверить метрики
   curl http://localhost:8080/metrics | grep go_goroutines
   
   # Уменьшить параллелизм
   CONTENT_SCRAPER_CONCURRENT=2  # вместо 3
   ```

### Высокое использование памяти

**Диагностика**:
```bash
# Профилирование памяти
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Проверить метрики
curl http://localhost:8080/metrics | grep go_memstats
```

**Решения**:

1. **Утечки памяти**
   - Проверить логи на предмет роста pendingNews
   - Периодическая очистка уже реализована

2. **Ограничить память Docker**
   ```yaml
   # В docker-compose.yml
   services:
     news-bot:
       deploy:
         resources:
           limits:
             memory: 512M
   ```

## Проблемы с RSS

### RSS источник недоступен

**Диагностика**:
```bash
# Проверить доступность
curl -I https://example.com/feed.xml

# Проверить circuit breaker
curl http://localhost:8080/metrics | grep circuit_breaker_calls
```

**Решения**:

1. **Источник временно недоступен**
   - Circuit breaker автоматически пропустит источник
   - Проверится снова через recovery timeout (30-60 сек)

2. **Изменился URL источника**
   ```bash
   # Обновить через API или БД
   curl -X PUT "http://localhost:8080/api/v1/sources/update?id=1" \
     -H "Content-Type: application/json" \
     -d '{"url": "https://new-url.com/feed.xml"}'
   ```

### Невалидный RSS

**Симптомы**:
```
Error parsing RSS: XML syntax error
```

**Решения**:

1. **Проверить формат RSS**
   ```bash
   curl https://example.com/feed.xml | xmllint --format -
   ```

2. **Деактивировать проблемный источник**
   ```bash
   docker exec -it tg-rss-db psql -U postgres -d news_bot -c \
     "UPDATE sources SET status='inactive' WHERE id=X;"
   ```

## Проблемы с Docker

### No space left on device (деплой / извлечение образов)

**Симптомы**:
- В CI/CD job "Deploy to Server": `failed to extract layer ... to overlayfs ... no space left on device`
- При запуске контейнеров: `failed to create prepare snapshot dir: mkdir ... no space left on device`
- Контейнеры redis, db или bot не создаются

**Причина**: на сервере закончилось место на диске (образы, overlayfs, логи).

**Диагностика на сервере**:
```bash
# Место на диске
df -h

# Что занимает место в Docker
docker system df
docker system df -v
```

**Решения**:

1. **Срочно освободить место (на сервере по SSH)**:
   ```bash
   # Остановить стек
   cd ~/news-bot && docker compose down

   # Удалить неиспользуемые образы и кэш сборки
   docker container prune -f
   docker image prune -a -f
   docker builder prune -af

   # При необходимости — неиспользуемые volumes (осторожно: можно удалить данные!)
   # docker volume prune -f

   # Проверить результат
   docker system df
   df -h
   ```

2. **Запустить деплой снова** — в workflow добавлена автоматическая очистка перед деплоем (после `down`), при повторном прогоне места должно хватить.

3. **Надолго**: увеличить диск, настроить ротацию логов (см. ниже), мониторить `df -h` и при необходимости добавить cron для `docker system prune -f`.

### Нехватка места на диске (общая)

**Диагностика**:
```bash
# Проверить использование Docker
docker system df

# Детальная информация
docker system df -v
```

**Решения**:
```bash
# Очистить неиспользуемые образы
docker image prune -a

# Очистить остановленные контейнеры
docker container prune

# Очистить неиспользуемые volumes
docker volume prune

# Полная очистка (осторожно!)
docker system prune -a --volumes
```

### Логи занимают много места

**Решения**:
```bash
# Ограничить размер логов в docker-compose.yml
services:
  news-bot:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

# Очистить текущие логи
truncate -s 0 $(docker inspect --format='{{.LogPath}}' tg-rss-news-bot-1)
```

## Логирование и мониторинг

### Просмотр логов

```bash
# Все логи
docker-compose logs

# Конкретный сервис
docker-compose logs news-bot

# Следить в реальном времени
docker-compose logs -f news-bot

# Последние N строк
docker-compose logs --tail=100 news-bot

# С временными метками
docker-compose logs -t news-bot

# Фильтровать по уровню
docker-compose logs news-bot | grep ERROR
docker-compose logs news-bot | grep WARN
```

### Изменение уровня логирования

```bash
# В .env файле
LOG_LEVEL=DEBUG  # DEBUG, INFO, WARN, ERROR

# Перезапустить
docker-compose restart news-bot
```

### Экспорт логов

```bash
# Сохранить логи в файл
docker-compose logs > logs_$(date +%Y%m%d_%H%M%S).txt

# За определенный период
docker-compose logs --since="2024-01-01" --until="2024-01-31" > january_logs.txt
```

## Получение помощи

Если проблема не решена:

1. **Соберите диагностическую информацию**:
   ```bash
   # Создать отчет
   ./scripts/diagnostic-report.sh > diagnostic.txt
   ```

2. **Создайте Issue на GitHub**:
   - URL: https://github.com/yourusername/tg-rss/issues
   - Приложите diagnostic.txt
   - Укажите версию (git commit hash)

3. **Проверьте FAQ**: [FAQ.md](FAQ.md)

4. **Свяжитесь с поддержкой**: support@example.com
