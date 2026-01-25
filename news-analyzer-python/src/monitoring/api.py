"""FastAPI приложение для метрик и health checks."""

from fastapi import FastAPI, HTTPException, Depends
from fastapi import status as http_status
from fastapi.responses import PlainTextResponse
from fastapi.security import HTTPBasic, HTTPBasicCredentials
from fastapi.openapi.docs import get_swagger_ui_html, get_redoc_html
from fastapi.openapi.utils import get_openapi
import secrets
import prometheus_client
from prometheus_client import generate_latest, CONTENT_TYPE_LATEST
import psutil
import os
from datetime import datetime
from pathlib import Path
from dotenv import load_dotenv

from ..config import load_settings
from ..db import Database
from ..cache.redis_cache import RedisCache
from ..fetcher import NewsFetcher
import os
import subprocess  # nosec - used for local script execution
import threading

# Загружаем переменные окружения из .env при старте приложения
env_path = Path(__file__).parent.parent.parent / ".env"
if env_path.exists():
    load_dotenv(env_path)

# Отключаем автоматическую генерацию /docs и /redoc
app = FastAPI(
    title="News Analyzer API",
    version="1.0.0",
    docs_url=None,
    redoc_url=None,
    openapi_url=None
)

# Security для Basic Auth
security = HTTPBasic()


def verify_credentials(credentials: HTTPBasicCredentials = Depends(security)):
    """Проверка учетных данных для доступа к документации."""
    correct_username = os.getenv("NEWS_ANALYZER_ADMIN", "admin")
    correct_password = os.getenv("NEWS_ANALYZER_PASSWORD", "changeme")
    
    is_correct_username = secrets.compare_digest(
        credentials.username.encode("utf8"), correct_username.encode("utf8")
    )
    is_correct_password = secrets.compare_digest(
        credentials.password.encode("utf8"), correct_password.encode("utf8")
    )
    
    if not (is_correct_username and is_correct_password):
        raise HTTPException(
            status_code=http_status.HTTP_401_UNAUTHORIZED,
            detail="Неверные учетные данные",
            headers={"WWW-Authenticate": "Basic"},
        )
    return credentials.username


@app.get("/docs", include_in_schema=False)
async def get_documentation(username: str = Depends(verify_credentials)):
    """Swagger UI документация (защищена Basic Auth)."""
    return get_swagger_ui_html(openapi_url="/openapi.json", title="News Analyzer API - Docs")


@app.get("/redoc", include_in_schema=False)
async def get_redoc_documentation(username: str = Depends(verify_credentials)):
    """ReDoc документация (защищена Basic Auth)."""
    return get_redoc_html(openapi_url="/openapi.json", title="News Analyzer API - ReDoc")


@app.get("/openapi.json", include_in_schema=False)
async def get_open_api_endpoint(username: str = Depends(verify_credentials)):
    """OpenAPI схема (защищена Basic Auth)."""
    return get_openapi(title="News Analyzer API", version="1.0.0", routes=app.routes)


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    try:
        settings = load_settings()
        db = Database(settings.get_db_connection_string())

        health_status = {
            "status": "healthy",
            "timestamp": datetime.now().isoformat(),
            "checks": {}
        }

        # Проверка базы данных
        db_healthy = False
        if db.connect() and db.test_connection():
            db_healthy = True
            health_status["checks"]["database"] = "healthy"
        else:
            health_status["checks"]["database"] = "unhealthy"
            health_status["status"] = "unhealthy"

        db.disconnect()

        # Проверка Redis
        redis_healthy = False
        try:
            redis_host = os.getenv("REDIS_HOST", "redis")
            redis_port = int(os.getenv("REDIS_PORT", "6379"))
            redis_password = os.getenv("REDIS_PASSWORD")

            redis_cache = RedisCache(host=redis_host, port=redis_port, password=redis_password)
            if redis_cache.health_check():
                redis_healthy = True
                health_status["checks"]["redis"] = "healthy"
            else:
                health_status["checks"]["redis"] = "unhealthy"
        except Exception as e:
            health_status["checks"]["redis"] = f"error: {str(e)}"

        # Проверка NLTK данных
        nltk_healthy = False
        try:
            import nltk
            nltk.data.find('tokenizers/punkt')
            nltk.data.find('corpora/stopwords')
            nltk_healthy = True
            health_status["checks"]["nltk"] = "healthy"
        except Exception as e:
            health_status["checks"]["nltk"] = f"error: {str(e)}"

        # Общий статус
        if not (db_healthy and redis_healthy and nltk_healthy):
            health_status["status"] = "degraded"

        return health_status
    except Exception as e:
        raise HTTPException(status_code=503, detail=f"Service unhealthy: {str(e)}")


@app.get("/metrics")
async def metrics():
    """Prometheus метрики."""
    return PlainTextResponse(
        generate_latest(),
        media_type=CONTENT_TYPE_LATEST
    )


