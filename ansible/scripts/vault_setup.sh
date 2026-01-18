#!/bin/bash
# Скрипт для настройки Ansible Vault

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ANSIBLE_DIR="$(dirname "$SCRIPT_DIR")"

cd "$ANSIBLE_DIR"

echo "🔐 Настройка Ansible Vault для TG-RSS"
echo "======================================"

# Проверка наличия ansible-vault
if ! command -v ansible-vault &> /dev/null; then
    echo "❌ ansible-vault не найден. Установите Ansible:"
    echo "   Ubuntu/Debian: sudo apt install ansible"
    echo "   macOS: brew install ansible"
    echo "   pip: pip install ansible"
    exit 1
fi

# Создание директорий
mkdir -p inventory/group_vars/all
mkdir -p logs

# Генерация случайного пароля для vault
if [ ! -f .vault_password ]; then
    echo "🔑 Генерация vault password..."
    openssl rand -base64 32 > .vault_password
    echo "✅ Vault password создан: .vault_password"
    echo "⚠️  Сохраните этот пароль в безопасном месте!"
    echo "   Он потребуется для расшифровки секретов."
fi

# Проверка существования vault файла
VAULT_FILE="inventory/group_vars/all/vault.yml"
if [ ! -f "$VAULT_FILE" ]; then
    echo "📝 Создание vault файла..."
    cat > "$VAULT_FILE" << 'EOF'
# Ansible Vault - Защищенные секреты
# Этот файл зашифрован ansible-vault

# База данных
vault_postgres_password: "CHANGE_THIS_PASSWORD"
vault_postgres_user: "tg_rss"
vault_postgres_db: "tg_rss"

# Redis
vault_redis_password: "CHANGE_THIS_REDIS_PASSWORD"

# Telegram
vault_telegram_api_key: "CHANGE_THIS_TELEGRAM_TOKEN"
vault_telegram_signal_api_key: "CHANGE_THIS_SIGNAL_TOKEN"

# SSH ключи (опционально)
vault_ssh_private_key: |
  -----BEGIN OPENSSH PRIVATE KEY-----
  CHANGE_THIS_SSH_PRIVATE_KEY
  -----END OPENSSH PRIVATE KEY-----

vault_ssh_public_key: "ssh-ed25519 CHANGE_THIS_SSH_PUBLIC_KEY user@host"

# SSL сертификаты (опционально)
vault_ssl_cert: |
  -----BEGIN CERTIFICATE-----
  CHANGE_THIS_SSL_CERT
  -----END CERTIFICATE-----

vault_ssl_key: |
  -----BEGIN PRIVATE KEY-----
  CHANGE_THIS_SSL_KEY
  -----END PRIVATE KEY-----

# Внешние сервисы
vault_grafana_admin_password: "CHANGE_THIS_GRAFANA_PASSWORD"
vault_prometheus_web_password: "CHANGE_THIS_PROMETHEUS_PASSWORD"

# Docker registry (опционально)
vault_docker_registry_username: "CHANGE_THIS_DOCKER_USER"
vault_docker_registry_password: "CHANGE_THIS_DOCKER_PASSWORD"
EOF
    echo "✅ Vault файл создан: $VAULT_FILE"
fi

# Шифрование vault файла
echo "🔒 Шифрование vault файла..."
if ansible-vault encrypt "$VAULT_FILE" --vault-password-file .vault_password; then
    echo "✅ Vault файл зашифрован"
else
    echo "❌ Ошибка шифрования vault файла"
    exit 1
fi

# Проверка расшифровки
echo "🔓 Проверка расшифровки vault файла..."
if ansible-vault view "$VAULT_FILE" --vault-password-file .vault_password > /dev/null; then
    echo "✅ Vault файл успешно расшифровывается"
else
    echo "❌ Ошибка расшифровки vault файла"
    exit 1
fi

echo ""
echo "🎉 Ansible Vault настроен успешно!"
echo ""
echo "📋 Следующие шаги:"
echo "1. Отредактируйте $VAULT_FILE с реальными секретами:"
echo "   ansible-vault edit $VAULT_FILE --vault-password-file .vault_password"
echo ""
echo "2. Запуск playbook с vault:"
echo "   ansible-playbook playbooks/server-setup.yml --vault-password-file .vault_password"
echo ""
echo "3. Или установите переменную окружения:"
echo "   export ANSIBLE_VAULT_PASSWORD_FILE=.vault_password"
echo ""
echo "⚠️  Важно:"
echo "   - Никогда не коммитите .vault_password в git"
echo "   - Храните vault password в безопасном месте"
echo "   - Регулярно меняйте vault password"
echo "   - Ограничьте доступ к .vault_password файлу"