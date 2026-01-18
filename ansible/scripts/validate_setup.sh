#!/bin/bash
# Скрипт для проверки корректности настройки Ansible проекта

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ANSIBLE_DIR="$(dirname "$SCRIPT_DIR")"

cd "$ANSIBLE_DIR"

echo "🔍 Проверка настройки Ansible для TG-RSS"
echo "========================================="

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

check_result() {
    local result=$1
    local message=$2
    if [ $result -eq 0 ]; then
        echo -e "${GREEN}✅${NC} $message"
        return 0
    else
        echo -e "${RED}❌${NC} $message"
        return 1
    fi
}

warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

error_count=0
warning_count=0

# 1. Проверка наличия Ansible
echo ""
echo "1. 🔧 Проверка Ansible..."
if command -v ansible &> /dev/null; then
    ansible_version=$(ansible --version | head -1 | cut -d' ' -f2)
    check_result 0 "Ansible установлен: $ansible_version"
else
    check_result 1 "Ansible не установлен"
    error_count=$((error_count + 1))
fi

# 2. Проверка ansible.cfg
echo ""
echo "2. ⚙️ Проверка конфигурации..."
if [ -f "ansible.cfg" ]; then
    check_result 0 "ansible.cfg найден"

    # Проверка inventory
    inventory_path=$(grep "^inventory" ansible.cfg | cut -d'=' -f2 | xargs)
    if [ -f "$inventory_path" ]; then
        check_result 0 "Inventory файл найден: $inventory_path"
    else
        warning "Inventory файл не найден: $inventory_path"
        warning_count=$((warning_count + 1))
    fi

    # Проверка roles_path
    if grep -q "^roles_path" ansible.cfg; then
        check_result 0 "roles_path настроен"
    else
        warning "roles_path не настроен в ansible.cfg"
        warning_count=$((warning_count + 1))
    fi
else
    check_result 1 "ansible.cfg не найден"
    error_count=$((error_count + 1))
fi

# 3. Проверка структуры директорий
echo ""
echo "3. 📁 Проверка структуры проекта..."
dirs=("inventory" "playbooks" "roles" "scripts")
for dir in "${dirs[@]}"; do
    if [ -d "$dir" ]; then
        check_result 0 "Директория $dir существует"
    else
        check_result 1 "Директория $dir отсутствует"
        error_count=$((error_count + 1))
    fi
done

# 4. Проверка inventory
echo ""
echo "4. 📋 Проверка inventory..."
if [ -f "inventory/hosts.ini" ]; then
    check_result 0 "hosts.ini найден"

    # Проверка наличия групп
    if grep -q "\[tg_rss_servers\]" inventory/hosts.ini; then
        check_result 0 "Группа tg_rss_servers найдена"
    else
        warning "Группа tg_rss_servers не найдена в inventory"
        warning_count=$((warning_count + 1))
    fi
else
    warning "hosts.ini не найден (нормально для CI/CD)"
    warning_count=$((warning_count + 1))
fi

# 5. Проверка ролей
echo ""
echo "5. 🎭 Проверка ролей Ansible..."
required_roles=("system_optimization" "docker_setup" "monitoring_setup" "security_hardening")
for role in "${required_roles[@]}"; do
    if [ -d "roles/$role" ] || [ -d "playbooks/roles/$role" ] || [ -d "../roles/$role" ]; then
        check_result 0 "Роль $role найдена"
    else
        check_result 1 "Роль $role не найдена"
        error_count=$((error_count + 1))
    fi
done

# 6. Проверка плейбуков
echo ""
echo "6. 📜 Проверка плейбуков..."
playbooks=("server-setup.yml")
for playbook in "${playbooks[@]}"; do
    if [ -f "playbooks/$playbook" ]; then
        check_result 0 "Плейбук $playbook найден"

        # Синтаксическая проверка
        if ansible-playbook --syntax-check "playbooks/$playbook" &> /dev/null; then
            check_result 0 "Синтаксис $playbook корректный"
        else
            check_result 1 "Синтаксическая ошибка в $playbook"
            error_count=$((error_count + 1))
        fi
    else
        check_result 1 "Плейбук $playbook не найден"
        error_count=$((error_count + 1))
    fi
done

