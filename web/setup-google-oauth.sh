#!/bin/bash
# Быстрая настройка Google OAuth для CloudSecGuard
# Запустить: bash setup-google-oauth.sh

SERVER_IP="23.19.228.158"
REDIRECT_URL="http://${SERVER_IP}:3000/api/auth/google/callback"

echo ""
echo "════════════════════════════════════════════════════════"
echo "  CloudSecGuard — настройка Google OAuth"
echo "════════════════════════════════════════════════════════"
echo ""
echo "Шаг 1. Открой в браузере:"
echo "  https://console.cloud.google.com/apis/credentials"
echo ""
echo "Шаг 2. Создай OAuth Client ID:"
echo "  → Нажми [+ CREATE CREDENTIALS] → OAuth client ID"
echo "  → Application type: Web application"
echo "  → Name: CloudSecGuard"
echo ""
echo "Шаг 3. В разделе 'Authorized redirect URIs' добавь:"
echo "  ${REDIRECT_URL}"
echo ""
echo "Шаг 4. Скопируй Client ID и Client Secret, вставь ниже:"
echo ""

read -p "  GOOGLE_CLIENT_ID: " CLIENT_ID
read -p "  GOOGLE_CLIENT_SECRET: " CLIENT_SECRET

if [ -z "$CLIENT_ID" ] || [ -z "$CLIENT_SECRET" ]; then
  echo ""
  echo "Ошибка: введи оба значения"
  exit 1
fi

# Записываем в .env
cat > "$(dirname "$0")/.env" <<EOF
GOOGLE_CLIENT_ID=${CLIENT_ID}
GOOGLE_CLIENT_SECRET=${CLIENT_SECRET}
GOOGLE_REDIRECT_URL=${REDIRECT_URL}
EOF

echo ""
echo "✓ Файл .env обновлён"
echo ""
echo "Шаг 5. Перезапускаем backend..."
cd "$(dirname "$0")"
docker compose up -d --no-build backend

echo ""
echo "✓ Готово! Кнопка 'Sign in with Google' теперь активна."
echo "  Открой: http://${SERVER_IP}:3000/login"
echo ""
