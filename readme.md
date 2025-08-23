[![CI/CD Pipeline](https://github.com/kaevdokimov/tg-rss/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/kaevdokimov/tg-rss/actions/workflows/ci-cd.yml)

### Запуск

build:

```
    docker-compose build
```

up:

```
    docker-compose up
```

Теперь Telegram-бот готов к использованию.

## 🎯 Возможности

Бот поддерживает как текстовые команды, так и удобные inline-кнопки для навигации.

### 📱 Удобная навигация с кнопками

После команды `/start` бот показывает главное меню с кнопками:
- 📰 **Последние новости** - получить последние 10 новостей
- 📋 **Мои источники** - просмотреть все доступные источники
- ➕ **Добавить источник** - добавить новый RSS-источник
- 📝 **Мои подписки** - управление подписками
- ❓ **Помощь** - справка по командам

### ⌨️ Текстовые команды

1. `/start` - Подписаться на получение новостей
2. `/add <URL>` - Добавить новостной сайт - источник URL на RSS-ленту новостей
3. `/sources` - Посмотреть все источники новостей
4. `/addsub <id>` - Подписаться на источник новостей, `<id>` - идентификатор источника новостей
5. `/delsub <id>` - Отписаться от источника новостей, `<id>` - идентификатор источника новостей
6. `/news` - Получить последние 10 новостей из всех источников
7. `/help` - Показать справку по командам

Пример команд на добавление RSS-лент:

1. /add https://lenta.ru/rss/google-newsstand/main/
2. /add https://ria.ru/export/rss2/index.xml?page_type=google_newsstand
3. /add https://rssexport.rbc.ru/rbcnews/news/30/full.rss
4. /add https://tass.ru/rss/v2.xml
5. /add http://government.ru/all/rss/
