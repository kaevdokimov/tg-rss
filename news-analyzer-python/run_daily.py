#!/usr/bin/env python3
"""
Главный скрипт для ежедневного анализа новостей.

Использование:
    python run_daily.py

Или через cron:
    0 0 * * * cd /path/to/news-analyzer-python && /path/to/venv/bin/python run_daily.py
"""

import sys
from pathlib import Path

# Добавляем src в путь для импортов
sys.path.insert(0, str(Path(__file__).parent / "src"))

from datetime import datetime

from src.config import load_settings
from src.db import Database
from src.fetcher import NewsFetcher
from src.preprocessor import TextCleaner
from src.analyzer import TextVectorizer, NewsClusterer
from src.narrative import NarrativeBuilder
from src.reporter import ReportFormatter, SummaryGenerator
from src.utils import setup_logger, ensure_dir, get_logger


def main():
    """Основная функция запуска анализа."""
    try:
        # Загружаем конфигурацию
        logger = setup_logger(
            log_level="INFO",
            log_dir=Path("./storage/logs"),
            log_to_file=True
        )
        logger.info("=" * 60)
        logger.info("Запуск анализа новостей")
        logger.info("=" * 60)
        
        settings = load_settings()
        
        # Создаем необходимые директории
        ensure_dir(settings.reports_dir)
        ensure_dir(settings.logs_dir)
        
        # Подключаемся к БД
        logger.info("Подключение к базе данных...")
        db = Database(settings.get_db_connection_string())
        db.connect()
        
        try:
            # Тестируем подключение
            if not db.test_connection():
                logger.error("Не удалось подключиться к БД")
                sys.exit(1)
            
            # Получаем новости
            logger.info("Получение новостей из БД...")
            fetcher = NewsFetcher(db, settings)
            news_items = fetcher.fetch_recent_news()
            
            if not news_items:
                logger.warning("Новости не найдены. Анализ завершен.")
                return
            
            logger.info(f"Получено {len(news_items)} новостей для анализа")
            
            # 1. Предобработка текста
            logger.info("Предобработка текста...")
            cleaner = TextCleaner(
                stopwords_extra=settings.stopwords_extra,
                min_word_length=settings.min_word_length,
                max_word_length=settings.max_word_length
            )
            
            processed_texts = []
            for item in news_items:
                if settings.use_titles_only:
                    text = item.title
                else:
                    text = f"{item.title} {item.description}"
                processed = cleaner.preprocess(text)
                processed_texts.append(processed)
            
            logger.info(f"Предобработано {len(processed_texts)} текстов")
            
            # 2. Векторизация
            logger.info("Векторизация текстов...")
            vectorizer = TextVectorizer(
                max_features=settings.max_features,
                min_df=settings.min_df,
                max_df=settings.max_df
            )
            vectors = vectorizer.fit_transform(processed_texts)
            
            # 3. Кластеризация
            logger.info("Кластеризация новостей...")
            clusterer = NewsClusterer(
                min_cluster_size=settings.cluster_min_size,
                min_samples=settings.cluster_min_samples,
                metric=settings.cluster_metric
            )
            labels = clusterer.fit_predict(vectors)
            
            # 4. Построение нарративов
            logger.info("Построение нарративов...")
            narrative_builder = NarrativeBuilder()
            narratives = narrative_builder.build_narratives(
                news_items=news_items,
                labels=labels,
                vectorizer=vectorizer,
                top_n=settings.top_narratives
            )
            
            # 5. Генерация отчета
            logger.info("Генерация отчета...")
            analysis_date = datetime.now()
            
            # Сохраняем JSON отчет
            formatter = ReportFormatter(
                reports_dir=settings.reports_dir,
                date_format=settings.date_format
            )
            report_path = formatter.save_report(
                narratives=narratives,
                total_news=len(news_items),
                analysis_date=analysis_date
            )
            
            # Генерируем текстовое резюме
            summary_gen = SummaryGenerator()
            summary = summary_gen.generate(
                narratives=narratives,
                total_news=len(news_items),
                analysis_date=analysis_date
            )
            
            # Выводим резюме в консоль и логи
            logger.info("\n" + summary)
            
            logger.info("=" * 60)
            logger.info("Анализ завершен успешно")
            logger.info(f"Отчет сохранен: {report_path}")
            logger.info("=" * 60)
            
        finally:
            db.disconnect()
            
    except FileNotFoundError as e:
        print(f"Ошибка конфигурации: {e}", file=sys.stderr)
        print("Убедитесь, что файлы .env и config.yaml существуют и настроены.", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        logger = get_logger()
        logger.exception(f"Критическая ошибка при выполнении анализа: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
