# Ansible Vault - Управление секретами

## Обзор

Ansible Vault используется для безопасного хранения и управления секретными данными (пароли, API ключи, сертификаты) в зашифрованном виде.

## Настройка

### 1. Инициализация Vault

```bash
cd ansible
./scripts/vault_setup.sh
```

Этот скрипт:
- Создаст vault password файл (`.vault_password`)
- Создаст шаблон vault файла (`inventory/group_vars/all/vault.yml`)
- Зашифрует vault файл

### 2. Редактирование секретов

```bash
# Отредактировать зашифрованный файл
ansible-vault edit inventory/group_vars/all/vault.yml --vault-password-file .vault_password

# Или через переменную окружения
export ANSIBLE_VAULT_PASSWORD_FILE=.vault_password
ansible-vault edit inventory/group_vars/all/vault.yml
```

### 3. Структура секретов

```yaml
# База данных
vault_postgres_password: "your_secure_password"
vault_postgres_user: "tg_rss"
vault_postgres_db: "tg_rss"

# Redis
vault_redis_password: "your_redis_password"

# Telegram боты
vault_telegram_api_key: "your_telegram_bot_token"
vault_telegram_signal_api_key: "your_signal_bot_token"

# SSH ключи (для управления)
vault_ssh_private_key: |
  -----BEGIN OPENSSH PRIVATE KEY-----
  your_private_key_here
  -----END OPENSSH PRIVATE KEY-----

# SSL сертификаты
vault_ssl_cert: |
  -----BEGIN CERTIFICATE-----
  your_ssl_certificate_here
  -----END CERTIFICATE-----

vault_ssl_key: |
  -----BEGIN PRIVATE KEY-----
  your_ssl_private_key_here
  -----END PRIVATE KEY-----

# Внешние сервисы
vault_grafana_admin_password: "secure_grafana_password"
vault_prometheus_web_password: "secure_prometheus_password"

# Docker registry
vault_docker_registry_username: "registry_user"
vault_docker_registry_password: "registry_password"
```

## Использование

### Запуск с vault

```bash
# Через файл пароля
ansible-playbook playbooks/server-setup.yml --vault-password-file .vault_password

# Через переменную окружения
export ANSIBLE_VAULT_PASSWORD_FILE=.vault_password
ansible-playbook playbooks/server-setup.yml

# Через stdin (интерактивно)
ansible-playbook playbooks/server-setup.yml --ask-vault-pass
```

### Работа с vault файлами

```bash
# Просмотр зашифрованного файла
ansible-vault view inventory/group_vars/all/vault.yml

# Шифрование файла
ansible-vault encrypt inventory/group_vars/all/vault.yml

# Расшифровка файла
ansible-vault decrypt inventory/group_vars/all/vault.yml

# Перешифровка с новым паролем
ansible-vault rekey inventory/group_vars/all/vault.yml
```

## Безопасность

### Лучшие практики

1. **Хранение vault password**
   ```bash
   # Создайте отдельный файл с ограниченными правами
   chmod 600 .vault_password
   chown ansible:ansible .vault_password
   ```

2. **Переменные окружения**
   ```bash
   # Используйте переменные окружения для CI/CD
   export ANSIBLE_VAULT_PASSWORD_FILE=.vault_password
   export VAULT_PASSWORD=$(cat .vault_password)
   ```

3. **Разделение секретов**
   ```yaml
   # Используйте разные vault файлы для разных окружений
   inventory/group_vars/production/vault.yml
   inventory/group_vars/staging/vault.yml
   inventory/group_vars/development/vault.yml
   ```

4. **Ротация секретов**
   ```bash
   # Регулярно меняйте vault password
   ansible-vault rekey inventory/group_vars/all/vault.yml

   # Обновляйте секреты
   ansible-vault edit inventory/group_vars/all/vault.yml
   ```

### CI/CD интеграция

```yaml
# .github/workflows/deploy.yml
- name: Deploy with Ansible
  run: |
    echo "${{ secrets.ANSIBLE_VAULT_PASSWORD }}" > .vault_password
    chmod 600 .vault_password
    ansible-playbook playbooks/server-setup.yml --vault-password-file .vault_password
```

### Проверка безопасности

```bash
# Проверка что vault файлы зашифрованы
ansible-vault view inventory/group_vars/all/vault.yml > /dev/null

# Проверка целостности vault
ansible-inventory --list --vault-password-file .vault_password
```

## Переменные в ролях

### Использование vault переменных в ролях

```yaml
# roles/docker_setup/defaults/main.yml
docker_registry_username: "{{ vault_docker_registry_username | default('') }}"
docker_registry_password: "{{ vault_docker_registry_password | default('') }}"
```

### Переопределение переменных

```yaml
# inventory/group_vars/production/main.yml
postgres_password: "{{ vault_postgres_password }}"
redis_password: "{{ vault_redis_password }}"
telegram_api_key: "{{ vault_telegram_api_key }}"
```

## Troubleshooting

### Распространенные проблемы

1. **"Vault password required"**
   ```bash
   # Укажите файл с паролем
   --vault-password-file .vault_password

   # Или используйте переменную окружения
   export ANSIBLE_VAULT_PASSWORD_FILE=.vault_password
   ```

2. **"Failed to decrypt"**
   ```bash
   # Проверьте правильность vault password
   ansible-vault view inventory/group_vars/all/vault.yml --vault-password-file .vault_password
   ```

3. **"Variable undefined"**
   ```yaml
   # Убедитесь что vault переменные правильно определены
   ansible-vault view inventory/group_vars/all/vault.yml

   # Проверьте синтаксис в playbook
   ansible-playbook playbooks/server-setup.yml --syntax-check --vault-password-file .vault_password
   ```

### Отладка

```bash
# Показать все переменные включая vault
ansible-playbook playbooks/server-setup.yml -v --vault-password-file .vault_password

# Показать только vault переменные
ansible-inventory --list --vault-password-file .vault_password | jq '.all.children.tg_rss_servers.hosts.localhost'
```