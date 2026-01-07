#!/bin/bash

# Скрипт быстрого развертывания TG-RSS на оптимизированный сервер
set -e

echo "🚀 Начинаем развертывание TG-RSS..."

# Проверка наличия ansible
if ! command -v ansible &> /dev/null; then
    echo "❌ Ansible не установлен. Установите его:"
    echo "pip install ansible"
    exit 1
fi

# Проверка переменных окружения
if [ -z "$SERVER_HOST" ] || [ -z "$SERVER_USER" ]; then
    echo "❌ Установите переменные окружения:"
    echo "export SERVER_HOST=your-server-ip"
    echo "export SERVER_USER=your-username"
    echo "export SSH_PRIVATE_KEY='your-private-key'"
    exit 1
fi

echo "📋 Настройка Ansible инвентаря..."
mkdir -p ansible/inventory
cat > ansible/inventory/hosts.ini << EOF
[tg_rss_servers]
$SERVER_HOST ansible_host=$SERVER_HOST ansible_user=$SERVER_USER ansible_ssh_private_key_file=/tmp/deploy_key

[all:vars]
ansible_python_interpreter=/usr/bin/python3
EOF

# Сохранение SSH ключа
echo "$SSH_PRIVATE_KEY" > /tmp/deploy_key
chmod 600 /tmp/deploy_key

echo "🔧 Запуск Ansible playbook..."
cd ansible
ansible-playbook -i inventory/hosts.ini playbooks/server-setup.yml

echo "📦 Развертывание приложения..."
ssh -i /tmp/deploy_key -o StrictHostKeyChecking=no $SERVER_USER@$SERVER_HOST << 'EOF'
    # Создание директории приложения
    sudo mkdir -p /opt/tg-rss
    sudo chown $USER:$USER /opt/tg-rss
    cd /opt/tg-rss

    # Клонирование репозитория (замените на ваш)
    git clone https://github.com/your-username/tg-rss.git .
    cp env.example .env

    echo "⚠️  Отредактируйте .env файл перед продолжением!"
    echo "📝 Нажмите Enter после редактирования .env файла"
    read

    # Запуск приложения
    docker-compose pull
    docker-compose up -d

    # Очистка
    docker system prune -f

    echo "✅ Приложение запущено!"
EOF

echo "🏥 Проверка здоровья..."
sleep 30

if curl -f http://$SERVER_HOST:8080/health > /dev/null 2>&1; then
    echo "✅ Health check прошел успешно!"
    echo "🌐 Приложение доступно по адресу: http://$SERVER_HOST:8080"
    echo "📊 Мониторинг: http://$SERVER_HOST:3000 (admin/admin)"
else
    echo "❌ Health check не прошел. Проверьте логи:"
    echo "ssh -i /tmp/deploy_key $SERVER_USER@$SERVER_HOST 'cd /opt/tg-rss && docker-compose logs'"
fi

# Очистка
rm -f /tmp/deploy_key

echo "🎉 Развертывание завершено!"
