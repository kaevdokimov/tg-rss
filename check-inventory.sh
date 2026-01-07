#!/bin/bash
# Скрипт для проверки inventory файла локально

echo "🔍 Проверка inventory файла..."
echo

if [ ! -f "ansible/inventory/hosts.ini" ]; then
    echo "❌ Inventory файл не найден: ansible/inventory/hosts.ini"
    exit 1
fi

echo "📋 Содержимое inventory файла:"
echo "----------------------------------------"
cat ansible/inventory/hosts.ini
echo "----------------------------------------"
echo

echo "🔧 Проверка ansible.cfg:"
if [ -f "ansible/ansible.cfg" ]; then
    echo "✅ ansible.cfg найден"
    grep -E "(roles_path|inventory)" ansible/ansible.cfg || echo "Настройки путей не найдены"
else
    echo "❌ ansible.cfg не найден"
fi
echo

echo "📁 Структура ролей:"
if [ -d "ansible/playbooks/roles" ]; then
    echo "✅ Директория ролей найдена"
    ls -la ansible/playbooks/roles/ | grep -E "^d" | wc -l | xargs echo "Найдено ролей: "
    ls ansible/playbooks/roles/ | head -5
else
    echo "❌ Директория ролей не найдена"
fi
echo

echo "✅ Проверка завершена"
