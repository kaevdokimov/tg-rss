"""FastAPI приложение для метрик и health checks."""

from fastapi import FastAPI, HTTPException
from fastapi.responses import PlainTextResponse
import prometheus_client
from prometheus_client import generate_latest, CONTENT_TYPE_LATEST
import psutil
import os
from datetime import datetime

from ..config import load_settings
from ..db import Database

app = FastAPI(title="News Analyzer API", version="1.0.0")


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    try:
        settings = load_settings()
        db = Database(settings.get_db_connection_string())

        if db.connect() and db.test_connection():
            db.disconnect()
            return {"status": "healthy", "timestamp": datetime.now().isoformat()}
        else:
            raise HTTPException(status_code=503, detail="Database unhealthy")
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

        return {
            "service": "news-analyzer",
            "version": "1.0.0",
            "status": "healthy" if db_status == "healthy" else "unhealthy",
            "database": db_status,
            "timestamp": datetime.now().isoformat(),
            "system": {
                "memory_used_percent": memory.percent,
                "disk_used_percent": disk.percent,
                "cpu_count": os.cpu_count()
            },
            "configuration": {
                "time_window_hours": settings.time_window_hours,
                "top_narratives": settings.top_narratives,
                "cluster_min_size": settings.cluster_min_size
            }
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Status check failed: {str(e)}")
    finally:
        try:
            db.disconnect()
        except:
            pass


@app.get("/")
async def root():
    """Корневой endpoint."""
    return {
        "service": "News Analyzer API",
        "version": "1.0.0",
        "endpoints": {
            "/health": "Health check",
            "/metrics": "Prometheus metrics",
            "/status": "Detailed service status"
        }
    }