# 7. Проверка Ansible Vault
echo ""
echo "7. 🔐 Проверка Ansible Vault..."
if [ -f ".vault_password" ]; then
    check_result 0 "Vault password файл найден"

    # Проверка прав доступа
    vault_perms=$(stat -c %a .vault_password 2>/dev/null || stat -f %A .vault_password)
    if [ "$vault_perms" = "600" ] || [ "$vault_perms" = "rw-------" ]; then
        check_result 0 "Права доступа к vault password корректные"
    else
        check_result 1 "Некорректные права доступа к vault password (должно быть 600)"
        error_count=$((error_count + 1))
    fi

    # Проверка vault файла
    if [ -f "inventory/group_vars/all/vault.yml" ]; then
        check_result 0 "Vault файл найден"

        # Проверка что файл зашифрован
        if head -1 inventory/group_vars/all/vault.yml | grep -q "ANSIBLE_VAULT"; then
            check_result 0 "Vault файл зашифрован"
        else
            warning "Vault файл не зашифрован"
            warning_count=$((warning_count + 1))
        fi

        # Проверка расшифровки
        if ansible-vault view inventory/group_vars/all/vault.yml --vault-password-file .vault_password &> /dev/null; then
            check_result 0 "Vault файл корректно расшифровывается"
        else
            check_result 1 "Ошибка расшифровки vault файла"
            error_count=$((error_count + 1))
        fi
    else
        warning "Vault файл не найден"
        warning_count=$((warning_count + 1))
    fi
else
    warning "Vault password файл не найден (создайте с помощью ./scripts/vault_setup.sh)"
    warning_count=$((warning_count + 1))
fi

# 8. Проверка скриптов
echo ""
echo "8. 📜 Проверка скриптов..."
scripts=("scripts/vault_setup.sh" "scripts/validate_setup.sh")
for script in "${scripts[@]}"; do
    if [ -f "$script" ]; then
        check_result 0 "Скрипт $script найден"

        # Проверка прав выполнения
        if [ -x "$script" ]; then
            check_result 0 "Скрипт $script исполняемый"
        else
            warning "Скрипт $script не имеет прав выполнения"
            warning_count=$((warning_count + 1))
        fi
    else
        check_result 1 "Скрипт $script не найден"
        error_count=$((error_count + 1))
    fi
done

# 9. Проверка зависимостей
echo ""
echo "9. 📦 Проверка зависимостей..."
if python3 -c "import yaml, jinja2" 2>/dev/null; then
    check_result 0 "Python зависимости для Ansible установлены"
else
    check_result 1 "Python зависимости для Ansible не установлены"
    error_count=$((error_count + 1))
fi

# 10. Проверка CI/CD интеграции
echo ""
echo "10. 🔄 Проверка CI/CD интеграции..."
if [ -f "../.github/workflows/ci-cd.yml" ]; then
    check_result 0 "GitHub Actions workflow найден"

    # Проверка использования Ansible в CI/CD
    if grep -q "ansible-playbook" ../.github/workflows/ci-cd.yml; then
        check_result 0 "Ansible используется в CI/CD"
    else
        warning "Ansible не найден в CI/CD workflow"
        warning_count=$((warning_count + 1))
    fi
else
    warning "GitHub Actions workflow не найден"
    warning_count=$((warning_count + 1))
fi

# Итоговый отчет
echo ""
echo "========================================="
echo "📊 РЕЗУЛЬТАТЫ ПРОВЕРКИ"
echo "========================================="

if [ $error_count -eq 0 ]; then
    echo -e "${GREEN}✅ Настройка корректна!${NC}"
    echo "   • $warning_count предупреждений"
    echo ""
    echo "🚀 Готов к развертыванию:"
    echo "   make deploy"
else
    echo -e "${RED}❌ Найдены ошибки в настройке!${NC}"
    echo "   • $error_count ошибок"
    echo "   • $warning_count предупреждений"
    echo ""
    echo "🔧 Исправьте ошибки и запустите проверку снова:"
    echo "   ./scripts/validate_setup.sh"
    exit 1
fi

if [ $warning_count -gt 0 ]; then
    echo ""
    echo -e "${YELLOW}⚠️ Рекомендации:${NC}"
    echo "   • Настройте Ansible Vault: ./scripts/vault_setup.sh"
    echo "   • Проверьте права доступа к файлам"
    echo "   • Обновите inventory для вашего окружения"
fi