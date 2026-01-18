"""Асинхронная обработка новостей для улучшения производительности."""

import asyncio
import time
from concurrent.futures import ThreadPoolExecutor
from typing import List, Dict, Any, Optional
import numpy as np

from ..db.database import NewsItem
from ..preprocessor.text_cleaner import TextCleaner
from ..analyzer.vectorizer import TextVectorizer
from ..analyzer.cluster import NewsClusterer
from ..narrative.builder import NarrativeBuilder
from ..monitoring.metrics import metrics_manager
from ..utils.logger import get_logger

logger = get_logger(__name__)


class AsyncNewsProcessor:
    """Асинхронный процессор новостей с параллельной обработкой."""

    def __init__(self, max_workers: int = 4):
        """Инициализация асинхронного процессора."""
        self.max_workers = max_workers
        self.executor = ThreadPoolExecutor(max_workers=max_workers, thread_name_prefix="news_processor")

    async def preprocess_texts_async(self, news_items: List[NewsItem],
                                   settings: Any) -> List[str]:
        """
        Асинхронная предобработка текстов.

        Args:
            news_items: Список новостей
            settings: Настройки приложения

        Returns:
            Список обработанных текстов
        """
        start_time = time.time()

        # Создаем задачи для параллельной обработки
        tasks = []
        for item in news_items:
            task = asyncio.get_event_loop().run_in_executor(
                self.executor,
                self._preprocess_single_item,
                item,
                settings
            )
            tasks.append(task)

        # Ждем завершения всех задач
        processed_texts = []
        for task in asyncio.as_completed(tasks):
            try:
                text = await task
                processed_texts.append(text)
            except Exception as e:
                logger.error("Error in text preprocessing task", "error", str(e))
                processed_texts.append("")  # Добавляем пустую строку в случае ошибки

        processing_time = time.time() - start_time
        metrics_manager.record_preprocessing(processing_time, len(news_items))

        logger.info("Async text preprocessing completed",
            "texts_count", len(processed_texts),
            "processing_time", round(processing_time, 2))

        return processed_texts

    def _preprocess_single_item(self, item: NewsItem, settings: Any) -> str:
        """Предобработка одного элемента (выполняется в thread pool)."""
        try:
            cleaner = TextCleaner(
                stopwords_extra=settings.stopwords_extra,
                min_word_length=settings.min_word_length,
                max_word_length=settings.max_word_length
            )

            if settings.use_titles_only:
                text = item.title
            else:
                text = f"{item.title} {item.description}"

            return cleaner.preprocess(text)
        except Exception as e:
            logger.error("Error preprocessing single item",
                "item_id", getattr(item, 'id', 'unknown'),
                "error", str(e))
            return ""

    async def vectorize_texts_async(self, processed_texts: List[str],
                                   settings: Any) -> Optional[List[List[float]]]:
        """
        Асинхронная векторизация текстов.

        Args:
            processed_texts: Обработанные тексты
            settings: Настройки приложения

        Returns:
            Векторы или None в случае ошибки
        """
        start_time = time.time()

        try:
            # Выполняем векторизацию в thread pool
            loop = asyncio.get_event_loop()
            vectors = await loop.run_in_executor(
                self.executor,
                self._vectorize_texts_sync,
                processed_texts,
                settings
            )

            processing_time = time.time() - start_time
            metrics_manager.record_vectorization(processing_time)

            logger.info("Async text vectorization completed",
                "texts_count", len(processed_texts),
                "vectors_shape", f"{len(vectors)}x{len(vectors[0]) if vectors else 0}",
                "processing_time", round(processing_time, 2))

            return vectors

        except Exception as e:
            logger.error("Error in async vectorization", "error", str(e))
            return None

    def _vectorize_texts_sync(self, processed_texts: List[str], settings: Any) -> List[List[float]]:
        """Синхронная векторизация (выполняется в thread pool)."""
        max_features = min(settings.max_features, 5000)
        vectorizer = TextVectorizer(
            max_features=max_features,
            min_df=settings.min_df,
            max_df=settings.max_df
        )

        vectors = vectorizer.fit_transform(processed_texts)
        return vectors

    async def cluster_texts_async(self, vectors: List[List[float]],
                                 settings: Any) -> Optional[Dict[str, Any]]:
        """
        Асинхронная кластеризация векторов.

        Args:
            vectors: Векторы текстов
            settings: Настройки приложения

        Returns:
            Результаты кластеризации или None
        """
        start_time = time.time()

        try:
            loop = asyncio.get_event_loop()
            result = await loop.run_in_executor(
                self.executor,
                self._cluster_texts_sync,
                vectors,
                settings
            )

            processing_time = time.time() - start_time
            metrics_manager.record_clustering(
                processing_time,
                result.get('n_clusters', 0),
                result.get('n_noise', 0),
                len(vectors)
            )

            logger.info("Async clustering completed",
                "vectors_count", len(vectors),
                "clusters", result.get('n_clusters', 0),
                "noise", result.get('n_noise', 0),
                "processing_time", round(processing_time, 2))

            return result

        except Exception as e:
            logger.error("Error in async clustering", "error", str(e))
            return None

    def _cluster_texts_sync(self, vectors: List[List[float]], settings: Any) -> Dict[str, Any]:
        """Синхронная кластеризация (выполняется в thread pool)."""
        clusterer = NewsClusterer(
            min_cluster_size=settings.cluster_min_size,
            min_samples=settings.cluster_min_samples,
            metric=settings.cluster_metric
        )

        labels, n_clusters, n_noise, unique_labels = clusterer.fit_predict(vectors)

        return {
            'labels': labels,
            'n_clusters': n_clusters,
            'n_noise': n_noise,
            'unique_labels': list(unique_labels)
        }

    async def build_narratives_async(self, news_items: List[NewsItem],
                                    labels: List[int],
                                    vectorizer: TextVectorizer,
                                    settings: Any,
                                    processed_texts: List[str]) -> Optional[List[Dict[str, Any]]]:
        """
        Асинхронное построение нарративов.

        Args:
            news_items: Новости
            labels: Метки кластеров
            vectorizer: Векторизатор
            settings: Настройки
            processed_texts: Обработанные тексты

        Returns:
            Нарративы или None
        """
        start_time = time.time()

        try:
            loop = asyncio.get_event_loop()
            narratives = await loop.run_in_executor(
                self.executor,
                self._build_narratives_sync,
                news_items,
                labels,
                vectorizer,
                settings,
                processed_texts
            )

            processing_time = time.time() - start_time
            metrics_manager.record_narrative_building(processing_time)

            logger.info("Async narrative building completed",
                "narratives_count", len(narratives) if narratives else 0,
                "processing_time", round(processing_time, 2))

            return narratives

        except Exception as e:
            logger.error("Error in async narrative building", "error", str(e))
            return None

    def _build_narratives_sync(self, news_items: List[NewsItem],
                              labels: List[int],
                              vectorizer: TextVectorizer,
                              settings: Any,
                              processed_texts: List[str]) -> List[Dict[str, Any]]:
        """Синхронное построение нарративов (выполняется в thread pool)."""
        narrative_builder = NarrativeBuilder()
        narratives = narrative_builder.build_narratives(
            news_items=news_items,
            labels=labels,
            vectorizer=vectorizer,
            top_n=settings.top_narratives,
            processed_texts=processed_texts
        )
        return narratives

    async def process_news_batch_async(self, news_items: List[NewsItem],
                                      settings: Any) -> Optional[Dict[str, Any]]:
        """
        Полная асинхронная обработка батча новостей.

        Args:
            news_items: Список новостей для обработки
            settings: Настройки приложения

        Returns:
            Полные результаты анализа или None
        """
        logger.info("Starting async news batch processing",
            "news_count", len(news_items))

        # 1. Асинхронная предобработка
        processed_texts = await self.preprocess_texts_async(news_items, settings)
        if not processed_texts:
            logger.error("Preprocessing failed")
            return None

        # 2. Асинхронная векторизация
        vectors = await self.vectorize_texts_async(processed_texts, settings)
        if not vectors:
            logger.error("Vectorization failed")
            return None

        # 3. Асинхронная кластеризация
        cluster_result = await self.cluster_texts_async(vectors, settings)
        if not cluster_result:
            logger.error("Clustering failed")
            return None

        # 4. Асинхронное построение нарративов
        narratives = await self.build_narratives_async(
            news_items, cluster_result['labels'], None, settings, processed_texts
        )
        if narratives is None:
            logger.error("Narrative building failed")
            return None

        # Собираем полный результат
        result = {
            'processed_texts': processed_texts,
            'vectors': vectors,
            'cluster_result': cluster_result,
            'narratives': narratives,
            'total_news': len(news_items),
            'processing_stats': {
                'texts_processed': len(processed_texts),
                'vectors_created': len(vectors),
                'clusters_found': cluster_result['n_clusters'],
                'narratives_built': len(narratives)
            }
        }

        logger.info("Async news batch processing completed",
            "total_news", len(news_items),
            "narratives", len(narratives))

        return result

    def shutdown(self):
        """Завершение работы процессора."""
        if self.executor:
            self.executor.shutdown(wait=True)
            logger.info("Async processor shutdown completed")