@app.get("/status")
async def status():
    """Подробный статус сервиса."""
    try:
        settings = load_settings()
        db = Database(settings.get_db_connection_string())

        db_status = "healthy" if db.connect() and db.test_connection() else "unhealthy"

        # Системные метрики
        memory = psutil.virtual_memory()
        disk = psutil.disk_usage('/')

        # Проверка Redis
        redis_status = "unknown"
        cache_stats = {}
        try:
            redis_host = os.getenv("REDIS_HOST", "redis")
            redis_port = int(os.getenv("REDIS_PORT", "6379"))
            redis_password = os.getenv("REDIS_PASSWORD")

            redis_cache = RedisCache(host=redis_host, port=redis_port, password=redis_password)
            if redis_cache.health_check():
                redis_status = "healthy"
                cache_stats = redis_cache.get_cache_stats()
            else:
                redis_status = "unhealthy"
        except Exception:
            # Не раскрываем детали исключения во внешнем API
            redis_status = "error"

        # Статистика новостей
        news_stats = {}
        try:
            stats = db.get_admin_stats()
            news_stats = {
                "total_users": stats.total_users,
                "total_news": stats.total_news,
                "total_sources": stats.total_sources
            }
        except Exception:
            # Не возвращаем текст исключения во внешнем ответе
            news_stats = {"error": "unavailable"}

        db.disconnect()

        return {
            "service": "news-analyzer",
            "version": "1.0.0",
            "status": "healthy" if db_status == "healthy" and redis_status == "healthy" else "degraded",
            "components": {
                "database": db_status,
                "redis": redis_status,
                "nltk": "healthy"  # Проверяется в /health
            },
            "timestamp": datetime.now().isoformat(),
            "system": {
                "memory_used_percent": round(memory.percent, 1),
                "memory_total_gb": round(memory.total / (1024**3), 1),
                "disk_used_percent": round(disk.percent, 1),
                "disk_total_gb": round(disk.total / (1024**3), 1),
                "cpu_count": os.cpu_count()
            },
            "configuration": {
                "time_window_hours": settings.time_window_hours,
                "top_narratives": settings.top_narratives,
                "cluster_min_size": settings.cluster_min_size,
                "max_features": settings.max_features
            },
            "cache": cache_stats,
            "news_stats": news_stats
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Status check failed: {str(e)}")


@app.get("/")
async def root():
    """Корневой endpoint."""
    return {
        "service": "News Analyzer API",
        "version": "1.0.0",
        "endpoints": {
            "/health": "Health check",
            "/metrics": "Prometheus metrics",
            "/status": "Detailed service status",
            "/diagnostics": "Component diagnostics",
            "/analyze": "Trigger manual analysis"
        }
    }


@app.get("/diagnostics")
async def diagnostics():
    """Диагностика всех компонентов."""
    try:
        diagnostics_result = {
            "timestamp": datetime.now().isoformat(),
            "diagnostics": {}
        }

        # Диагностика базы данных
        try:
            settings = load_settings()
            db = Database(settings.get_db_connection_string())
            if db.connect() and db.test_connection():
                # Получаем статистику новостей
                news_count = db.get_news_count_last_hours(hours=24, table_name=settings.db_table)
                diagnostics_result["diagnostics"]["database"] = {
                    "status": "healthy",
                    "news_count_24h": news_count,
                    "connection_string": f"{settings.db_host}:{settings.db_port}/{settings.db_name}"
                }
            else:
                diagnostics_result["diagnostics"]["database"] = {"status": "unhealthy"}
            db.disconnect()
        except Exception as e:
            diagnostics_result["diagnostics"]["database"] = {"status": "error", "error": str(e)}

        # Диагностика Redis
        try:
            redis_host = os.getenv("REDIS_HOST", "redis")
            redis_port = int(os.getenv("REDIS_PORT", "6379"))
            redis_password = os.getenv("REDIS_PASSWORD")

            redis_cache = RedisCache(host=redis_host, port=redis_port, password=redis_password)
            if redis_cache.health_check():
                stats = redis_cache.get_cache_stats()
                diagnostics_result["diagnostics"]["redis"] = {
                    "status": "healthy",
                    "host": f"{redis_host}:{redis_port}",
                    "cache_stats": stats
                }
            else:
                diagnostics_result["diagnostics"]["redis"] = {"status": "unhealthy"}
        except Exception as e:
            diagnostics_result["diagnostics"]["redis"] = {"status": "error", "error": str(e)}

        # Диагностика NLTK
        try:
            import nltk
            nltk.data.find('tokenizers/punkt')
            nltk.data.find('corpora/stopwords')
            diagnostics_result["diagnostics"]["nltk"] = {"status": "healthy"}
        except Exception as e:
            diagnostics_result["diagnostics"]["nltk"] = {"status": "error", "error": str(e)}

        # Диагностика ML компонентов
        try:
            import sklearn
            import hdbscan
            import numpy as np
            diagnostics_result["diagnostics"]["ml"] = {
                "status": "healthy",
                "sklearn_version": sklearn.__version__,
                "hdbscan_available": True,
                "numpy_available": True
            }
        except Exception as e:
            diagnostics_result["diagnostics"]["ml"] = {"status": "error", "error": str(e)}

        return diagnostics_result
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Diagnostics failed: {str(e)}")


@app.post("/analyze")
async def trigger_analysis():
    """Принудительный запуск анализа новостей."""
    try:
        # Запускаем анализ в отдельном потоке, чтобы не блокировать API
        def run_analysis():
            try:
                result = subprocess.run(  # nosec - local script execution
                    ["python3", "run_daily.py"],
                    cwd="/app",
                    capture_output=True,
                    text=True,
                    timeout=1800  # 30 минут таймаут
                )
                print(f"Analysis completed with code: {result.returncode}")
                if result.returncode != 0:
                    print(f"Analysis stderr: {result.stderr}")
            except subprocess.TimeoutExpired:
                print("Analysis timed out")
            except Exception as e:
                print(f"Analysis failed: {e}")

        thread = threading.Thread(target=run_analysis, daemon=True)
        thread.start()

        return {
            "status": "analysis_started",
            "message": "News analysis has been triggered in background",
            "timestamp": datetime.now().isoformat()
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to trigger analysis: {str(e)}")