# История изменений деплоя

## Добавлено для деплоя

### Docker файлы
- ✅ `Dockerfile` - multi-stage build для оптимизации размера образа
- ✅ `.dockerignore` - исключение ненужных файлов из образа
- ✅ `docker-entrypoint.sh` - скрипт запуска с автоматическим расписанием

### Интеграция в docker-compose
- ✅ Добавлен сервис `news-analyzer` в `docker-compose.yml`
- ✅ Настроены зависимости от БД
- ✅ Настроены volumes для персистентности данных
- ✅ Добавлен healthcheck
- ✅ Настроены ограничения ресурсов

### Автоматизация
- ✅ Автоматический запуск по расписанию (по умолчанию 00:00)
- ✅ Поддержка переменных окружения для настройки
- ✅ Простой цикл проверки времени (работает без root)

### Документация
- ✅ `DEPLOYMENT.md` - подробная документация по деплою
- ✅ `README_DEPLOY.md` - краткая инструкция
- ✅ Обновлен `env.example` с переменными для analyzer

### Makefile
- ✅ Добавлены команды для управления analyzer:
  - `make analyzer-logs` - просмотр логов
  - `make analyzer-console` - консоль в контейнере
  - `make analyzer-run` - ручной запуск анализа
  - `make analyzer-test` - тест подключения
  - `make analyzer-build` - пересборка образа
  - `make analyzer-restart` - перезапуск
  - `make analyzer-stop/start` - остановка/запуск

## Использование

### Быстрый старт
```bash
# Запустить все сервисы (включая analyzer)
make up

# Проверить работу analyzer
make analyzer-logs
make analyzer-test
```

### Настройка расписания
Добавьте в `.env` (в корне проекта):
```env
ANALYZER_CRON_SCHEDULE=0 12 * * *  # Запуск в 12:00
```

### Ручной запуск
```bash
make analyzer-run
```

## Структура

```
news-analyzer-python/
├── Dockerfile              # Образ контейнера
├── .dockerignore           # Исключения для Docker
├── docker-entrypoint.sh    # Скрипт запуска
├── DEPLOYMENT.md          # Подробная документация
├── README_DEPLOY.md       # Краткая инструкция
└── ...                    # Остальные файлы проекта
```

## Volumes

Созданы два volume для персистентности:
- `analyzer_reports` - JSON отчеты
- `analyzer_logs` - логи работы

## Переменные окружения

Доступны через docker-compose:
- `ANALYZER_CRON_SCHEDULE` - расписание (по умолчанию: `0 0 * * *`)
- `RUN_ON_STARTUP` - запуск при старте (по умолчанию: `false`)
- `ANALYZER_LOG_LEVEL` - уровень логирования (по умолчанию: `INFO`)

Все переменные БД наследуются из основного `.env` файла.
