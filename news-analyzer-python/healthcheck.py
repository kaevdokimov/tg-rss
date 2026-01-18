#!/usr/bin/env python3
"""Healthcheck script for Docker container."""

import sys
import os
from pathlib import Path

# Добавляем src в путь для импортов
sys.path.insert(0, '/app/src')

def main():
    """Основная функция healthcheck."""
    try:
        # Проверяем валидность конфигурации
        from src.config import validate_config_file
        is_valid, errors, warnings = validate_config_file()

        if not is_valid:
            print(f"Configuration validation failed: {errors}")
            return 1

        # Проверяем подключение к БД
        from src.config import load_settings
        from src.db import Database

        settings = load_settings()
        db = Database(settings.get_db_connection_string())

        if db.connect() and db.test_connection():
            db.disconnect()
            print("Health check passed")
            return 0
        else:
            print("Database connection failed")
            db.disconnect()
            return 1

    except ImportError as e:
        print(f"Import error: {e}")
        return 1
    except FileNotFoundError as e:
        print(f"Configuration file not found: {e}")
        return 1
    except Exception as e:
        print(f"Health check failed: {e}")
        return 1

if __name__ == "__main__":
    sys.exit(main())