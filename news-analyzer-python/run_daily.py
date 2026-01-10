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
# Также подавляем все SyntaxWarning для надежности
warnings.filterwarnings("ignore", category=SyntaxWarning)

# Добавляем src в путь для импортов
sys.path.insert(0, str(Path(__file__).parent / "src"))

# Настраиваем NLTK
import nltk
nltk_data_dir = os.getenv("NLTK_DATA", "/app/nltk_data")
if nltk_data_dir not in nltk.data.path:
    nltk.data.path.insert(0, nltk_data_dir)

# NLTK уже настроен выше

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

        # Проверяем и загружаем NLTK данные
        try:
            nltk.data.find('tokenizers/punkt')
            logger.info("✓ NLTK punkt данные найдены")
        except LookupError:
            logger.warning("✗ NLTK punkt данные не найдены, пытаемся загрузить...")
            try:
                nltk.download('punkt', quiet=True)
                logger.info("✓ NLTK punkt данные успешно загружены")
            except Exception as e:
                logger.error(f"✗ Не удалось загрузить NLTK punkt данные: {e}")
                logger.warning("Будет использоваться fallback токенизация")

        # Проверяем критически важные переменные окружения
        telegram_token = os.getenv("TELEGRAM_SIGNAL_API_KEY")
        if not telegram_token:
            logger.warning("⚠️ TELEGRAM_SIGNAL_API_KEY не установлен - отчеты не будут отправляться в Telegram")
        else:
            logger.info("✓ TELEGRAM_SIGNAL_API_KEY установлен")

        settings = load_settings()
        
        # Создаем необходимые директории
        ensure_dir(settings.reports_dir)
        ensure_dir(settings.logs_dir)
        
        # Проверяем настройки Telegram бота
        telegram_token = os.getenv("TELEGRAM_SIGNAL_API_KEY")
        if not telegram_token:
            logger.warning("TELEGRAM_SIGNAL_API_KEY не установлен - отчеты не будут отправляться в Telegram")
        else:
            logger.info("Telegram бот настроен для отправки отчетов")

        # Подключаемся к БД
        logger.info("Подключение к базе данных...")
        db = Database(settings.get_db_connection_string())
        db.connect()
        
        try:
            # Тестируем подключение
            if not db.test_connection():
                logger.error("Не удалось подключиться к БД")
                sys.exit(1)
            
            # ОПТИМИЗАЦИЯ: Проверяем количество новостей перед анализом
            logger.info("Проверка количества новостей...")
            min_news_threshold = int(os.getenv("ANALYZER_MIN_NEWS_THRESHOLD",
                                               os.getenv("MIN_NEWS_THRESHOLD", "10")))

            news_count = db.get_news_count_last_hours(
                hours=settings.time_window_hours,
                table_name=settings.db_table
            )

            if news_count < min_news_threshold:
                logger.info(
                    f"Найдено только {news_count} новостей за последние {settings.time_window_hours} часов "
                    f"(минимум: {min_news_threshold}). Анализ пропущен для снижения нагрузки."
                )
                return

            # Получаем новости
            logger.info(f"Найдено {news_count} новостей. Начинаем получение данных...")
            fetcher = NewsFetcher(db, settings)
            news_items = fetcher.fetch_recent_news()

            if not news_items:
                logger.warning("Новости не найдены. Анализ завершен.")
                return
            
            logger.info(f"Получено {len(news_items)} новостей для анализа")
            
            # Оптимизация: ограничиваем количество новостей для обработки
            # чтобы избежать перегрузки сервера
            # Используем ANALYZER_MAX_NEWS_LIMIT для контейнера, если установлена,
            # иначе MAX_NEWS_LIMIT для обратной совместимости
            # Временно уменьшаем лимит для стабильности работы
            max_news_limit = int(os.getenv("ANALYZER_MAX_NEWS_LIMIT",
                                          os.getenv("MAX_NEWS_LIMIT", "1200")))
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
            
            # Используем последовательную обработку для надежности (параллельная вызывает проблемы с NLTK)
            logger.info("Используется последовательная обработка текста...")
            processed_texts = []
            for item in news_items:
                if settings.use_titles_only:
                    text = item.title
                else:
                    text = f"{item.title} {item.description}"
                processed = cleaner.preprocess(text)
                processed_texts.append(processed)
            
            logger.info(f"Предобработано {len(processed_texts)} текстов")

            # Проверяем качество предобработки
            non_empty_texts = [t for t in processed_texts if t.strip()]
            logger.info(f"Непустых текстов после предобработки: {len(non_empty_texts)} из {len(processed_texts)}")

            # Проверяем, что processed_texts не пустые
            if not processed_texts or len(processed_texts) == 0:
                logger.error("processed_texts пустой!")
                return

            # Проверяем первые несколько текстов
            sample_texts = processed_texts[:3]
            logger.info(f"Примеры обработанных текстов: {sample_texts}")

            if len(non_empty_texts) < 10:
                logger.warning("Слишком мало непустых текстов для качественного анализа")
                return

            # 2. Векторизация
            logger.info("Векторизация текстов...")
            try:
                # Уменьшаем max_features для контейнера с ограниченными ресурсами
                max_features = min(settings.max_features, 5000)  # Ограничиваем до 5k признаков для экономии памяти
                logger.info(f"Используем max_features={max_features}")

                vectorizer = TextVectorizer(
                    max_features=max_features,
                    min_df=settings.min_df,
                    max_df=settings.max_df
                )
                logger.info(f"Начинаем векторизацию {len(processed_texts)} текстов...")
                logger.info(f"Векторизация {len(processed_texts)} текстов с {max_features} признаками...")
                vectors = vectorizer.fit_transform(processed_texts)
                logger.info(f"Векторы созданы: форма {len(vectors)}x{len(vectors[0]) if vectors else 0}")

                if not vectors or len(vectors) == 0:
                    logger.error("Векторизация вернула пустой результат!")
                    return

                # Проверяем качество векторов
                logger.info(f"Проверка векторов: тип={type(vectors)}, длина={len(vectors) if vectors else 0}")

                # Оцениваем использование памяти (примерно)
                if vectors and len(vectors) > 0 and vectors[0]:
                    estimated_memory_mb = (len(vectors) * len(vectors[0]) * 4) / (1024 * 1024)  # float32 = 4 bytes
                    logger.info(f"Оценка использования памяти векторами: {estimated_memory_mb:.1f} MB")
            except Exception as e:
                logger.error(f"Ошибка при векторизации: {e}")
                logger.exception("Подробности ошибки векторизации:")
                raise
            
            # 3. Кластеризация
            logger.info("Кластеризация новостей...")
            logger.info(f"Количество векторов для кластеризации: {len(vectors)}")
            try:
                clusterer = NewsClusterer(
                    min_cluster_size=settings.cluster_min_size,
                    min_samples=settings.cluster_min_samples,
                    metric=settings.cluster_metric
                )
                logger.info("Запуск кластеризации HDBSCAN...")
                labels, n_clusters, n_noise, unique_labels = clusterer.fit_predict(vectors)
                logger.info(f"Кластеризация завершена: {n_clusters} кластеров, {n_noise} шумовых точек")
                logger.info(f"Метки кластеров: {len(labels)} элементов, уникальные: {len(unique_labels)}")
            except Exception as e:
                logger.error(f"Ошибка при кластеризации: {e}")
                raise
            
            # 4. Построение нарративов
            logger.info("Построение нарративов...")
            logger.info(f"Количество новостей: {len(news_items)}, меток: {len(labels)}")
            try:
                narrative_builder = NarrativeBuilder()
                logger.info("Инициализация NarrativeBuilder...")
                narratives = narrative_builder.build_narratives(
                    news_items=news_items,
                    labels=labels,
                    vectorizer=vectorizer,
                    top_n=settings.top_narratives,
                    processed_texts=processed_texts
                )
                logger.info(f"Нарративы построены: {len(narratives)} из {n_clusters} кластеров")
                print(f"DEBUG: Построено {len(narratives)} нарративов из {n_clusters} кластеров")
            except Exception as e:
                print(f"DEBUG: Ошибка при построении нарративов: {e}")
                logger.error(f"Ошибка при построении нарративов: {e}")
                narratives = []  # Fallback to empty list
            
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

                # Проверяем, не выполнялся ли анализ сегодня
                today_start = analysis_date.replace(hour=0, minute=0, second=0, microsecond=0)
                today_end = analysis_date.replace(hour=23, minute=59, second=59, microsecond=999999)

                # Проверяем, не выполнялся ли анализ сегодня (отключено для тестирования)
                # recent_analysis = db.get_recent_analysis(hours=24)
                # if recent_analysis and len(recent_analysis) > 0:
                #     logger.info(f"Найден недавний анализ (ID: {recent_analysis[0].id}). Пропускаем сохранение для избежания дублирования.")
                # else:
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
            
            # Генерируем текстовое резюме с дополнительными метриками
            summary_gen = SummaryGenerator()

            # Добавляем метрики качества кластеризации
            clustering_metrics = {
                'total_clusters': n_clusters,
                'noise_points': n_noise,
                'noise_percentage': n_noise / len(labels) * 100 if labels else 0,
                'avg_cluster_size': sum(labels.count(cid) for cid in unique_labels if cid != -1) / n_clusters if n_clusters > 0 else 0,
                'max_cluster_size': max((labels.count(cid) for cid in unique_labels if cid != -1), default=0),
                'min_cluster_size': min((labels.count(cid) for cid in unique_labels if cid != -1), default=0)
            }

            summary = summary_gen.generate(
                narratives=narratives,
                total_news=len(news_items),
                analysis_date=analysis_date,
                clustering_metrics=clustering_metrics
            )
            
            # Выводим резюме в консоль и логи
            logger.info("\n" + summary)
            logger.info(f"Отчет готов к отправке. Длина: {len(summary)} символов")

            # 6. Отправка отчета в Telegram всем подписанным пользователям
            # Используется отдельный бот для отправки отчетов (TELEGRAM_SIGNAL_API_KEY)
            logger.info("Проверка токена Telegram...")
            telegram_token = os.getenv("TELEGRAM_SIGNAL_API_KEY")
            logger.info(f"TELEGRAM_SIGNAL_API_KEY: {'установлен' if telegram_token else 'НЕ установлен'}")

            if telegram_token:
                try:
                    logger.info("Получение списка пользователей из БД...")
                    users = db.get_all_users()
                    
                    if not users:
                        logger.warning("Пользователи не найдены в БД. Отчет не будет отправлен.")
                    else:
                        logger.info(f"Найдено {len(users)} пользователей. Отправка отчетов...")
                        
                        # Создаем notifier (без markdown для надежности)
                        notifier = TelegramNotifier(bot_token=telegram_token, parse_mode=None)
                        
                        # Получаем список chat_id
                        chat_ids = [user.chat_id for user in users]
                        
                        # Сначала попробуем отправить тестовое сообщение для проверки токена
                        logger.info("Тестирование отправки в Telegram...")
                        test_success = notifier.send_message(chat_ids[0], "🧪 Тестовое сообщение от анализатора новостей")
                        if not test_success:
                            logger.error("Тестовое сообщение не отправлено - проблема с токеном или ботом")
                            results = {chat_id: False for chat_id in chat_ids}
                        else:
                            logger.info("✓ Тестовое сообщение отправлено успешно")
                            # Отправляем текстовое резюме всем пользователям
                            results = {}
                            for chat_id in chat_ids:
                                success = notifier.send_summary(chat_id, summary)
                                results[chat_id] = success
                        
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

        except Exception as e:
            logger.error(f"Критическая ошибка в основной логике анализа: {e}")
            logger.exception("Подробности ошибки:")
            logger.error(f"Тип ошибки: {type(e).__name__}")
            import traceback
            logger.error(f"Трассировка:\n{traceback.format_exc()}")
            raise
            
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
