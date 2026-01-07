#!/bin/bash

# Скрипт быстрого исправления SSH проблемы для CI/CD
# Запустите этот скрипт на вашей локальной машине

set -e

echo "🔧 Исправление SSH настройки для TG-RSS CI/CD"
echo

# Проверяем, что у нас есть необходимые инструменты
command -v ssh-keygen >/dev/null 2>&1 || { echo "❌ ssh-keygen не найден. Установите OpenSSH."; exit 1; }

# Запрашиваем информацию о сервере
read -p "Введите IP/домен сервера: " SERVER_HOST
read -p "Введите имя пользователя на сервере: " SERVER_USER
read -p "Введите порт SSH (по умолчанию 22): " SERVER_PORT
SERVER_PORT=${SERVER_PORT:-22}

echo
echo "📋 Конфигурация:"
echo "   Сервер: $SERVER_HOST"
echo "   Пользователь: $SERVER_USER"
echo "   Порт: $SERVER_PORT"
echo

# Генерируем новый SSH ключ
KEY_NAME="github_ci_$(date +%Y%m%d_%H%M%S)"
KEY_PATH="$HOME/.ssh/$KEY_NAME"

echo "🔑 Генерируем новый SSH ключ..."
ssh-keygen -t ed25519 -C "tg-rss-ci@github.com" -f "$KEY_PATH" -N ""

echo "✅ SSH ключ сгенерирован: $KEY_PATH"
echo

# Проверяем ключ
echo "🔍 Проверяем сгенерированный ключ..."
if ssh-keygen -l -f "$KEY_PATH" >/dev/null 2>&1; then
    echo "✅ Ключ валиден"
else
    echo "❌ Ключ поврежден"
    exit 1
fi

echo

# Добавляем ключ на сервер
echo "🚀 Добавляем публичный ключ на сервер..."
echo "   Выполняется: ssh-copy-id -i ${KEY_PATH}.pub -p $SERVER_PORT ${SERVER_USER}@${SERVER_HOST}"

if ssh-copy-id -i "${KEY_PATH}.pub" -p "$SERVER_PORT" "${SERVER_USER}@${SERVER_HOST}" 2>/dev/null; then
    echo "✅ Публичный ключ добавлен на сервер"
else
    echo "⚠️  ssh-copy-id не удался. Добавьте ключ вручную:"
    echo
    echo "   Скопируйте эту строку на сервер в ~/.ssh/authorized_keys:"
    cat "${KEY_PATH}.pub"
    echo
    echo "   Команда на сервере:"
    echo "   echo '$(cat ${KEY_PATH}.pub)' >> ~/.ssh/authorized_keys"
    echo
    read -p "Нажмите Enter после добавления ключа на сервер..."
fi

# Тестируем подключение
echo
echo "🔗 Тестируем SSH подключение..."
if ssh -i "$KEY_PATH" -p "$SERVER_PORT" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "${SERVER_USER}@${SERVER_HOST}" "echo 'SSH подключение работает!'" 2>/dev/null; then
    echo "✅ SSH подключение успешно!"
else
    echo "❌ SSH подключение не работает"
    echo "   Проверьте настройки сервера и повторите"
    exit 1
fi

echo
echo "📋 GitHub Secrets для настройки:"
echo
echo "1. SERVER_HOST: $SERVER_HOST"
echo "2. SERVER_USER: $SERVER_USER"
echo "3. SERVER_PORT: $SERVER_PORT"
echo "4. SERVER_SSH_KEY:"
echo

# Показываем приватный ключ для копирования
echo "-----COPY EVERYTHING BELOW THIS LINE-----"
cat "$KEY_PATH"
echo "-----COPY EVERYTHING ABOVE THIS LINE-----"

echo
echo "📝 Инструкция:"
echo "1. Скопируйте приватный ключ выше"
echo "2. Перейдите в GitHub: Settings → Secrets and variables → Actions"
echo "3. Обновите или создайте секреты:"
echo "   - SERVER_HOST: $SERVER_HOST"
echo "   - SERVER_USER: $SERVER_USER"
echo "   - SERVER_PORT: $SERVER_PORT"
echo "   - SERVER_SSH_KEY: [вставьте приватный ключ]"
echo
echo "4. Запустите CI/CD pipeline в GitHub Actions"
echo
echo "🎉 Настройка SSH завершена!"
echo
echo "💡 Полезные команды:"
echo "   Тестирование ключа: ./test-ssh-key.sh $KEY_PATH"
echo "   Диагностика: запустите 'SSH Diagnostic' в GitHub Actions"
