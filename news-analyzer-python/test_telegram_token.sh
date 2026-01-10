#!/bin/bash
# Тест токена Telegram бота
TOKEN=${TELEGRAM_SIGNAL_API_KEY:-''}
if [ -z "$TOKEN" ]; then
  echo 'TELEGRAM_SIGNAL_API_KEY не установлен'
  exit 1
fi

echo "Тестирование токена: ${TOKEN:0:20}..."
curl -s "https://api.telegram.org/bot$TOKEN/getMe" | python3 -m json.tool
