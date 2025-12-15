# Multi-stage build для оптимизации размера образа
FROM python:3.11-slim AS builder

# Устанавливаем системные зависимости для сборки
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

# Создаем рабочую директорию
WORKDIR /app

# Копируем requirements.txt для кэширования слоев
COPY requirements.txt .

# Устанавливаем Python зависимости
RUN pip install --no-cache-dir --user -r requirements.txt

# Runtime stage
FROM python:3.11-slim

# Устанавливаем системные зависимости для runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    cron \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Создаем непривилегированного пользователя
RUN groupadd -r appuser && useradd -r -g appuser appuser

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем установленные пакеты из builder stage
COPY --from=builder /root/.local /home/appuser/.local

# Копируем исходный код
COPY --chown=appuser:appuser . .

# Копируем config.yaml.example как config.yaml, если config.yaml не существует
# Это позволяет использовать конфигурацию по умолчанию
RUN if [ ! -f config.yaml ]; then cp config.yaml.example config.yaml; fi

# Устанавливаем права на скрипты
RUN chmod +x run_daily.py setup_nltk.py test_connection.py docker-entrypoint.sh

# Переключаемся на непривилегированного пользователя
USER appuser

# Устанавливаем PATH для Python пакетов
ENV PATH=/home/appuser/.local/bin:$PATH
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# Инициализируем NLTK данные (если еще не загружены)
RUN python setup_nltk.py || true

# Создаем директории для данных
RUN mkdir -p storage/reports storage/logs

# Healthcheck для проверки работоспособности
HEALTHCHECK --interval=60s --timeout=10s --start-period=30s --retries=3 \
    CMD python -c "import sys; sys.path.insert(0, '/app/src'); from src.db import Database; from src.config import load_settings; s = load_settings(); db = Database(s.get_db_connection_string()); db.connect(); exit(0 if db.test_connection() else 1)" || exit 1

# По умолчанию запускаем entrypoint скрипт
ENTRYPOINT ["/app/docker-entrypoint.sh"]
