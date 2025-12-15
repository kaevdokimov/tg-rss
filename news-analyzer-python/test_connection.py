#!/usr/bin/env python3
"""
Тестовый скрипт для проверки подключения к БД и получения новостей.

Использование:
    python test_connection.py
"""

import sys
from pathlib import Path

# Добавляем src в путь для импортов
sys.path.insert(0, str(Path(__file__).parent / "src"))

from src.config import load_settings
from src.db import Database
from src.fetcher import NewsFetcher
from src.utils import setup_logger

def main():
    """Тестирует подключение к БД и получение новостей."""
    try:
        logger = setup_logger(log_level="INFO", log_to_file=False)
        logger.info("Тест подключения к БД")
        logger.info("=" * 60)
        
        # Загружаем конфигурацию
        settings = load_settings()
        logger.info(f"Настройки загружены:")
        logger.info(f"  БД: {settings.db_host}:{settings.db_port}/{settings.db_name}")
        logger.info(f"  Таблица: {settings.db_table}")
        logger.info(f"  Окно анализа: {settings.time_window_hours} часов")
        
        # Подключаемся к БД
        logger.info("\nПодключение к БД...")
        db = Database(settings.get_db_connection_string())
        
        try:
            # Тестируем подключение
            if not db.test_connection():
                logger.error("❌ Не удалось подключиться к БД")
                sys.exit(1)
            
            logger.info("✅ Подключение к БД успешно")
            
            # Получаем новости
            logger.info("\nПолучение новостей...")
            fetcher = NewsFetcher(db, settings)
            news_items = fetcher.fetch_recent_news()
            
            if not news_items:
                logger.warning("⚠️  Новости не найдены за указанный период")
            else:
                logger.info(f"✅ Получено {len(news_items)} новостей")
                logger.info("\nПримеры новостей:")
                for i, item in enumerate(news_items[:3], 1):
                    logger.info(f"\n{i}. {item.title[:80]}...")
                    logger.info(f"   Опубликовано: {item.published_at}")
                    logger.info(f"   Ссылка: {item.link[:60]}...")
            
            logger.info("\n" + "=" * 60)
            logger.info("✅ Тест завершен успешно")
            
        finally:
            db.disconnect()
            
    except FileNotFoundError as e:
        print(f"❌ Ошибка конфигурации: {e}", file=sys.stderr)
        print("Убедитесь, что файлы .env и config.yaml существуют и настроены.", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        logger.exception(f"❌ Ошибка: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
