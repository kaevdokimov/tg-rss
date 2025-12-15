# Финальная оптимизация: очистка Python мусора и перенос сборки

## ✅ Выполненные задачи

### 1. Улучшена очистка Python мусора

#### Builder stage (строки 18-26):
```dockerfile
# Очищаем временные файлы сборки
find /root/.local -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
find /root/.local -name "*.pyc" -delete 2>/dev/null || true
find /root/.local -name "*.pyo" -delete 2>/dev/null || true
# Очищаем .dist-info и .egg-info (оставляем только необходимое)
find /root/.local -type d -name "*.dist-info" -exec rm -rf {} + 2>/dev/null || true
find /root/.local -type d -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true
```

#### Runtime stage (строки 75-82):
```dockerfile
# Очищаем Python кэш и временные файлы
find /home/appuser/.local -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
find /home/appuser/.local -name "*.pyc" -delete 2>/dev/null || true
find /home/appuser/.local -name "*.pyo" -delete 2>/dev/null || true
find /home/appuser/.local -name "*.pyd" -delete 2>/dev/null || true
# Очищаем документацию пакетов (не нужна в runtime)
find /home/appuser/.local -type d -name "*.dist-info" -exec sh -c 'rm -rf "$1"/RECORD "$1"/INSTALLER "$1"/REQUESTED 2>/dev/null || true' _ {} \; || true
# Очищаем тесты из установленных пакетов
find /home/appuser/.local -type d -name "tests" -exec rm -rf {} + 2>/dev/null || true
find /home/appuser/.local -type d -name "test" -exec rm -rf {} + 2>/dev/null || true
find /home/appuser/.local -type d -name "*.tests" -exec rm -rf {} + 2>/dev/null || true
```

### 2. Перенос сборки в GitHub Actions

#### Добавлен новый job `docker-build-analyzer`:
- ✅ Собирает образ в GitHub Actions runners
- ✅ Публикует в GitHub Container Registry
- ✅ Теги: `latest` и `{sha}`
- ✅ Использует кэширование для ускорения

#### Изменен процесс деплоя:
- ✅ Убрана локальная сборка на сервере
- ✅ Убрано копирование `news-analyzer-python/`
- ✅ Только pull образа из registry
- ✅ Быстрее деплой

#### Обновлен docker-compose.yml:
- ✅ Изменено с `build:` на `image:`
- ✅ Образ: `ghcr.io/${GITHUB_REPOSITORY_OWNER}/news-analyzer:latest`

## Преимущества

### Производительность:
- ⚡ **Быстрее деплой** - не нужно собирать на сервере (экономия 5-10 минут)
- ⚡ **Меньше нагрузка** на сервер (не нужны компиляторы)
- ⚡ **Единая точка сборки** - все образы собираются в GitHub Actions

### Размер образа:
- 📦 **Меньше размер** за счет удаления:
  - Тестов из пакетов (~50-100 MB)
  - Документации пакетов (~20-30 MB)
  - Кэшей Python (~10-20 MB)
  - Временных файлов (~5-10 MB)
- 📦 **Ожидаемое уменьшение**: ~100-150 MB

### Надежность:
- ✅ Единая сборка для всех окружений
- ✅ Версионирование через SHA коммита
- ✅ Кэширование для ускорения сборки

## Процесс сборки и деплоя

### 1. GitHub Actions собирает образы:
```
build → docker-build → docker-build-analyzer → deploy
```

### 2. Образы публикуются:
```
ghcr.io/{owner}/news-bot:latest
ghcr.io/{owner}/news-bot:{sha}
ghcr.io/{owner}/news-analyzer:latest
ghcr.io/{owner}/news-analyzer:{sha}
```

### 3. Деплой на сервер:
```bash
# Pull образов
docker pull ghcr.io/{owner}/news-bot:latest
docker pull ghcr.io/{owner}/news-analyzer:latest

# Запуск
docker compose up -d

# Очистка
docker system prune -af
```

## Проверка

### Размер образа:
```bash
docker images | grep news-analyzer
# Должен быть меньше после оптимизации
```

### Отсутствие мусора:
```bash
# Проверка __pycache__
docker exec -it news-analyzer find /home/appuser/.local -name "__pycache__" | wc -l
# Должно быть 0 или очень мало

# Проверка тестов
docker exec -it news-analyzer find /home/appuser/.local -type d -name "tests" | wc -l
# Должно быть 0
```

### Работа приложения:
```bash
docker exec -it news-analyzer python test_dependencies.py
docker exec -it news-analyzer python test_connection.py
```

## Измененные файлы

1. ✅ `Dockerfile` - улучшена очистка Python мусора
2. ✅ `.github/workflows/ci-cd.yml` - добавлен job для сборки образа
3. ✅ `docker-compose.yml` - изменено с build на image
4. ✅ Убрано копирование news-analyzer-python при деплое

## Очистка при деплое

При каждом деплое автоматически выполняется:
- `docker image prune -af` - неиспользуемые образы
- `docker container prune -f` - остановленные контейнеры
- `docker network prune -f` - неиспользуемые сети
- `docker volume prune -f` - неиспользуемые volumes
- `docker builder prune -af` - build cache

Это помогает поддерживать сервер в чистом состоянии и экономить место.
