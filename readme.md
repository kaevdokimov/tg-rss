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

1. /start - Подписаться на новости
2. /add - Добавить новостной сайт - источник URL на RSS-ленту новостей
3. /news5 - Получить последние 5 новостей из всех источников
4. /news10 - Получить последние 10 новостей из всех источников

Пример команд на добавление RSS-лент:

1. /start
2. /add https://lenta.ru/rss
3. /add https://ria.ru/export/rss2/index.xml
4. /add https://rssexport.rbc.ru/rbcnews/news/30/full.rss