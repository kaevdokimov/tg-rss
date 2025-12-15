#!/bin/bash
set -e

# Функция для логирования
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

log "Запуск news-analyzer..."

# Проверяем наличие конфигурации
if [ ! -f "config.yaml" ]; then
    log "ОШИБКА: config.yaml не найден!"
    log "Скопируйте config.yaml.example в config.yaml и настройте его."
    exit 1
fi

# Проверяем наличие .env (опционально, может быть в переменных окружения)
if [ ! -f ".env" ]; then
    log "ПРЕДУПРЕЖДЕНИЕ: .env не найден, используем переменные окружения"
fi

# Инициализируем NLTK данные, если еще не загружены
log "Проверка данных NLTK..."
python setup_nltk.py || log "ПРЕДУПРЕЖДЕНИЕ: Не удалось загрузить данные NLTK"

# Создаем директории, если их нет
mkdir -p storage/reports storage/logs

# Определяем расписание из переменной окружения или используем по умолчанию (00:00 каждый день)
CRON_SCHEDULE="${ANALYZER_CRON_SCHEDULE:-0 0 * * *}"
log "Расписание запуска: $CRON_SCHEDULE"

# Если нужно запустить сразу при старте (для тестирования)
if [ "${RUN_ON_STARTUP:-false}" = "true" ]; then
    log "Запуск анализа при старте контейнера..."
    python run_daily.py || log "ОШИБКА: Анализ завершился с ошибкой"
fi

# Парсим расписание cron для простого цикла
parse_cron_schedule() {
    local schedule="$1"
    # Простой парсер для формата "минута час * * *"
    # Для более сложных расписаний можно использовать внешние библиотеки
    if [[ $schedule =~ ^([0-9]+)\ ([0-9]+)\ \*\ \*\ \*$ ]]; then
        CRON_MINUTE="${BASH_REMATCH[1]}"
        CRON_HOUR="${BASH_REMATCH[2]}"
        return 0
    else
        # По умолчанию 00:00
        CRON_MINUTE="0"
        CRON_HOUR="0"
        return 1
    fi
}

# Используем простой цикл для автоматического запуска (работает без root)
log "Используем простой цикл для автоматического запуска..."
parse_cron_schedule "$CRON_SCHEDULE" || log "Используем расписание по умолчанию: 00:00"

log "Контейнер готов. Ожидание времени запуска (${CRON_HOUR}:${CRON_MINUTE})..."
log "Для ручного запуска: docker exec -it news-analyzer python run_daily.py"

LAST_RUN_DATE=""

while true; do
    CURRENT_HOUR=$(date +%H)
    CURRENT_MINUTE=$(date +%M)
    CURRENT_DATE=$(date +%Y-%m-%d)
    
    # Проверяем, наступило ли время запуска и не запускали ли мы уже сегодня
    if [ "$CURRENT_HOUR" = "$CRON_HOUR" ] && [ "$CURRENT_MINUTE" = "$CRON_MINUTE" ] && [ "$LAST_RUN_DATE" != "$CURRENT_DATE" ]; then
        log "=========================================="
        log "Запуск ежедневного анализа..."
        log "=========================================="
        python run_daily.py
        EXIT_CODE=$?
        if [ $EXIT_CODE -eq 0 ]; then
            log "Анализ завершен успешно"
            LAST_RUN_DATE="$CURRENT_DATE"
        else
            log "ОШИБКА: Анализ завершился с кодом $EXIT_CODE"
        fi
        log "=========================================="
        # Ждем минуту, чтобы не запускать несколько раз
        sleep 60
    fi
    
    # Проверяем каждую минуту
    sleep 60
done
