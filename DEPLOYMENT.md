# Руководство по развертыванию TG-RSS

Полное руководство по развертыванию Telegram RSS бота на production-сервере.

## Оглавление

- [Требования](#требования)
- [Быстрый старт](#быстрый-старт)
- [Настройка автоматического деплоя](#настройка-автоматического-деплоя)
- [Варианты развертывания](#варианты-развертывания)
- [Конфигурация](#конфигурация)
- [Мониторинг](#мониторинг)
- [Обновление](#обновление)
- [Backup и восстановление](#backup-и-восстановление)
- [Безопасность](#безопасность)

## Требования

### Минимальные требования

- **ОС**: Ubuntu 20.04+ / Debian 11+ / CentOS 8+
- **RAM**: 1.2 GB (оптимизировано для VPS)
- **CPU**: 1 core
- **Диск**: 10 GB (включая логи и БД)
- **Сеть**: постоянное подключение к интернету

### Рекомендуемые требования

- **RAM**: 2 GB
- **CPU**: 2 cores
- **Диск**: 20 GB (SSD предпочтительнее)

### Необходимое ПО

Устанавливается автоматически через Ansible:
- Docker 24.0+
- Docker Compose 2.20+
- Git
- Python 3.10+ (для аналитического модуля)

## Быстрый старт

### 1. Подготовка

```bash
# Клонировать репозиторий
git clone https://github.com/yourusername/tg-rss.git
cd tg-rss

# Создать файл с переменными окружения
cp .env.example .env
```

### 2. Настройка переменных окружения

Отредактируйте `.env`:

```bash
# Обязательные параметры
TELEGRAM_API_KEY=your_telegram_bot_token_here
POSTGRES_PASSWORD=secure_password_here

# Опциональные параметры (с разумными значениями по умолчанию)
POSTGRES_USER=postgres
POSTGRES_DB=news_bot
REDIS_ADDR=redis:6379
```

**Важно**: 
- Получите `TELEGRAM_API_KEY` от [@BotFather](https://t.me/BotFather)
- Используйте надежный пароль для БД (минимум 16 символов)

### 3. Запуск

```bash
# Запустить все сервисы
docker-compose up -d

# Проверить статус
docker-compose ps

# Просмотреть логи
docker-compose logs -f
```

### 4. Проверка работоспособности

```bash
# Health check
curl http://localhost:8080/health

# Метрики
curl http://localhost:8080/metrics

# Проверка бота в Telegram
# Отправьте /start вашему боту
```

## Настройка автоматического деплоя

По умолчанию автоматический деплой **отключен**, чтобы избежать ошибок при отсутствии настроенного сервера.

### Включение автоматического деплоя

#### 1. Настройка переменных репозитория

Перейдите: **Settings → Secrets and variables → Actions → Variables**

Создайте переменные:

**Для production деплоя (main ветка):**
```
DEPLOYMENT_ENABLED = true
```

**Для staging деплоя (develop/staging ветка):**
```
STAGING_DEPLOYMENT_ENABLED = true
```

#### 2. Настройка секретов для production

Перейдите: **Settings → Secrets and variables → Actions → Secrets**

**SSH доступ к серверу:**
- `SERVER_HOST` - IP адрес или домен сервера
- `SERVER_USER` - имя пользователя SSH
- `SERVER_SSH_KEY` - приватный SSH ключ
- `SERVER_PORT` - порт SSH (опционально, по умолчанию: 22)

**Credentials для приложения:**
- `TELEGRAM_API_KEY` - токен основного Telegram бота
- `TELEGRAM_SIGNAL_API_KEY` - токен бота для отчетов
- `POSTGRES_USER` - пользователь PostgreSQL
- `POSTGRES_PASSWORD` - пароль PostgreSQL
- `POSTGRES_DB` - имя базы данных
- `CONTENT_SCRAPER_INTERVAL` - интервал скрапинга (например: `30m`)
- `CONTENT_SCRAPER_BATCH` - размер батча (например: `10`)
- `CONTENT_SCRAPER_CONCURRENT` - параллельные скраперы (например: `3`)

**News Analyzer API:**
- `NEWS_ANALYZER_ADMIN` - имя пользователя для Basic Auth
- `NEWS_ANALYZER_PASSWORD` - пароль для Basic Auth

**Telegram уведомления (опционально):**
- `GH_NOTIFY_TELEGRAM_BOT_TOKEN` - токен бота для уведомлений
- `GH_NOTIFY_TELEGRAM_CHAT_ID` - ID чата для уведомлений

#### 3. Генерация SSH ключа

```bash
# Генерация нового SSH ключа
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/deploy_key -N ""

# Скопировать публичный ключ на сервер
ssh-copy-id -i ~/.ssh/deploy_key.pub user@your-server.com

# Скопировать приватный ключ в буфер обмена
cat ~/.ssh/deploy_key
```

Содержимое приватного ключа (`~/.ssh/deploy_key`) добавьте в секрет `SERVER_SSH_KEY`.

#### 4. Подготовка сервера

На сервере должны быть установлены Docker и Docker Compose:

```bash
# Обновление системы
sudo apt update && sudo apt upgrade -y

# Установка Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Добавление пользователя в группу docker
sudo usermod -aG docker $USER

# Установка Docker Compose
sudo apt install docker-compose-plugin -y

# Проверка установки
docker --version
docker compose version

# Создание директории для проекта
mkdir -p ~/news-bot
```

#### 5. Проверка настройки

После настройки всех секретов и переменных:

1. Сделайте commit и push в ветку `main`
2. Откройте Actions в GitHub
3. Найдите запущенный workflow "CI/CD Pipeline"
4. Проверьте успешное выполнение job "Deploy to Server"

### Отключение деплоя

Чтобы временно отключить автоматический деплой:

- `DEPLOYMENT_ENABLED` → `false` (для production)
- `STAGING_DEPLOYMENT_ENABLED` → `false` (для staging)

## Варианты развертывания

### Вариант 1: Docker Compose (рекомендуется)

Для быстрого развертывания на одном сервере.

#### Шаги

1. **Подготовка сервера**

```bash
# Обновить систему
sudo apt update && sudo apt upgrade -y

# Установить Docker (если не установлен)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Установить Docker Compose
sudo apt install docker-compose-plugin -y
```

2. **Клонирование и настройка**

```bash
# Клонировать репозиторий
git clone https://github.com/yourusername/tg-rss.git
cd tg-rss

# Создать директории для данных
mkdir -p data/postgres data/prometheus data/grafana

# Настроить переменные окружения
cp .env.example .env
nano .env  # Отредактировать
```

3. **Запуск**

```bash
# Собрать образы
docker-compose build

# Запустить сервисы
docker-compose up -d

# Проверить логи
docker-compose logs -f news-bot
```

### Вариант 2: Ansible (для production)

Для автоматизированного развертывания с мониторингом и безопасностью.

#### Предварительные требования

- Ansible 2.10+ на локальной машине
- SSH доступ к серверу
- Sudo права на сервере

#### Шаги

1. **Настройка inventory**

```bash
cd ansible

# Скопировать пример inventory
cp inventory/production/hosts.example inventory/production/hosts

# Отредактировать hosts файл
nano inventory/production/hosts
```

Пример `hosts`:
```ini
[production]
your-server.com ansible_user=your_user ansible_port=22
```

2. **Настройка переменных**

```bash
# Скопировать пример переменных
cp inventory/production/group_vars/all.yml.example inventory/production/group_vars/all.yml

# Отредактировать переменные
nano inventory/production/group_vars/all.yml
```

3. **Создание секретов**

```bash
# Создать файл с секретами
ansible-vault create inventory/production/vault.yml

# Добавить секреты:
telegram_api_key: "your_bot_token"
postgres_password: "secure_password"
```

4. **Запуск playbook**

```bash
# Полная настройка сервера
ansible-playbook -i inventory/production/hosts playbooks/server-setup.yml --ask-vault-pass

# Быстрый деплой (обновление)
ansible-playbook -i inventory/production/hosts playbooks/fast-deploy.yml --ask-vault-pass
```

## Конфигурация

### Переменные окружения

#### Обязательные

| Переменная | Описание | Пример |
|------------|----------|--------|
| `TELEGRAM_API_KEY` | Токен Telegram бота | `123456:ABC-DEF...` |
| `POSTGRES_PASSWORD` | Пароль БД | `secure_pass_123` |

#### Опциональные

| Переменная | Описание | Значение по умолчанию |
|------------|----------|-----------------------|
| `POSTGRES_HOST` | Хост PostgreSQL | `db` |
| `POSTGRES_PORT` | Порт PostgreSQL | `5432` |
| `POSTGRES_USER` | Пользователь БД | `postgres` |
| `POSTGRES_DB` | Название БД | `news_bot` |
| `REDIS_ADDR` | Адрес Redis | `redis:6379` |
| `REDIS_PASSWORD` | Пароль Redis | `` (пусто) |
| `TIMEOUT` | Интервал опроса RSS (сек) | `60` |
| `LOG_LEVEL` | Уровень логирования | `INFO` |
| `CONTENT_SCRAPER_INTERVAL` | Интервал скрапинга (мин) | `1` |
| `CONTENT_SCRAPER_BATCH` | Размер батча | `50` |
| `CONTENT_SCRAPER_CONCURRENT` | Параллельные запросы | `3` |

### Порты

| Сервис | Порт | Описание |
|--------|------|----------|
| news-bot | 8080 | Health checks и метрики |
| PostgreSQL | 5432 | База данных (внутренний) |
| Redis | 6379 | Кэш и очереди (внутренний) |
| Prometheus | 9090 | Метрики (опционально) |
| Grafana | 3000 | Дашборды (опционально) |

## Мониторинг

### Health Checks

```bash
# Основной health check
curl http://localhost:8080/health

# Prometheus метрики
curl http://localhost:8080/metrics

# OpenAPI спецификация
curl http://localhost:8080/openapi.yaml
```

### Логи

```bash
# Все логи
docker-compose logs

# Логи конкретного сервиса
docker-compose logs news-bot
docker-compose logs news-analyzer

# Следить за логами в реальном времени
docker-compose logs -f news-bot

# Последние 100 строк
docker-compose logs --tail=100 news-bot
```

### Prometheus + Grafana

Доступ к дашбордам:
- Grafana: http://your-server:3000 (admin/admin)
- Prometheus: http://your-server:9090

Импорт дашборда:
1. Открыть Grafana
2. Dashboards → Import
3. Загрузить `docs/grafana-dashboard.json`

## Обновление

### Обновление через Docker Compose

```bash
# Остановить сервисы
docker-compose down

# Получить обновления
git pull origin main

# Пересобрать образы
docker-compose build

# Запустить с новой версией
docker-compose up -d

# Проверить логи
docker-compose logs -f news-bot
```

### Обновление через Ansible

```bash
cd ansible
ansible-playbook -i inventory/production/hosts playbooks/fast-deploy.yml --ask-vault-pass
```

### Обновление без downtime

```bash
# 1. Создать backup
docker exec tg-rss-db pg_dump -U postgres news_bot > backup_$(date +%Y%m%d).sql

# 2. Обновить код
git pull origin main

# 3. Пересобрать только news-bot
docker-compose build news-bot

# 4. Обновить с graceful restart
docker-compose up -d --no-deps news-bot

# 5. Проверить health
curl http://localhost:8080/health
```

## Backup и восстановление

### Backup базы данных

```bash
# Создать backup
docker exec tg-rss-db pg_dump -U postgres news_bot > backup.sql

# Сжать backup
gzip backup.sql
```

### Восстановление

```bash
# Распаковать backup
gunzip backup.sql.gz

# Восстановить БД
docker exec -i tg-rss-db psql -U postgres news_bot < backup.sql
```

### Автоматический backup

Добавить в crontab:

```bash
# Backup каждый день в 3:00
0 3 * * * docker exec tg-rss-db pg_dump -U postgres news_bot | gzip > /backups/news_bot_$(date +\%Y\%m\%d).sql.gz
```

## Безопасность

### Рекомендации

1. **Используйте надежные пароли**
   ```bash
   # Генерация пароля
   openssl rand -base64 32
   ```

2. **Ограничьте доступ к портам**
   ```bash
   # Только необходимые порты
   sudo ufw allow 22/tcp  # SSH
   sudo ufw allow 80/tcp  # HTTP (если нужен)
   sudo ufw allow 443/tcp # HTTPS (если нужен)
   sudo ufw enable
   ```

3. **Используйте HTTPS для API**
   - Настройте nginx reverse proxy с Let's Encrypt

4. **Регулярно обновляйте систему**
   ```bash
   sudo apt update && sudo apt upgrade -y
   ```

5. **Мониторьте логи**
   ```bash
   # Проверка подозрительной активности
   sudo journalctl -u docker -f
   ```

Подробнее: [docs/security-setup.md](docs/security-setup.md)

## Troubleshooting

### Ошибка "missing server host"

**Решение:** Проверьте, что секрет `SERVER_HOST` установлен в Settings → Secrets.

### SSH connection failed

**Возможные причины:**
1. Неправильный формат SSH ключа
2. Неверный хост или порт
3. Firewall блокирует SSH
4. Публичный ключ не добавлен в `~/.ssh/authorized_keys`

**Решение:** Проверьте логи workflow и убедитесь, что можете подключиться вручную.

### Docker permission denied

**Решение:**
```bash
sudo usermod -aG docker $USER
# Выйдите и войдите заново
```

Подробнее: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

## Поддержка

- Issues: https://github.com/yourusername/tg-rss/issues
- Документация: https://github.com/yourusername/tg-rss/tree/main/docs
- Email: support@example.com
