#!/usr/bin/env python3
"""
Главный скрипт для ежедневного анализа новостей.

Использование:
    python run_daily.py

Или через cron:
    0 0 * * * cd /path/to/news-analyzer-python && /path/to/venv/bin/python run_daily.py
"""

import os
import sys
import warnings
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from functools import partial
import time

# Подавляем предупреждения SyntaxWarning из библиотеки hdbscan
# Это предупреждение связано с форматированием строк в самой библиотеке
warnings.filterwarnings("ignore", category=SyntaxWarning, module="hdbscan")

# Добавляем src в путь для импортов
sys.path.insert(0, str(Path(__file__).parent / "src"))

from datetime import datetime

from src.config import load_settings
from src.db import Database, User
from src.fetcher import NewsFetcher
from src.preprocessor import TextCleaner
from src.analyzer import TextVectorizer, NewsClusterer
from src.narrative import NarrativeBuilder
from src.reporter import ReportFormatter, SummaryGenerator, TelegramNotifier
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
            
            # Оптимизация: ограничиваем количество новостей для обработки
            # чтобы избежать перегрузки сервера
            max_news_limit = int(os.getenv("MAX_NEWS_LIMIT", "1000"))
            if len(news_items) > max_news_limit:
                logger.warning(
                    f"Обнаружено {len(news_items)} новостей, что превышает лимит {max_news_limit}. "
                    f"Обрабатываем только последние {max_news_limit} новостей."
                )
                news_items = news_items[:max_news_limit]
            
            # 1. Предобработка текста
            logger.info("Предобработка текста...")
            cleaner = TextCleaner(
                stopwords_extra=settings.stopwords_extra,
                min_word_length=settings.min_word_length,
                max_word_length=settings.max_word_length
            )
            
            # Оптимизация: параллельная обработка текста для ускорения
            def preprocess_item(item, use_titles_only):
                """Функция для предобработки одного элемента."""
                if use_titles_only:
                    text = item.title
                else:
                    text = f"{item.title} {item.description}"
                return cleaner.preprocess(text)
            
            # Используем параллельную обработку для больших объемов данных
            # Для малых объемов последовательная обработка быстрее из-за накладных расходов
            # Используем ThreadPoolExecutor вместо ProcessPoolExecutor для избежания проблем с сериализацией
            if len(news_items) > 100:
                logger.info("Используется параллельная обработка текста...")
                max_workers = min(4, os.cpu_count() or 1)  # Ограничиваем количество потоков
                preprocess_func = partial(preprocess_item, use_titles_only=settings.use_titles_only)
                
                processed_texts = []
                try:
                    with ThreadPoolExecutor(max_workers=max_workers) as executor:
                        # Отправляем задачи и сохраняем соответствие индексов
                        future_to_index = {
                            executor.submit(preprocess_func, item): idx 
                            for idx, item in enumerate(news_items)
                        }
                        # Создаем список результатов нужного размера
                        processed_texts = [None] * len(news_items)
                        # Собираем результаты в правильном порядке
                        for future in as_completed(future_to_index):
                            idx = future_to_index[future]
                            try:
                                processed_texts[idx] = future.result()
                            except Exception as e:
                                logger.error(f"Ошибка при предобработке текста для элемента {idx}: {e}")
                                # Fallback: обрабатываем последовательно при ошибке
                                processed_texts = []
                                for item in news_items:
                                    if settings.use_titles_only:
                                        text = item.title
                                    else:
                                        text = f"{item.title} {item.description}"
                                    processed_texts.append(cleaner.preprocess(text))
                                break
                except Exception as e:
                    logger.warning(f"Ошибка при параллельной обработке, переключаемся на последовательную: {e}")
                    # Fallback: последовательная обработка
                    processed_texts = []
                    for item in news_items:
                        if settings.use_titles_only:
                            text = item.title
                        else:
                            text = f"{item.title} {item.description}"
                        processed_texts.append(cleaner.preprocess(text))
            else:
                # Последовательная обработка для малых объемов
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
            
            # Сохраняем результат анализа в БД для исторических данных
            logger.info("Сохранение результата анализа в БД...")
            try:
                # Убеждаемся, что таблица существует
                db.ensure_analysis_table_exists()
                
                # Сохраняем результат
                analysis_id = db.save_analysis_result(
                    analysis_date=analysis_date,
                    total_news=len(news_items),
                    narratives=narratives
                )
                logger.info(f"Результат анализа сохранен в БД с ID: {analysis_id}")
            except Exception as e:
                logger.error(f"Ошибка при сохранении результата анализа в БД: {e}")
                logger.warning("Продолжаем работу, отчет сохранен в файл")
            
            # Генерируем текстовое резюме
            summary_gen = SummaryGenerator()
            summary = summary_gen.generate(
                narratives=narratives,
                total_news=len(news_items),
                analysis_date=analysis_date
            )
            
            # Выводим резюме в консоль и логи
            logger.info("\n" + summary)
            
            # 6. Отправка отчета в Telegram всем подписанным пользователям
            # Используется отдельный бот для отправки отчетов (TELEGRAM_SIGNAL_API_KEY)
            telegram_token = os.getenv("TELEGRAM_SIGNAL_API_KEY")
            
            if telegram_token:
                try:
                    logger.info("Получение списка пользователей из БД...")
                    users = db.get_all_users()
                    
                    if not users:
                        logger.warning("Пользователи не найдены в БД. Отчет не будет отправлен.")
                    else:
                        logger.info(f"Найдено {len(users)} пользователей. Отправка отчетов...")
                        
                        # Создаем notifier
                        notifier = TelegramNotifier(bot_token=telegram_token)
                        
                        # Получаем список chat_id
                        chat_ids = [user.chat_id for user in users]
                        
                        # Отправляем отчет всем пользователям
                        results = notifier.send_report_to_all(chat_ids, report_path)
                        
                        # Статистика отправки
                        successful = sum(1 for success in results.values() if success)
                        failed = len(results) - successful
                        
                        logger.info(f"Отправка завершена: успешно {successful}, ошибок {failed}")
                        
                        if failed > 0:
                            failed_chat_ids = [chat_id for chat_id, success in results.items() if not success]
                            logger.warning(f"Не удалось отправить {failed} пользователям: {failed_chat_ids[:10]}...")  # Показываем первые 10
                except Exception as e:
                    logger.error(f"Ошибка при отправке отчетов в Telegram: {e}")
            else:
                logger.info("Telegram не настроен (TELEGRAM_SIGNAL_API_KEY не установлен)")
            
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
