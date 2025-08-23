# Настройка Redpanda для tg-rss

## Описание

Проект был модифицирован для использования Redpanda (совместимого с Kafka) для отправки новостей только подписанным пользователям через очередь сообщений.

## Основные изменения

1. **RSS Poller**: Теперь отправляет новости в Redpanda вместо прямой отправки всем пользователям
2. **Message Processor**: Новый компонент для обработки сообщений из Redpanda
3. **Конфигурация**: Добавлены настройки для Redpanda

## Переменные окружения

Добавьте следующие переменные в ваш `.env` файл:

```env
# Redpanda настройки
REDPANDA_BROKERS=localhost:9092
REDPANDA_NEWS_TOPIC=news-items
REDPANDA_NOTIFY_TOPIC=news-notifications
```

## Запуск с Docker Compose

1. Убедитесь, что у вас есть `.env` файл с необходимыми переменными
2. Запустите все сервисы:

```bash
docker-compose up -d
```

Это запустит:
- PostgreSQL базу данных
- Redpanda (совместимый с Kafka)
- Telegram бота

## Локальная разработка

Для локальной разработки:

1. Запустите только Redpanda и PostgreSQL:
```bash
docker-compose up -d redpanda db
```

2. Запустите бота локально:
```bash
go run main.go
```

## Архитектура

```
RSS Sources → RSS Poller → Redpanda (news-items) → News Processor → PostgreSQL + Telegram Users
```

1. **RSS Poller** парсит RSS источники и отправляет новости в Redpanda (топик news-items)
2. **News Processor** читает новости из очереди, записывает в БД и отправляет подписанным пользователям
3. Каждое сообщение содержит информацию о новости

### Топики Redpanda:
- **news-items** - для новых новостей из RSS источников
- **news-notifications** - для уведомлений пользователям (резервный)

## Преимущества

- **Масштабируемость**: Можно запустить несколько экземпляров Message Processor
- **Надежность**: Сообщения сохраняются в Redpanda и не теряются при сбоях
- **Изоляция**: RSS парсинг и отправка сообщений разделены
- **Фильтрация**: Отправляются только подписанным пользователям

## Мониторинг

Redpanda предоставляет веб-интерфейс на порту 8082:
- URL: http://localhost:8082
- Можно просматривать топики, сообщения и метрики

## Отладка

Для отладки можно использовать Redpanda CLI:

```bash
# Подключиться к Redpanda
docker exec -it news_redpanda rpk cluster info

# Просмотреть топики
docker exec -it news_redpanda rpk topic list

# Просмотреть сообщения в топике новостей
docker exec -it news_redpanda rpk topic consume news-items

# Просмотреть сообщения в топике уведомлений
docker exec -it news_redpanda rpk topic consume news-notifications
```
