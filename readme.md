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

Команды:

1. /start - Подписаться на получение новостей
2. /add - Добавить новостной сайт - источник URL на RSS-ленту новостей
3. /sources - Посмотреть все источники новостей
4. /addsub <id> - Подписаться на источник новостей, <id> - идентификатор источника новостей
5. /delsub <id> - Отписаться от источника новостей, <id> - идентификатор источника новостей
6. /news - Получить последние 10 новостей из всех источников

Пример команд на добавление RSS-лент:

1. /add https://lenta.ru/rss/google-newsstand/main/
2. /add https://ria.ru/export/rss2/index.xml?page_type=google_newsstand
3. /add https://rssexport.rbc.ru/rbcnews/news/30/full.rss
4. /add https://tass.ru/rss/v2.xml
5. /add http://government.ru/all/rss/
