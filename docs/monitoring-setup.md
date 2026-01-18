# Настройка мониторинга TG-RSS

## Prometheus Alerting Rules

Для настройки алертинга скопируйте файл `docs/prometheus-alerting-rules.yml` в директорию с правилами Prometheus:

```bash
# На сервере с Prometheus
sudo cp docs/prometheus-alerting-rules.yml /etc/prometheus/rules/
sudo systemctl reload prometheus
```

### Доступные алерты

#### 🔴 Critical Alerts (severity: critical)
- **ServiceDown**: Сервис полностью недоступен
- **HealthCheckFailing**: Health check endpoint возвращает ошибку
- **RSSPollsCompletelyDown**: RSS опросы полностью остановлены
- **TelegramMessagesCompletelyDown**: Отправка сообщений Telegram остановлена

#### 🟡 Warning Alerts (severity: warning)
- **RSSPollsFailing**: >50% RSS опросов завершаются ошибкой
- **TelegramMessagesFailing**: >30% сообщений Telegram завершаются ошибкой
- **CircuitBreakerOpen**: Circuit Breaker открыт для одного или нескольких сервисов
- **DatabaseConnectionsExhausted**: Исчерпаны соединения с БД
- **HTTPRequestsHighErrorRate**: >20% HTTP запросов завершаются ошибкой
- **ContentValidationHighErrorRate**: >50% контента не проходит валидацию
- **HighGoroutineCount**: >1000 активных горутин
- **HighMemoryUsage**: >80% использования памяти

#### 🔵 Info Alerts (severity: info)
- **CacheMissRateHigh**: >80% промахов кэша
- **TelegramRateLimitHit**: Возможные rate limits Telegram API

## Grafana Dashboard

Для импорта dashboard в Grafana:

1. Откройте Grafana UI
2. Перейдите в "Dashboards" → "Import"
3. Загрузите файл `docs/grafana-dashboard.json`
4. Выберите Prometheus как источник данных

### Метрики Dashboard

Dashboard включает следующие разделы:
- **RSS Processing**: Статистика опросов и обработки новостей
- **Telegram Messages**: Метрики отправки сообщений
- **Circuit Breaker Status**: Состояние защитных механизмов
- **Database Connections**: Мониторинг соединений с БД
- **HTTP Requests**: Статистика HTTP запросов
- **Application Health**: Общее состояние сервиса
- **Content Validation**: Метрики валидации контента

## Alertmanager Configuration

Пример конфигурации Alertmanager для отправки уведомлений:

```yaml
# /etc/prometheus/alertmanager.yml
global:
  smtp_smarthost: 'smtp.gmail.com:587'
  smtp_from: 'alerts@your-domain.com'
  smtp_auth_username: 'your-email@gmail.com'
  smtp_auth_password: 'your-app-password'

route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'telegram-notifications'

receivers:
  - name: 'telegram-notifications'
    telegram_configs:
      - bot_token: 'YOUR_BOT_TOKEN'
        chat_id: 'YOUR_CHAT_ID'
        api_url: 'https://api.telegram.org'
        parse_mode: 'HTML'

  - name: 'email-notifications'
    email_configs:
      - to: 'admin@your-domain.com'
        subject: '{{ .GroupLabels.alertname }}'
        body: |
          {{ range .Alerts }}
          Alert: {{ .Annotations.summary }}
          Description: {{ .Annotations.description }}
          Severity: {{ .Labels.severity }}
          {{ end }}
```

## Настройка Blackbox Exporter

Для мониторинга HTTP endpoints добавьте в `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'blackbox'
    metrics_path: /probe
    params:
      module: [http_2xx]
    static_configs:
      - targets:
        - http://localhost:8080/health
        - http://localhost:8080/metrics
        - http://localhost:8080/openapi.yaml
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: blackbox-exporter:9115
```

## Метрики производительности

### Целевые значения (SLO):

- **Availability**: 99.9% uptime
- **RSS Processing**: <5% ошибок опросов
- **Telegram Delivery**: <10% ошибок отправки
- **HTTP Response Time**: <500ms для 95% запросов
- **Database Connection Pool**: <80% utilization

### Мониторинг ресурсов:

- **CPU**: <70% средняя загрузка
- **Memory**: <80% использование
- **Disk I/O**: <1000 IOPS
- **Network**: <100Mbps трафик

## Troubleshooting

### Распространенные проблемы:

1. **Высокий процент ошибок RSS**
   - Проверьте сетевую связность
   - Проверьте доступность RSS источников
   - Проверьте Circuit Breaker статус

2. **Проблемы с Telegram API**
   - Проверьте rate limits
   - Проверьте токен бота
   - Проверьте сетевую связность с Telegram

3. **Проблемы с базой данных**
   - Проверьте connection pool
   - Проверьте дисковое пространство
   - Проверьте нагрузку на БД

4. **Высокое потребление ресурсов**
   - Проверьте количество горутин
   - Проверьте утечки памяти
   - Проверьте эффективность кэширования

## Логи и трассировка

Для дополнительной диагностики:

1. **Structured Logging**: Все логи в JSON формате для ELK stack
2. **Distributed Tracing**: OpenTelemetry для трассировки запросов (план)
3. **Log Rotation**: Автоматическая ротация логов (план)

## Автоматизация

Используйте Ansible плейбуки для автоматической настройки мониторинга:

```bash
ansible-playbook -i inventory.ini playbooks/monitoring-setup.yml
```