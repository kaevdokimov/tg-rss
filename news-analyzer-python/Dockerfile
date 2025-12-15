# Multi-stage build для оптимизации размера образа
FROM python:3.14-slim AS builder

# Устанавливаем системные зависимости для сборки
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    cmake \
    make \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Устанавливаем uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
ENV UV_SYSTEM_PYTHON=1

# Создаем рабочую директорию
WORKDIR /app

# Копируем requirements.txt для кэширования слоев
COPY requirements.txt .

# Устанавливаем build зависимости для hdbscan (cython и numpy нужны для компиляции)
RUN uv pip install --system --no-cache "cython<4.0" "numpy<3.0"

# Устанавливаем Python зависимости через uv
RUN uv pip install --system --no-cache -r requirements.txt && \
    # Проверяем, что пакеты установлены
    python -c "import site; pkgs = site.getsitepackages(); print('Site packages:', pkgs); print('First site-packages:', pkgs[0] if pkgs else 'NONE')" && \
    python -c "import nltk, pandas, numpy, sklearn, hdbscan, psycopg2; print('All packages imported successfully')" && \
    # Проверяем, где именно установлены пакеты
    ls -la /usr/local/lib/python3.14/site-packages/ | head -10 && \
    ls -la /usr/lib/python3.14/site-packages/ 2>/dev/null | head -10 || echo "No packages in /usr/lib/python3.14/site-packages" && \
    # Определяем путь к site-packages для очистки
    SITE_PKGS=$(python -c "import site; print(site.getsitepackages()[0])") && \
    echo "Cleaning packages in: $SITE_PKGS" && \
    # Очищаем временные файлы сборки
    find "$SITE_PKGS" -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true && \
    find "$SITE_PKGS" -name "*.pyc" -delete 2>/dev/null || true && \
    find "$SITE_PKGS" -name "*.pyo" -delete 2>/dev/null || true && \
    # Очищаем .dist-info и .egg-info (оставляем только необходимое)
    find "$SITE_PKGS" -type d -name "*.dist-info" -exec rm -rf {} + 2>/dev/null || true && \
    find "$SITE_PKGS" -type d -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true

# Runtime stage
FROM python:3.14-slim

# Устанавливаем системные зависимости для runtime (cron не нужен, используем простой цикл)
RUN apt-get update && apt-get install -y --no-install-recommends \
    tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean \
    && rm -rf /tmp/* /var/tmp/*

# Создаем непривилегированного пользователя
RUN groupadd -r appuser && useradd -r -g appuser appuser

# Устанавливаем рабочую директорию
WORKDIR /app

# Создаем директорию для пакетов перед копированием
RUN mkdir -p /usr/local/lib/python3.14/site-packages

# Копируем установленные пакеты из builder stage
# uv устанавливает пакеты в /usr/local/lib/python3.14/site-packages для системного Python
COPY --from=builder /usr/local/lib/python3.14/site-packages /usr/local/lib/python3.14/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Устанавливаем PYTHONPATH для поиска пакетов
ENV PYTHONPATH=/usr/local/lib/python3.14/site-packages

# Копируем исходный код
COPY --chown=appuser:appuser . .

# Копируем config.yaml.example как config.yaml, если config.yaml не существует
# Это позволяет использовать конфигурацию по умолчанию
RUN if [ ! -f config.yaml ]; then cp config.yaml.example config.yaml; fi && \
    # Устанавливаем права на скрипты
    chmod +x run_daily.py setup_nltk.py test_connection.py docker-entrypoint.sh && \
    chmod +x scripts/get_analysis_history.py 2>/dev/null || true && \
    # Создаем директории для данных (от root, потом изменим владельца)
    mkdir -p storage/reports storage/logs && \
    chown -R appuser:appuser storage && \
    # Проверяем, что пакеты доступны (проверка после копирования)
    echo "Checking packages after copy..." && \
    ls -la /usr/local/lib/python3.14/site-packages/ | head -5 && \
    python -c "import sys; print('Python path:', sys.path)" && \
    python -c "import site; print('Site packages:', site.getsitepackages())" && \
    python -c "import nltk; print('NLTK found:', nltk.__file__)" || (echo "WARNING: NLTK not found after copy" && python -c "import sys; [print(p) for p in sys.path]") && \
    # Инициализируем NLTK данные (если еще не загружены)
    python setup_nltk.py || true && \
    # Очищаем временные файлы
    rm -rf /tmp/* /var/tmp/* /root/.cache/nltk || true && \
    # Очищаем Python кэш и временные файлы
    find /usr/local/lib/python3.14/site-packages -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.14/site-packages -name "*.pyc" -delete 2>/dev/null || true && \
    find /usr/local/lib/python3.14/site-packages -name "*.pyo" -delete 2>/dev/null || true && \
    find /usr/local/lib/python3.14/site-packages -name "*.pyd" -delete 2>/dev/null || true && \
    # Очищаем документацию пакетов (не нужна в runtime)
    find /usr/local/lib/python3.14/site-packages -type d -name "*.dist-info" -exec sh -c 'rm -rf "$1"/RECORD "$1"/INSTALLER "$1"/REQUESTED 2>/dev/null || true' _ {} \; || true && \
    # Очищаем тесты из установленных пакетов
    find /usr/local/lib/python3.14/site-packages -type d -name "tests" -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.14/site-packages -type d -name "test" -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.14/site-packages -type d -name "*.tests" -exec rm -rf {} + 2>/dev/null || true

# Переключаемся на непривилегированного пользователя
USER appuser

# Устанавливаем переменные окружения
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1
ENV UV_SYSTEM_PYTHON=1
# PYTHONPATH уже установлен выше

# Healthcheck для проверки работоспособности
HEALTHCHECK --interval=60s --timeout=10s --start-period=30s --retries=3 \
    CMD python -c "import sys; sys.path.insert(0, '/app/src'); from src.db import Database; from src.config import load_settings; s = load_settings(); db = Database(s.get_db_connection_string()); db.connect(); exit(0 if db.test_connection() else 1)" || exit 1

# По умолчанию запускаем entrypoint скрипт
ENTRYPOINT ["/app/docker-entrypoint.sh"]
