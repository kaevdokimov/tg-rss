# Краткая инструкция по деплою

## Быстрый старт

1. **Убедитесь, что в корне проекта есть `.env` файл** с настройками БД (используется общий для Go и Python сервисов)

2. **Запустите все сервисы:**
   ```bash
   make up
   ```

3. **Проверьте работу:**
   ```bash
   # Логи Python-сервиса
   make analyzer-logs
   
   # Тест подключения
   make analyzer-test
   
   # Запуск анализа вручную
   make analyzer-run
   ```

## Настройка расписания

По умолчанию анализ запускается каждый день в 00:00.

Для изменения расписания добавьте в `.env`:
```env
ANALYZER_CRON_SCHEDULE=0 12 * * *  # Запуск в 12:00
```

## Результаты

Отчеты сохраняются в Docker volume `analyzer_reports` и доступны через:
```bash
docker exec -it news-analyzer ls -la storage/reports/
```

## Полезные команды

```bash
# Все команды для работы с analyzer
make analyzer-logs      # Логи
make analyzer-console   # Консоль
make analyzer-run       # Запуск анализа
make analyzer-test      # Тест БД
make analyzer-build     # Пересборка
make analyzer-restart   # Перезапуск
```

Подробная документация: см. `DEPLOYMENT.md`
