# 📰 TG-RSS - Telegram RSS Reader Bot

[![CI/CD Pipeline](https://github.com/kaevdokimov/tg-rss/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/kaevdokimov/tg-rss/actions/workflows/ci-cd.yml)

Telegram-бот для чтения RSS-лент с удобным интерфейсом и автоматическим обновлением новостей.

## 🚀 Возможности

- 📰 **Чтение RSS-лент** - поддержка популярных новостных источников
- 🔔 **Автоматические уведомления** - получение свежих новостей
- 📱 **Удобный интерфейс** - inline-кнопки для навигации
- 📋 **Управление подписками** - добавление/удаление источников
- 🗄️ **PostgreSQL** - надежное хранение данных
- 🐳 **Docker** - простое развертывание

## 🛠️ Технологии

- **Go 1.25** - основной язык разработки
- **PostgreSQL 17.6** - база данных
- **Telegram Bot API** - интеграция с Telegram
- **Docker & Docker Compose** - контейнеризация
- **gofeed** - парсинг RSS-лент

## 📋 Требования

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
├── bot/           # Telegram бот логика
├── config/        # Конфигурация приложения
├── db/           # Работа с базой данных
├── rss/          # RSS парсинг
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── main.go
└── Makefile
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
