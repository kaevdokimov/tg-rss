# Настройка автоматического деплоя

Этот документ описывает, как настроить автоматический деплой приложения на сервер через GitHub Actions.

## Отключение деплоя по умолчанию

По умолчанию автоматический деплой **отключен**, чтобы избежать ошибок при отсутствии настроенного сервера. CI/CD pipeline будет собирать Docker образы, но не будет пытаться задеплоить их.

## Включение автоматического деплоя

### 1. Настройка переменных репозитория

Перейдите в настройки репозитория: **Settings → Secrets and variables → Actions → Variables**

Создайте следующие переменные:

#### Для production деплоя (main ветка):
```
DEPLOYMENT_ENABLED = true
```

#### Для staging деплоя (develop/staging ветка):
```
STAGING_DEPLOYMENT_ENABLED = true
```

### 2. Настройка секретов для production деплоя

Перейдите в: **Settings → Secrets and variables → Actions → Secrets**

Создайте следующие секреты:

#### SSH доступ к серверу:
- `SERVER_HOST` - IP адрес или домен сервера (например: `192.168.1.100` или `example.com`)
- `SERVER_USER` - имя пользователя SSH (например: `deploy` или `ubuntu`)
- `SERVER_SSH_KEY` - приватный SSH ключ для доступа к серверу
- `SERVER_PORT` - порт SSH (опционально, по умолчанию: 22)

#### Credentials для приложения:
- `TELEGRAM_API_KEY` - токен основного Telegram бота
- `TELEGRAM_SIGNAL_API_KEY` - токен бота для отчетов анализа
- `POSTGRES_USER` - пользователь PostgreSQL
- `POSTGRES_PASSWORD` - пароль PostgreSQL
- `POSTGRES_DB` - имя базы данных PostgreSQL
- `CONTENT_SCRAPER_INTERVAL` - интервал скрапинга контента (например: `30m`)
- `CONTENT_SCRAPER_BATCH` - размер батча для скрапинга (например: `10`)
- `CONTENT_SCRAPER_CONCURRENT` - количество параллельных скраперов (например: `3`)

#### Health check (опционально для rollback):
- `SSH_HOST` - хост для health check (может совпадать с `SERVER_HOST`)
- `SSH_USER` - пользователь для health check (может совпадать с `SERVER_USER`)
- `SSH_KEY` - SSH ключ для health check (может совпадать с `SERVER_SSH_KEY`)
- `SSH_PORT` - порт SSH для health check (может совпадать с `SERVER_PORT`)

### 3. Настройка секретов для staging деплоя

Если используете staging окружение, создайте аналогичные секреты с префиксом `STAGING_`:

- `STAGING_HOST`
- `STAGING_USER`
- `STAGING_SSH_KEY`
- `STAGING_SSH_PORT`

### 4. Настройка секретов для Telegram уведомлений (опционально)

Для получения уведомлений о деплоях в Telegram:

- `GH_NOTIFY_TELEGRAM_BOT_TOKEN` - токен Telegram бота для уведомлений
- `GH_NOTIFY_TELEGRAM_CHAT_ID` - ID чата для отправки уведомлений

## Генерация SSH ключа

Если у вас еще нет SSH ключа для деплоя, создайте его:

```bash
# Генерация нового SSH ключа
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/deploy_key -N ""

# Скопируйте публичный ключ на сервер
ssh-copy-id -i ~/.ssh/deploy_key.pub user@your-server.com

# Скопируйте приватный ключ в буфер обмена
cat ~/.ssh/deploy_key
```

Содержимое приватного ключа (`~/.ssh/deploy_key`) добавьте в секрет `SERVER_SSH_KEY`.

## Подготовка сервера

На сервере должны быть установлены:

1. **Docker** (версия 20.10+)
2. **Docker Compose** (версия 2.0+)

### Установка Docker на Ubuntu:

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
```

### Создание директории для проекта:

```bash
mkdir -p ~/news-bot
```

## Проверка настройки

После настройки всех секретов и переменных:

1. Сделайте commit и push в ветку `main` (для production) или `develop/staging` (для staging)
2. Откройте Actions в GitHub репозитории
3. Найдите запущенный workflow "CI/CD Pipeline" или "Deploy to Staging"
4. Проверьте, что job "Deploy to Server" или "build-and-deploy" выполняется успешно

## Отключение деплоя

Чтобы временно отключить автоматический деплой, удалите или измените переменные:

- `DEPLOYMENT_ENABLED` → `false` (для production)
- `STAGING_DEPLOYMENT_ENABLED` → `false` (для staging)

## Troubleshooting

### Ошибка "missing server host"

Это означает, что секрет `SERVER_HOST`, `STAGING_HOST`, `SSH_HOST` не установлен или пустой.

**Решение:** Проверьте, что все необходимые секреты созданы в Settings → Secrets.

### SSH connection failed

**Возможные причины:**
1. Неправильный формат SSH ключа (должен быть приватный ключ)
2. Неверный хост или порт
3. Firewall блокирует SSH соединение
4. Публичный ключ не добавлен в `~/.ssh/authorized_keys` на сервере

**Решение:** Проверьте логи workflow и убедитесь, что можете подключиться к серверу вручную.

### Docker permission denied

**Решение:** Добавьте пользователя в группу docker:
```bash
sudo usermod -aG docker $USER
# Выйдите и войдите заново
```

## Дополнительная информация

- [DEPLOYMENT.md](./DEPLOYMENT.md) - Подробное руководство по деплою
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - Решение проблем
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
