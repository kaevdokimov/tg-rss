# Multi-stage build для оптимизации размера образа
FROM python:3.13-slim AS builder

# Устанавливаем системные зависимости для сборки
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    cmake \
    make \
    libc6-dev \
    libyaml-dev \
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
    ls -la /usr/local/lib/python3.13/site-packages/ | head -10 && \
    ls -la /usr/lib/python3.13/site-packages/ 2>/dev/null | head -10 || echo "No packages in /usr/lib/python3.13/site-packages" && \
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
FROM python:3.13-slim

# Устанавливаем системные зависимости для runtime
# Включаем build зависимости для компиляции hdbscan, pyyaml и psycopg2-binary
RUN apt-get update && apt-get install -y --no-install-recommends \
    tzdata \
    gcc \
    g++ \
    cmake \
    make \
    libc6-dev \
    libyaml-dev \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean \
    && rm -rf /tmp/* /var/tmp/*

# Устанавливаем uv для установки пакетов в runtime (более надежный способ)
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
ENV UV_SYSTEM_PYTHON=1

# Создаем непривилегированного пользователя
RUN groupadd -r appuser && useradd -r -g appuser appuser

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем requirements.txt для установки пакетов
COPY --from=builder /app/requirements.txt /tmp/requirements.txt

# Устанавливаем build зависимости для hdbscan
RUN uv pip install --system --no-cache "cython<4.0" "numpy<3.0"

# Устанавливаем Python зависимости через uv в runtime stage
# Это гарантирует, что пакеты будут доступны
# scikit-learn автоматически установит scipy, который нужен для hdbscan
# Устанавливаем scikit-learn первым, чтобы scipy был доступен для hdbscan
RUN uv pip install --system --no-cache "scikit-learn==1.6.0" && \
    uv pip install --system --no-cache -r /tmp/requirements.txt

# Проверяем установку всех критических пакетов отдельно для диагностики
RUN python -c "import nltk; print('✓ nltk')" && \
    python -c "import pandas; print('✓ pandas')" && \
    python -c "import numpy; print('✓ numpy')" && \
    python -c "import sklearn; print('✓ sklearn')" && \
    python -c "import hdbscan; print('✓ hdbscan')" && \
    python -c "import psycopg2; print('✓ psycopg2')" && \
    python -c "import yaml; print('✓ yaml (pyyaml)')" && \
    python -c "import dotenv; print('✓ python-dotenv')" && \
    python -c "import requests; print('✓ requests')" && \
    python -c "import tqdm; print('✓ tqdm')" && \
    python -c "import redis; print('✓ redis')" && \
    python -c "import aiofiles; print('✓ aiofiles')" && \
    echo "All packages imported successfully"

# Очищаем временные файлы и удаляем build зависимости
# Оставляем libpq-dev и postgresql-client, так как они могут понадобиться для psycopg2-binary
RUN rm -f /tmp/requirements.txt && \
    apt-get purge -y gcc g++ cmake make libc6-dev libyaml-dev && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Устанавливаем PYTHONPATH для поиска пакетов
ENV PYTHONPATH=/usr/local/lib/python3.13/site-packages:/app/nltk_data
# Устанавливаем директорию для данных NLTK
ENV NLTK_DATA=/app/nltk_data
# Настройки для FastAPI
ENV API_HOST=0.0.0.0
ENV API_PORT=8000

# Копируем исходный код
COPY --chown=appuser:appuser . .

# Устанавливаем права на healthcheck скрипт
RUN chmod +x healthcheck.py

# Копируем config.yaml.example как config.yaml, если config.yaml не существует
# Это позволяет использовать конфигурацию по умолчанию
RUN if [ ! -f config.yaml ]; then cp config.yaml.example config.yaml; fi && \
    # Устанавливаем права на скрипты
    chmod +x run_daily.py setup_nltk.py test_connection.py docker-entrypoint.sh && \
    chmod +x scripts/get_analysis_history.py 2>/dev/null || true && \
    # Создаем директории для данных (от root, потом изменим владельца)
    mkdir -p storage/reports storage/logs nltk_data && \
    chown -R appuser:appuser storage nltk_data && \
    # Загружаем NLTK данные от имени appuser
    su - appuser -c "python setup_nltk.py" && \
    # Проверяем, что все критические пакеты доступны
    python -c "import nltk; print('✓ NLTK found:', nltk.__file__)" || echo "WARNING: NLTK not found" && \
    python -c "import yaml; print('✓ yaml (pyyaml) found:', yaml.__file__)" || echo "ERROR: yaml not found" && \
    python -c "import pandas, numpy, sklearn, hdbscan, psycopg2, dotenv, requests, tqdm; print('✓ All other packages available')" || echo "WARNING: Some packages not found" && \
    # Инициализируем NLTK данные (если еще не загружены)
    python setup_nltk.py || true && \
    # Очищаем временные файлы
    rm -rf /tmp/* /var/tmp/* /root/.cache/nltk || true && \
    # Очищаем Python кэш и временные файлы
    find /usr/local/lib/python3.13/site-packages -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.13/site-packages -name "*.pyc" -delete 2>/dev/null || true && \
    find /usr/local/lib/python3.13/site-packages -name "*.pyo" -delete 2>/dev/null || true && \
    find /usr/local/lib/python3.13/site-packages -name "*.pyd" -delete 2>/dev/null || true && \
    # Очищаем документацию пакетов (не нужна в runtime)
    find /usr/local/lib/python3.13/site-packages -type d -name "*.dist-info" -exec sh -c 'rm -rf "$1"/RECORD "$1"/INSTALLER "$1"/REQUESTED 2>/dev/null || true' _ {} \; || true && \
    # Очищаем тесты из установленных пакетов
    find /usr/local/lib/python3.13/site-packages -type d -name "tests" -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.13/site-packages -type d -name "test" -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.13/site-packages -type d -name "*.tests" -exec rm -rf {} + 2>/dev/null || true

# Переключаемся на непривилегированного пользователя
USER appuser

# Устанавливаем переменные окружения
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1
ENV UV_SYSTEM_PYTHON=1
# PYTHONPATH и NLTK_DATA уже установлены выше

# Healthcheck для проверки работоспособности
HEALTHCHECK --interval=60s --timeout=10s --start-period=30s --retries=3 \
    CMD python healthcheck.py || exit 1

# По умолчанию запускаем entrypoint скрипт
ENTRYPOINT ["/app/docker-entrypoint.sh"]
