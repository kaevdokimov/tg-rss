# Руководство по настройке безопасности

Полное руководство по настройке безопасности проекта TG-RSS.

## Оглавление

- [GitHub Secrets](#github-secrets)
- [API Security](#api-security)
- [Branch Protection](#branch-protection)
- [Ротация секретов](#ротация-секретов)
- [Best Practices](#best-practices)

## GitHub Secrets

### Обзор необходимых секретов

Проект требует следующие секреты для работы CI/CD и деплоя:

#### Production деплой
- `SERVER_HOST` - IP или домен сервера
- `SERVER_USER` - пользователь SSH
- `SERVER_SSH_KEY` - приватный SSH ключ
- `SERVER_PORT` - порт SSH (опционально, по умолчанию: 22)

#### Application credentials
- `TELEGRAM_API_KEY` - токен основного Telegram бота
- `TELEGRAM_SIGNAL_API_KEY` - токен бота для отчетов
- `POSTGRES_USER` - пользователь PostgreSQL
- `POSTGRES_PASSWORD` - пароль PostgreSQL
- `POSTGRES_DB` - имя базы данных

#### News Analyzer API
- `NEWS_ANALYZER_ADMIN` - имя пользователя для Basic Auth
- `NEWS_ANALYZER_PASSWORD` - пароль для Basic Auth

### Добавление секретов

#### Через веб-интерфейс

1. Откройте: **Settings** → **Secrets and variables** → **Actions**
2. Нажмите **New repository secret**
3. Введите **Name** и **Value**
4. Нажмите **Add secret**

#### Через GitHub CLI

```bash
# Установка gh CLI
brew install gh  # macOS
# или
sudo apt install gh  # Ubuntu

# Авторизация
gh auth login

# Добавление секретов
gh secret set NEWS_ANALYZER_ADMIN -b"admin"
gh secret set NEWS_ANALYZER_PASSWORD -b"$(openssl rand -base64 32)"
gh secret set TELEGRAM_API_KEY -b"your_bot_token"
gh secret set POSTGRES_PASSWORD -b"$(openssl rand -base64 32)"
```

### Генерация надежных паролей

```bash
# OpenSSL (рекомендуется)
openssl rand -base64 32

# Python
python3 -c "import secrets; print(secrets.token_urlsafe(32))"

# pwgen (если установлен)
pwgen -s 32 1
```

**⚠️ Важно:** Сохраняйте пароли в password manager!

### Проверка секретов

```bash
# Список добавленных секретов
gh secret list

# Проверка на сервере (после деплоя)
docker exec news-analyzer env | grep NEWS_ANALYZER
```

## API Security

### Защита документации FastAPI

Endpoints `/docs`, `/redoc`, `/openapi.json` защищены HTTP Basic Authentication.

#### Локальная разработка

Добавьте в `.env`:

```bash
NEWS_ANALYZER_ADMIN=admin
NEWS_ANALYZER_PASSWORD=your_local_password
```

#### Production

Секреты автоматически передаются через GitHub Actions в `docker-compose.yml`:

```yaml
news-analyzer:
  environment:
    NEWS_ANALYZER_ADMIN: ${NEWS_ANALYZER_ADMIN}
    NEWS_ANALYZER_PASSWORD: ${NEWS_ANALYZER_PASSWORD}
```

### Проверка работы

```bash
# Без авторизации - должен вернуть 401
curl http://your-server:8000/docs

# С правильными credentials - должен вернуть HTML
curl -u admin:your_password http://your-server:8000/docs

# Health check (публичный endpoint)
curl http://your-server:8000/health
```

## Branch Protection

### Быстрая настройка (автоматически)

#### Шаг 1: Настройка прав GITHUB_TOKEN

1. Перейдите: **Settings** → **Actions** → **General**
2. В разделе **Workflow permissions**:
   - ✅ **Read and write permissions**
3. Нажмите **Save**

#### Шаг 2: Запуск workflow

1. Перейдите: **Actions** → **🛡️ Branch Protection**
2. Нажмите **Run workflow**
3. Workflow автоматически применит защиту

### Ручная настройка

Если автоматическая настройка не работает:

1. Перейдите: **Settings** → **Branches**
2. Нажмите **Add rule**
3. Настройте:

```
Branch name pattern: main

✅ Require a pull request before merging
   - Require approvals: 1
   - Dismiss stale pull request approvals when new commits are pushed

✅ Require status checks to pass before merging
   - Require branches to be up to date before merging
   - Status checks: test, lint, security-scan

❌ Allow force pushes
❌ Allow deletions
```

### Результат

После настройки:
- **Branch-Protection score**: 0 → 10
- **OpenSSF Scorecard**: +1.0 балла
- Защита от случайных force push
- Обязательный code review

## Ротация секретов

### Когда обновлять

- **Регулярно**: раз в 90 дней
- **Немедленно**: при подозрении на компрометацию
- **При смене персонала**: при уходе сотрудников с доступом

### Как обновить

```bash
# 1. Генерация нового пароля
NEW_PASSWORD=$(openssl rand -base64 32)
echo "Новый пароль: $NEW_PASSWORD"

# 2. Обновление в GitHub
gh secret set NEWS_ANALYZER_PASSWORD -b"$NEW_PASSWORD"

# 3. Триггер редеплоя
git commit --allow-empty -m "chore(sec): обновить пароль API"
git push origin main

# 4. Проверка после деплоя
curl -u admin:$NEW_PASSWORD http://your-server:8000/docs
```

### SSH ключи

#### Генерация нового ключа

```bash
# Создание ключа
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/deploy_key -N ""

# Копирование на сервер
ssh-copy-id -i ~/.ssh/deploy_key.pub user@your-server.com

# Добавление в GitHub Secrets
gh secret set SERVER_SSH_KEY < ~/.ssh/deploy_key
```

#### Ротация существующего ключа

```bash
# 1. Создать новый ключ
ssh-keygen -t ed25519 -C "deploy-$(date +%Y%m)" -f ~/.ssh/deploy_new -N ""

# 2. Добавить на сервер (не удаляя старый)
ssh-copy-id -i ~/.ssh/deploy_new.pub user@server

# 3. Обновить secret
gh secret set SERVER_SSH_KEY < ~/.ssh/deploy_new

# 4. Проверить работоспособность
# Запустить workflow вручную

# 5. Удалить старый ключ с сервера
ssh user@server "sed -i '/old-key-comment/d' ~/.ssh/authorized_keys"
```

## Best Practices

### Пароли

✅ **Рекомендуется:**
- Минимум 16 символов
- Случайная генерация (не "password123")
- Хранение в password manager
- Регулярная ротация (90 дней)
- Уникальные пароли для каждого сервиса

❌ **Запрещено:**
- Дефолтные пароли (`admin`/`changeme`)
- Короткие пароли (<16 символов)
- Предсказуемые пароли
- Хранение в открытом виде
- Коммит секретов в Git

### SSH ключи

✅ **Рекомендуется:**
- Использовать ed25519 (быстрее и безопаснее RSA)
- Разные ключи для разных окружений
- Ограничить права ключа (только нужные команды)
- Регулярная ротация
- Защита приватного ключа паролем (для личного использования)

❌ **Запрещено:**
- Использовать один ключ везде
- Хранить приватные ключи в репозитории
- Оставлять ключи без passphrase (локально)
- Игнорировать старые неиспользуемые ключи

### Мониторинг безопасности

```bash
# Проверка попыток неудачной авторизации
docker logs news-analyzer | grep "401"

# Проверка использования secrets
docker exec news-analyzer env | grep -E "(TELEGRAM|POSTGRES|NEWS_ANALYZER)"

# Аудит SSH подключений на сервере
sudo journalctl -u ssh | grep "Accepted"
```

### Ограничение доступа

#### Firewall

```bash
# Разрешить только необходимые порты
sudo ufw allow 22/tcp   # SSH
sudo ufw allow 80/tcp   # HTTP (если нужен)
sudo ufw allow 443/tcp  # HTTPS (если нужен)
sudo ufw enable

# Проверка правил
sudo ufw status
```

#### IP Whitelist (опционально)

```bash
# Ограничить SSH только для доверенных IP
sudo ufw delete allow 22/tcp
sudo ufw allow from YOUR_IP to any port 22
```

## Troubleshooting

### Проблема: 401 даже с правильным паролем

**Причины:**
1. Secrets не обновились на сервере
2. Кэшированный старый образ
3. Пароль содержит спецсимволы

**Решение:**

```bash
# Проверить переменные в контейнере
docker exec news-analyzer env | grep NEWS_ANALYZER

# Если пусто - пересоздать контейнер
docker-compose up -d --force-recreate news-analyzer

# Удалить старый образ
docker rmi ghcr.io/yourusername/news-analyzer:latest
docker-compose pull news-analyzer
docker-compose up -d
```

### Проблема: GitHub Actions не видит secrets

**Решение:**
- Убедитесь что secrets в **Repository secrets** (не Environment)
- Проверьте права администратора на репозиторий
- Secrets должны быть видны в Settings → Secrets and variables → Actions

### Проблема: SSH connection failed

**Диагностика:**

```bash
# Проверка подключения вручную
ssh -i ~/.ssh/deploy_key user@server

# Проверка authorized_keys на сервере
ssh user@server "cat ~/.ssh/authorized_keys"

# Проверка прав файлов
ssh user@server "ls -la ~/.ssh/"
```

**Решение:**
- Права на ~/.ssh должны быть 700
- Права на authorized_keys должны быть 600
- Публичный ключ должен быть добавлен на сервер
- Firewall не должен блокировать SSH порт

## Checklist безопасности

После настройки проверьте:

- [ ] Все secrets добавлены в GitHub
- [ ] Secrets используют надежные пароли (не дефолтные)
- [ ] SSH ключи настроены и работают
- [ ] Branch protection активна
- [ ] Workflow permissions настроены (Read and write)
- [ ] `/docs` требует авторизацию
- [ ] `/health` работает без авторизации
- [ ] Firewall настроен на сервере
- [ ] Пароли сохранены в password manager
- [ ] Документирован процесс ротации

## Дополнительные ресурсы

- [GitHub Secrets Documentation](https://docs.github.com/en/actions/security-guides/encrypted-secrets)
- [FastAPI Security](https://fastapi.tiangolo.com/tutorial/security/)
- [OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/)
- [SSH Best Practices](https://infosec.mozilla.org/guidelines/openssh)
