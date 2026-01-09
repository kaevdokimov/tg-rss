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

# Создаем директории, если их нет
mkdir -p storage/reports storage/logs

# Создаем директорию для NLTK данных, если её нет
NLTK_DATA_DIR="${NLTK_DATA:-/app/nltk_data}"
mkdir -p "$NLTK_DATA_DIR"
log "Директория NLTK: $NLTK_DATA_DIR"

# Проверяем, загружены ли NLTK данные
log "Проверка данных NLTK..."
if python -c "import nltk; nltk.data.find('tokenizers/punkt')" 2>/dev/null; then
    log "✓ NLTK punkt данные найдены"
else
    log "Загрузка данных NLTK..."
    python setup_nltk.py || log "ПРЕДУПРЕЖДЕНИЕ: Не удалось загрузить данные NLTK"
fi

# Определяем расписание запусков (по умолчанию 3:00, 9:00, 12:00, 15:00, 21:00)
# Формат: "HH:MM,HH:MM" или через переменную ANALYZER_SCHEDULE_TIMES
SCHEDULE_TIMES="${ANALYZER_SCHEDULE_TIMES:-3:00,9:00,12:00,15:00,21:00}"
# Убираем кавычки, если они есть (проблема с docker-compose.yml)
SCHEDULE_TIMES=$(echo "$SCHEDULE_TIMES" | sed 's/^"//;s/"$//')
log "Расписание запуска: $SCHEDULE_TIMES"

# Парсим расписание в массив
IFS=',' read -ra TIMES_ARRAY <<< "$SCHEDULE_TIMES"
declare -a SCHEDULE_HOURS=()
declare -a SCHEDULE_MINUTES=()

for time_str in "${TIMES_ARRAY[@]}"; do
    if [[ $time_str =~ ^([0-9]{1,2}):([0-9]{2})$ ]]; then
        hour="${BASH_REMATCH[1]}"
        minute="${BASH_REMATCH[2]}"
        # Убираем ведущий ноль из часа
        hour=$((10#$hour))
        minute=$((10#$minute))
        
        if [ $hour -ge 0 ] && [ $hour -le 23 ] && [ $minute -ge 0 ] && [ $minute -le 59 ]; then
            SCHEDULE_HOURS+=($hour)
            SCHEDULE_MINUTES+=($minute)
            log "Добавлено расписание: ${hour}:$(printf "%02d" $minute)"
        else
            log "ПРЕДУПРЕЖДЕНИЕ: Некорректное время '$time_str', пропускаем"
        fi
    else
        log "ПРЕДУПРЕЖДЕНИЕ: Некорректный формат времени '$time_str', ожидается HH:MM"
    fi
done

if [ ${#SCHEDULE_HOURS[@]} -eq 0 ]; then
    log "ОШИБКА: Не удалось распарсить расписание. Используем по умолчанию: 3:00, 9:00, 12:00, 15:00, 21:00"
    SCHEDULE_HOURS=(3 9 12 15 21)
    SCHEDULE_MINUTES=(0 0 0 0 0)
fi

# Если нужно запустить сразу при старте (для тестирования)
if [ "${RUN_ON_STARTUP:-false}" = "true" ]; then
    log "Запуск анализа при старте контейнера..."
    python run_daily.py || log "ОШИБКА: Анализ завершился с ошибкой"
fi

log "Контейнер готов. Ожидание времени запуска..."
for i in "${!SCHEDULE_HOURS[@]}"; do
    log "  - ${SCHEDULE_HOURS[$i]}:$(printf "%02d" ${SCHEDULE_MINUTES[$i]})"
done
log "Для ручного запуска: docker exec -it news-analyzer python run_daily.py"

# Храним время последнего запуска для каждого расписания
declare -A LAST_RUN_TIMES

while true; do
    CURRENT_HOUR=$(date +%H | sed 's/^0//')  # Убираем ведущий ноль
    CURRENT_MINUTE=$(date +%M | sed 's/^0//')  # Убираем ведущий ноль
    CURRENT_HOUR=$((10#$CURRENT_HOUR))
    CURRENT_MINUTE=$((10#$CURRENT_MINUTE))
    CURRENT_TIME_KEY="${CURRENT_HOUR}:${CURRENT_MINUTE}"
    CURRENT_DATE=$(date +%Y-%m-%d)
    
    # Проверяем каждое расписание
    for i in "${!SCHEDULE_HOURS[@]}"; do
        SCHED_HOUR=${SCHEDULE_HOURS[$i]}
        SCHED_MINUTE=${SCHEDULE_MINUTES[$i]}
        SCHED_TIME_KEY="${SCHED_HOUR}:${SCHED_MINUTE}"
        
        # Проверяем, наступило ли время запуска
        if [ "$CURRENT_HOUR" -eq "$SCHED_HOUR" ] && [ "$CURRENT_MINUTE" -eq "$SCHED_MINUTE" ]; then
            # Проверяем, не запускали ли мы уже в это время сегодня
            LAST_RUN_KEY="${CURRENT_DATE}_${SCHED_TIME_KEY}"
            
            if [ -z "${LAST_RUN_TIMES[$LAST_RUN_KEY]}" ]; then
                log "=========================================="
                log "Запуск анализа (${SCHED_HOUR}:$(printf "%02d" $SCHED_MINUTE))..."
                log "=========================================="
                python run_daily.py
                EXIT_CODE=$?
                if [ $EXIT_CODE -eq 0 ]; then
                    log "Анализ завершен успешно"
                    LAST_RUN_TIMES[$LAST_RUN_KEY]=1
                else
                    log "ОШИБКА: Анализ завершился с кодом $EXIT_CODE"
                fi
                log "=========================================="
                # Ждем минуту, чтобы не запускать несколько раз
                sleep 60
            fi
        fi
    done
    
    # Очищаем старые записи (старше текущей даты)
    for key in "${!LAST_RUN_TIMES[@]}"; do
        if [[ ! $key =~ ^${CURRENT_DATE}_ ]]; then
            unset LAST_RUN_TIMES[$key]
        fi
    done
    
    # Проверяем каждую минуту
    sleep 60
done
