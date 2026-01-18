# Ansible для настройки TG-RSS сервера

Этот Ansible playbook автоматически настраивает сервер для развертывания TG-RSS приложения с оптимизациями для ограниченных ресурсов (1 CPU, 1GB RAM, 15GB SSD).

## 🚀 Быстрый старт

### Предварительные требования

- Ubuntu 24.04 сервер
- SSH доступ с ключом
- Ansible 2.9+ на локальной машине

### Настройка инвентаря

1. Отредактируйте `ansible/inventory/hosts.ini`:
```ini
[tg_rss_servers]
your-server-ip ansible_host=YOUR_SERVER_IP ansible_user=root
```

2. Убедитесь, что SSH ключ настроен для доступа к серверу.

### Запуск playbook

```bash
cd ansible
ansible-playbook -i inventory/hosts.ini playbooks/server-setup.yml
```

## 📋 Что делает playbook

### 1. Системные оптимизации (`system_optimization`)
- Настройка swap (1GB)
- Оптимизация sysctl параметров для ограниченных ресурсов
- Настройка limits.conf для Docker
- Автоматические обновления безопасности
- Firewall (UFW) с базовыми правилами
- Fail2ban для защиты SSH

### 2. Docker настройка (`docker_setup`)
- Установка Docker CE с Compose
- Оптимизация Docker daemon для низких ресурсов
- Создание необходимых директорий
- Systemd overrides для ограничения ресурсов

### 3. Мониторинг (`monitoring_setup`)
- Node Exporter для сбора метрик системы
- Prometheus для хранения и обработки метрик
- Grafana для визуализации
- Alertmanager для алертов
- Скрипты мониторинга приложения
- Автоматическое резервное копирование

### 4. Безопасность (`security_hardening`)
- Отключение неиспользуемых служб
- SSH hardening (отключение root login, паролей)
- Базовая настройка безопасности

## 🔐 Управление секретами (Ansible Vault)

Проект использует Ansible Vault для безопасного хранения секретов в CI/CD и локальной разработке.

### Локальная настройка

```bash
cd ansible

# Инициализация vault (создает пароль и шифрует секреты)
./scripts/vault_setup.sh

# Редактирование секретов
ansible-vault edit inventory/group_vars/all/vault.yml --vault-password-file .vault_password

# Запуск с vault
ansible-playbook playbooks/server-setup.yml --vault-password-file .vault_password
```

### Настройка CI/CD (GitHub Actions)

#### 1. Создание Vault Password секрета

```bash
# Создайте случайный пароль
openssl rand -base64 32 > vault_password.txt

# Добавьте как GitHub Secret: ANSIBLE_VAULT_PASSWORD
```

#### 2. GitHub Secrets (Settings → Secrets and variables → Actions)

Добавьте следующие секреты:

```bash
# Ansible Vault
ANSIBLE_VAULT_PASSWORD=<ваш_vault_пароль>

# Сервер для деплоя
SERVER_HOST=<ip_или_domain>
SERVER_USER=<ssh_user>
SERVER_PORT=<ssh_port>
SERVER_SSH_KEY=<private_ssh_key>

# База данных
POSTGRES_USER=<db_user>
POSTGRES_PASSWORD=<db_password>
POSTGRES_DB=<db_name>

# Redis
REDIS_PASSWORD=<redis_password>

# Telegram боты
TELEGRAM_API_KEY=<main_bot_token>
TELEGRAM_SIGNAL_API_KEY=<analytics_bot_token>

# Опционально
CONTENT_SCRAPER_INTERVAL=1
CONTENT_SCRAPER_BATCH=50
CONTENT_SCRAPER_CONCURRENT=3
```

#### 3. Структура секретов в Vault

```yaml
# inventory/group_vars/all/vault.yml (зашифровано)
vault_postgres_password: "your_secure_password"
vault_postgres_user: "tg_rss"
vault_postgres_db: "tg_rss"
vault_redis_password: "your_redis_password"
vault_telegram_api_key: "your_telegram_bot_token"
vault_telegram_signal_api_key: "your_signal_bot_token"
```

### Запуск в CI/CD

Ansible запускается автоматически при:
- Push в `main` ветку
- Коммит содержит `[infra]` в сообщении

```bash
# Ручной запуск инфраструктуры
git commit -m "[infra] Update server configuration"
git push origin main
```

### Безопасность

- **Шифрование**: AES256 для всех секретов
- **Access Control**: Только CI/CD имеет доступ к vault паролю
- **Audit Trail**: Все изменения логируются
- **Rotation**: Регулярная смена vault пароля

Подробная документация: [`docs/VAULT.md`](docs/VAULT.md)

## 🔧 Переменные окружения

Создайте `.env` файл в корне проекта или используйте Ansible Vault для секретов:

```bash
# База данных (или через vault)
POSTGRES_PASSWORD={{ vault_postgres_password }}
POSTGRES_USER={{ vault_postgres_user }}
POSTGRES_DB={{ vault_postgres_db }}

# Redis
REDIS_PASSWORD={{ vault_redis_password }}

# Telegram
TELEGRAM_API_KEY={{ vault_telegram_api_key }}
TELEGRAM_SIGNAL_API_KEY={{ vault_telegram_signal_api_key }}

# Оптимизированные лимиты ресурсов
BOT_MEM_LIMIT=400m
BOT_CPUS=0.4
DB_MEM_LIMIT=200m
DB_CPUS=0.12
REDIS_MEM_LIMIT=80m
REDIS_CPUS=0.05
```

## 📊 Мониторинг

После развертывания доступны:

- **Node Exporter**: `http://your-server:9100`
- **Prometheus**: `http://your-server:9090`
- **Grafana**: `http://your-server:3000` (admin/admin)
- **Alertmanager**: `http://your-server:9093`

## 🔄 Развертывание приложения

После настройки сервера через Ansible:

```bash
# На сервере
cd /opt/tg-rss
git clone https://github.com/your-repo/tg-rss.git .
cp env.example .env
# Отредактируйте .env файл
docker-compose up -d
```

## 🛠 Устранение неполадок

### Проверка статуса служб
```bash
sudo systemctl status docker
sudo systemctl status node-exporter
sudo systemctl status prometheus
```

### Просмотр логов
```bash
sudo journalctl -u docker -f
sudo journalctl -u tg-rss-monitor -f
```

### Проверка ресурсов
```bash
htop
df -h
free -h
```

## 📈 Оптимизации производительности

### Для 1 CPU, 1GB RAM сервера:
- **Bot**: 400MB RAM, 0.4 CPU
- **PostgreSQL**: 200MB RAM, 0.12 CPU
- **Redis**: 80MB RAM, 0.05 CPU
- **News Analyzer**: 512MB RAM, 0.3 CPU (опционально)

### Swap: 1GB для предотвращения OOM

### Мониторинг каждые 5 минут с автоматическим перезапуском при проблемах

### Ежедневные бэкапы в 2:00 ночи
