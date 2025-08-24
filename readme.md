# 📰 TG-RSS - Telegram RSS Reader Bot

[![CI/CD Pipeline](https://github.com/kaevdokimov/tg-rss/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/kaevdokimov/tg-rss/actions/workflows/ci-cd.yml)

Telegram-бот для чтения RSS-лент с использованием Redpanda в качестве брокера сообщений. Бот предоставляет удобный интерфейс для подписки на новостные источники и получения автоматических уведомлений.

## 🚀 Основные возможности

- 📰 **Чтение RSS-лент** - поддержка любых RSS-источников
- 🔔 **Автоматические уведомления** - мгновенное получение свежих новостей
- 📱 **Интерактивный интерфейс** - удобное управление через inline-кнопки
- 📋 **Гибкое управление подписками** - добавление и удаление источников
- 🚀 **Высокая производительность** - асинхронная обработка с использованием Redpanda
- 🗄️ **Надежное хранение** - PostgreSQL для хранения данных
- 🐳 **Простое развертывание** - полная поддержка Docker

## 🏗️ Архитектура

```
RSS Sources → RSS Poller → Redpanda (news-items) → News Processor → PostgreSQL + Telegram Users
```

### Компоненты системы:

1. **RSS Poller**
   - Периодически опрашивает RSS-источники
   - Отправляет новые новости в топик Redpanda `news-items`

2. **News Processor**
   - Асинхронно обрабатывает сообщения из Redpanda
   - Сохраняет новости в базу данных PostgreSQL
   - Отправляет уведомления подписанным пользователям

3. **Telegram Bot**
   - Предоставляет пользовательский интерфейс
   - Управление подписками и настройками

### Топики Redpanda:
- **news-items** - входящие новости от RSS источников
- **news-notifications** - уведомления для пользователей (резервный канал)

## 🛠️ Технологический стек

- **Go** - основной язык разработки (1.25+)
- **PostgreSQL** - реляционная база данных
- **Redpanda** - высокопроизводительный брокер сообщений (Kafka-совместимый)
- **Telegram Bot API** - интеграция с мессенджером
- **Docker & Docker Compose** - контейнеризация и оркестрация
- **gofeed** - библиотека для парсинга RSS/Atom

## 🚀 Быстрый старт

### Предварительные требования

- Docker и Docker Compose
- Go 1.25+ (только для локальной разработки)
- Токен Telegram бота от [@BotFather](https://t.me/botfather)

### Настройка окружения

1. Клонируйте репозиторий:
   ```bash
   git clone https://github.com/kaevdokimov/tg-rss.git
   cd tg-rss
   ```

2. Создайте и настройте файл `.env`:
   ```bash
   cp .env.example .env
   ```

3. Отредактируйте `.env` файл, указав ваши настройки:
   ```env
   # Telegram Bot Token (обязательно)
   TELEGRAM_API_KEY=your_telegram_bot_token_here

   # Настройки базы данных
   POSTGRES_HOST=db
   POSTGRES_PORT=5432
   POSTGRES_USER=postgres
   POSTGRES_PASSWORD=your_secure_password
   POSTGRES_DB=tg_rss

   # Настройки Redpanda
   REDPANDA_BROKERS=news_redpanda:9092
   REDPANDA_NEWS_TOPIC=news-items
   REDPANDA_NOTIFY_TOPIC=news-notifications
   ```

### Запуск с Docker Compose

Для быстрого старта всех компонентов выполните:

```bash
# Запуск всех сервисов в фоновом режиме
docker-compose up -d

# Просмотр логов в реальном времени
docker-compose logs -f
```

### Локальная разработка

1. Запустите зависимости (Redpanda и PostgreSQL):
   ```bash
   docker-compose up -d redpanda db
   ```

2. Запустите приложение локально:
   ```bash
   go run main.go
   ```

## 🔍 Мониторинг и отладка

### Redpanda Web UI
Доступен по адресу: http://localhost:8082
- Просмотр топиков и сообщений
- Мониторинг производительности
- Управление партициями и репликами

### Полезные команды для отладки

```bash
# Просмотр списка топиков
docker exec -it news_redpanda rpk topic list

# Просмотр сообщений в топике новостей
docker exec -it news_redpanda rpk topic consume news-items

# Просмотр логов бота
docker-compose logs -f bot
```

## 📈 Преимущества архитектуры

- **Масштабируемость**: Возможность горизонтального масштабирования компонентов
- **Надежность**: Сообщения сохраняются в Redpanda и не теряются при сбоях
- **Производительность**: Асинхронная обработка новостей
- **Гибкость**: Легко добавлять новые источники и обработчики

### Команды для отладки

```bash
# Проверить статус Redpanda
docker exec -it news_redpanda rpk cluster info

# Список топиков
docker exec -it news_redpanda rpk topic list

# Просмотр сообщений в топике новостей
docker exec -it news_redpanda rpk topic consume news-items

# Просмотр уведомлений
docker exec -it news_redpanda rpk topic consume news-notifications
```

## 🏗️ Архитектурные преимущества

- **Масштабируемость**: Возможность запуска нескольких экземпляров Message Processor
- **Надежность**: Сообщения сохраняются в Redpanda и не теряются при сбоях
- **Изоляция**: RSS парсинг и отправка сообщений разделены
- **Фильтрация**: Отправляются только подписанным пользователям
- **Производительность**: Асинхронная обработка новостей через брокер сообщений

## 📝 Лицензия

MIT

- Docker и Docker Compose
- Telegram Bot Token (получить у [@BotFather](https://t.me/BotFather))

## ⚙️ Настройка

### 1. Клонирование репозитория

```bash
git clone https://github.com/kaevdokimov/tg-rss.git
cd tg-rss
```

### 2. Создание файла окружения

Создайте файл `.env.local` в корне проекта:

```env
# Telegram Bot Configuration
TELEGRAM_API_KEY=your_telegram_bot_token_here

# PostgreSQL Configuration
POSTGRES_HOST=db
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_secure_password
POSTGRES_DB=news_bot

# Redpanda Configuration
REDPANDA_BROKERS=news_redpanda:9092
REDPANDA_NEWS_TOPIC=news-items
REDPANDA_NOTIFY_TOPIC=news-notifications

# Optional Settings
TZ=Europe/Moscow
TIMEOUT=60
```

### 3. Запуск приложения

#### Быстрый запуск:
```bash
make init
```

#### Пошаговый запуск:
```bash
# Сборка и запуск
docker-compose build
docker-compose up -d

# Просмотр логов
docker-compose logs -f
```

## 🏗️ Архитектура

Проект использует архитектуру с очередью сообщений для надежной доставки новостей:

```
RSS Sources → RSS Poller → Redpanda (news-items) → News Processor → PostgreSQL + Telegram Users
```

### Компоненты:
- **RSS Poller** - парсит RSS источники и отправляет новости в Redpanda (топик news-items)
- **Redpanda** - очередь сообщений (совместима с Kafka)
- **News Processor** - читает новости из очереди, записывает в БД и отправляет подписанным пользователям
- **PostgreSQL** - хранение данных о пользователях, источниках и подписках

### Топики Redpanda:
- **news-items** - для новых новостей из RSS источников
- **news-notifications** - для уведомлений пользователям (резервный)

### Преимущества:
- ✅ **Масштабируемость** - можно запустить несколько экземпляров Message Processor
- ✅ **Надежность** - сообщения сохраняются в Redpanda и не теряются при сбоях
- ✅ **Изоляция** - RSS парсинг и отправка сообщений разделены
- ✅ **Фильтрация** - отправляются только подписанным пользователям

## 🎯 Использование

### 📱 Удобная навигация с кнопками

После команды `/start` бот показывает главное меню с кнопками:
- 📰 **Последние новости** - получить последние 10 новостей
- 📋 **Мои источники** - просмотреть все доступные источники
- ➕ **Добавить источник** - добавить новый RSS-источник
- 📝 **Мои подписки** - управление подписками
- ❓ **Помощь** - справка по командам

### ⌨️ Текстовые команды

| Команда | Описание |
|---------|----------|
| `/start` | Подписаться на получение новостей |
| `/add <URL>` | Добавить RSS-источник |
| `/sources` | Посмотреть все источники новостей |
| `/addsub <id>` | Подписаться на источник новостей |
| `/delsub <id>` | Отписаться от источника новостей |
| `/news` | Получить последние 10 новостей |
| `/help` | Показать справку по командам |

### 📰 Примеры RSS-источников

```bash
# Популярные новостные источники
/add https://lenta.ru/rss/google-newsstand/main/
/add https://ria.ru/export/rss2/index.xml?page_type=google_newsstand
/add https://rssexport.rbc.ru/rbcnews/news/30/full.rss
/add https://tass.ru/rss/v2.xml
/add http://government.ru/all/rss/
```

## 🐳 Docker команды

```bash
# Запуск
make up

# Остановка
make down

# Перезапуск
make restart

# Просмотр логов
make logs

# Подключение к контейнеру
make console
```

## 📁 Структура проекта

```
tg-rss/
├── bot/              # Telegram бот логика
├── config/           # Конфигурация приложения
├── db/              # Работа с базой данных
├── rss/             # RSS парсинг
├── redpanda/        # Redpanda producer/consumer
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── main.go
├── Makefile
└── REDPANDA_SETUP.md # Документация по Redpanda
```

## 🔧 Разработка

### Локальная разработка

```bash
# Установка зависимостей
go mod download

# Запуск локально (требуется PostgreSQL)
go run main.go
```

### Тестирование

```bash
# Запуск тестов
go test ./...
```

## 📊 Мониторинг

### Redpanda Console
Redpanda предоставляет веб-интерфейс для мониторинга:
- URL: http://localhost:8082
- Возможности: просмотр топиков, сообщений, метрик

### Отладка через CLI
```bash
# Подключиться к Redpanda
docker exec -it news_redpanda rpk cluster info

# Просмотреть топики
docker exec -it news_redpanda rpk topic list

# Просмотреть сообщения в топике
docker exec -it news_redpanda rpk topic consume news-notifications
```

## 📝 Лицензия

MIT License

## 🤝 Вклад в проект

1. Fork репозитория
2. Создайте feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменения (`git commit -m 'Add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Откройте Pull Request

## 📞 Поддержка

Если у вас есть вопросы или предложения, создайте [Issue](https://github.com/kaevdokimov/tg-rss/issues) в репозитории.
