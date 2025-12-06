# Build stage
FROM golang:1.25-alpine3.22 AS builder

# Устанавливаем необходимые зависимости для сборки
RUN apk add --no-cache gcc musl-dev

# Создаем непривилегированного пользователя
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы зависимостей для лучшего кэширования слоев
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение с оптимизациями
RUN CGO_ENABLED=1 go build \
    -ldflags='-s -w -extldflags "-static"' \
    -o tg-rss-app \
    -trimpath

# Запускаем тесты
RUN go test -v ./...

# Runtime stage
FROM alpine:3.22

# Устанавливаем необходимые runtime зависимости
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем бинарный файл из builder stage
COPY --from=builder --chown=appuser:appgroup /app/tg-rss-app .

# Переключаемся на непривилегированного пользователя
USER appuser

# Добавляем health check (проверяем наличие процесса)
# Примечание: pgrep может отсутствовать в Alpine, поэтому используем ps
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ps aux | grep -v grep | grep tg-rss-app || exit 1

# Запускаем приложение
CMD ["./tg-rss-app"]

