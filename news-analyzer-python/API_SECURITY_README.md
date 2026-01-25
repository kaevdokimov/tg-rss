# API Security - Защита документации FastAPI

## 🔒 Обзор изменений

В FastAPI приложение добавлена защита документации (`/docs` и `/redoc`) с помощью HTTP Basic Authentication.

## ✅ Исправленные проблемы

### 1. Ошибка Permission denied: 'environments'

**Проблема:** При обращении к `/health` возникала ошибка:
```json
{
  "detail": "Service unhealthy: [Errno 13] Permission denied: 'environments'"
}
```

**Причина:** Код пытался читать директорию `environments` относительно текущей рабочей директории, а не относительно модуля конфигурации.

**Решение:** В файле `src/config/validator.py` изменен путь к директории `environments`:

```python
# Было:
environments_dir = self.base_config_path.parent / "environments"

# Стало:
config_module_dir = Path(__file__).parent.resolve()
environments_dir = config_module_dir / "environments"
```

### 2. Защита документации паролем

**Что добавлено:**
- HTTP Basic Authentication для `/docs`, `/redoc` и `/openapi.json`
- Учетные данные настраиваются через переменные окружения
- Правильное отображение ошибок при неверных учетных данных

## 📝 Конфигурация

### Переменные окружения

Добавьте в `.env` файл:

```bash
# API Security
# Учетные данные для доступа к /docs и /redoc
NEWS_ANALYZER_ADMIN=admin
NEWS_ANALYZER_PASSWORD=your_secure_password_here
```

**Важно:** Используйте надежный пароль в production окружении!

### Для Docker / GitHub Secrets

В production окружении передавайте учетные данные через secrets:

```yaml
# docker-compose.yml
environment:
  - NEWS_ANALYZER_ADMIN=${NEWS_ANALYZER_ADMIN}
  - NEWS_ANALYZER_PASSWORD=${NEWS_ANALYZER_PASSWORD}
```

## 🚀 Использование

### Доступ к документации

#### Через браузер

1. Откройте http://localhost:8000/docs
2. Браузер запросит логин и пароль
3. Введите учетные данные из `.env`

#### Через curl

```bash
# С авторизацией
curl -u admin:your_password http://localhost:8000/docs

# Или с base64 кодированием
curl -H "Authorization: Basic $(echo -n 'admin:your_password' | base64)" http://localhost:8000/docs
```

#### Через Python requests

```python
import requests
from requests.auth import HTTPBasicAuth

response = requests.get(
    "http://localhost:8000/docs",
    auth=HTTPBasicAuth('admin', 'your_password')
)
```

### Открытые endpoints (без авторизации)

Следующие endpoints доступны без авторизации:

- `GET /` - главная страница API
- `GET /health` - health check
- `GET /status` - статус сервиса
- `GET /metrics` - Prometheus метрики
- `GET /diagnostics` - диагностика компонентов
- `POST /analyze` - запуск анализа

### Защищенные endpoints (требуют авторизации)

- `GET /docs` - Swagger UI документация
- `GET /redoc` - ReDoc документация
- `GET /openapi.json` - OpenAPI схема

## 🔧 Технические детали

### Файлы с изменениями

1. **src/monitoring/api.py**
   - Добавлен импорт `python-dotenv` для загрузки `.env` при старте
   - Отключена автоматическая генерация документации FastAPI
   - Добавлен `HTTPBasic` security scheme
   - Создана функция `verify_credentials()` для проверки учетных данных
   - Добавлены защищенные endpoints для документации

2. **src/config/validator.py**
   - Исправлен путь к директории `environments` (используется `Path(__file__).parent`)
   - Улучшена валидация конфигурации БД (пропуск переменных окружения)
   - Улучшена валидация векторизации (поддержка разных типов min_df/max_df)

3. **env.example**
   - Добавлены переменные `NEWS_ANALYZER_ADMIN` и `NEWS_ANALYZER_PASSWORD`

### Безопасность

- Используется `secrets.compare_digest()` для защиты от timing attacks
- Пароли передаются через переменные окружения (не хардкодятся в коде)
- HTTP Basic Auth требует HTTPS в production для безопасной передачи credentials
- Дефолтные значения (`admin`/`changeme`) должны быть изменены в production

## 🔄 Обновление существующих deployment

1. Обновите `.env` файл:
   ```bash
   echo "NEWS_ANALYZER_ADMIN=your_admin_username" >> .env
   echo "NEWS_ANALYZER_PASSWORD=your_secure_password" >> .env
   ```

2. Для Docker - передайте secrets:
   ```bash
   docker-compose up -d --force-recreate news-analyzer
   ```

3. Для GitHub Actions - добавьте secrets в репозитории:
   - `NEWS_ANALYZER_ADMIN`
   - `NEWS_ANALYZER_PASSWORD`

## 📊 Тестирование

```bash
# 1. Запуск сервера
cd /path/to/news-analyzer-python
source venv/bin/activate
python -m uvicorn src.monitoring.api:app --host 0.0.0.0 --port 8000 --reload

# 2. Проверка health (без авторизации)
curl http://localhost:8000/health

# 3. Проверка /docs без пароля (должен вернуть 401)
curl http://localhost:8000/docs
# {"detail":"Not authenticated"}

# 4. Проверка /docs с неправильным паролем (должен вернуть 401)
curl -u admin:wrong http://localhost:8000/docs
# {"detail":"Неверные учетные данные"}

# 5. Проверка /docs с правильным паролем (должен вернуть HTML)
curl -u admin:your_password http://localhost:8000/docs
# <!DOCTYPE html>...
```

## 🎯 Production рекомендации

1. **Используйте HTTPS**
   - HTTP Basic Auth передает credentials в base64 (не зашифровано)
   - HTTPS обязателен для безопасной передачи

2. **Сильные пароли**
   - Минимум 16 символов
   - Используйте генератор паролей

3. **Ограничьте доступ**
   - Используйте firewall для ограничения доступа к API
   - Настройте IP whitelist если возможно

4. **Мониторинг**
   - Логируйте неудачные попытки авторизации
   - Настройте alerts на подозрительную активность

5. **Secrets Management**
   - Используйте AWS Secrets Manager, Azure Key Vault, или HashiCorp Vault
   - Не коммитьте `.env` файл в git

## 📚 Дополнительные ресурсы

- [FastAPI Security](https://fastapi.tiangolo.com/tutorial/security/)
- [HTTP Basic Authentication](https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
