"""Redis caching для векторизации и кластеризации."""

import json
import pickle
from typing import Optional, Dict, Any, List
import redis
import hashlib
from datetime import datetime, timedelta

from ..utils.logger import get_logger

logger = get_logger(__name__)


class RedisCache:
    """Redis кэш для ML данных."""

    def __init__(self, host: str = "localhost", port: int = 6379, db: int = 0, password: str = None):
        """Инициализация Redis клиента."""
        self.client = redis.Redis(
            host=host,
            port=port,
            db=db,
            password=password,
            decode_responses=False,  # Для бинарных данных
            socket_timeout=5,
            socket_connect_timeout=5,
            retry_on_timeout=True,
            max_connections=10
        )
        self._test_connection()

    def _test_connection(self):
        """Тестирование подключения к Redis."""
        try:
            self.client.ping()
            logger.info("Redis connection established")
        except redis.ConnectionError as e:
            logger.warning(f"Redis connection failed: {e}")
            self.client = None

    def _make_key(self, prefix: str, data_hash: str) -> str:
        """Создание ключа для кэша."""
        return f"news_analyzer:{prefix}:{data_hash}"

    def _hash_texts(self, texts: List[str]) -> str:
        """Создание хэша для списка текстов."""
        content = "|".join(texts).encode('utf-8')
        return hashlib.sha256(content).hexdigest()[:16]

    def get_vectorized_texts(self, texts: List[str]) -> Optional[List[List[float]]]:
        """
        Получение векторизованных текстов из кэша.

        Args:
            texts: Список текстов

        Returns:
            Векторы или None если не найдено в кэше
        """
        if not self.client:
            return None

        try:
            texts_hash = self._hash_texts(texts)
            key = self._make_key("vectors", texts_hash)

            cached_data = self.client.get(key)
            if cached_data:
                vectors = pickle.loads(cached_data)
                logger.debug("Retrieved vectorized texts from cache", "key", key)
                return vectors
        except Exception as e:
            logger.warning("Failed to get vectorized texts from cache", "error", str(e))

        return None

    def set_vectorized_texts(self, texts: List[str], vectors: List[List[float]], ttl_seconds: int = 3600):
        """
        Сохранение векторизованных текстов в кэш.

        Args:
            texts: Список текстов
            vectors: Векторы
            ttl_seconds: Время жизни в секундах
        """
        if not self.client:
            return

        try:
            texts_hash = self._hash_texts(texts)
            key = self._make_key("vectors", texts_hash)

            # Сериализация с помощью pickle для numpy arrays
            data = pickle.dumps(vectors)
            self.client.setex(key, ttl_seconds, data)

            logger.debug("Cached vectorized texts", "key", key, "ttl", ttl_seconds)
        except Exception as e:
            logger.warning("Failed to cache vectorized texts", "error", str(e))

    def get_clustering_result(self, vectors_hash: str, params: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Получение результатов кластеризации из кэша.

        Args:
            vectors_hash: Хэш векторов
            params: Параметры кластеризации

        Returns:
            Результаты кластеризации или None
        """
        if not self.client:
            return None

        try:
            # Создаем хэш параметров для уникальности
            params_str = json.dumps(params, sort_keys=True)
            params_hash = hashlib.md5(params_str.encode()).hexdigest()[:8]
            key = self._make_key("clusters", f"{vectors_hash}_{params_hash}")

            cached_data = self.client.get(key)
            if cached_data:
                result = json.loads(cached_data.decode('utf-8'))
                logger.debug("Retrieved clustering result from cache", "key", key)
                return result
        except Exception as e:
            logger.warning("Failed to get clustering result from cache", "error", str(e))

        return None

    def set_clustering_result(self, vectors_hash: str, params: Dict[str, Any],
                            result: Dict[str, Any], ttl_seconds: int = 1800):
        """
        Сохранение результатов кластеризации в кэш.

        Args:
            vectors_hash: Хэш векторов
            params: Параметры кластеризации
            result: Результаты кластеризации
            ttl_seconds: Время жизни в секундах
        """
        if not self.client:
            return

        try:
            # Создаем хэш параметров для уникальности
            params_str = json.dumps(params, sort_keys=True)
            params_hash = hashlib.md5(params_str.encode()).hexdigest()[:8]
            key = self._make_key("clusters", f"{vectors_hash}_{params_hash}")

            # Сериализация в JSON
            data = json.dumps(result, ensure_ascii=False)
            self.client.setex(key, ttl_seconds, data)

            logger.debug("Cached clustering result", "key", key, "ttl", ttl_seconds)
        except Exception as e:
            logger.warning("Failed to cache clustering result", "error", str(e))

    def get_narratives(self, clusters_hash: str, params: Dict[str, Any]) -> Optional[List[Dict[str, Any]]]:
        """
        Получение нарративов из кэша.

        Args:
            clusters_hash: Хэш результатов кластеризации
            params: Параметры построения нарративов

        Returns:
            Нарративы или None
        """
        if not self.client:
            return None

        try:
            params_str = json.dumps(params, sort_keys=True)
            params_hash = hashlib.md5(params_str.encode()).hexdigest()[:8]
            key = self._make_key("narratives", f"{clusters_hash}_{params_hash}")

            cached_data = self.client.get(key)
            if cached_data:
                narratives = json.loads(cached_data.decode('utf-8'))
                logger.debug("Retrieved narratives from cache", "key", key)
                return narratives
        except Exception as e:
            logger.warning("Failed to get narratives from cache", "error", str(e))

        return None

    def set_narratives(self, clusters_hash: str, params: Dict[str, Any],
                      narratives: List[Dict[str, Any]], ttl_seconds: int = 1800):
        """
        Сохранение нарративов в кэш.

        Args:
            clusters_hash: Хэш результатов кластеризации
            params: Параметры построения нарративов
            narratives: Нарративы
            ttl_seconds: Время жизни в секундах
        """
        if not self.client:
            return

        try:
            params_str = json.dumps(params, sort_keys=True)
            params_hash = hashlib.md5(params_str.encode()).hexdigest()[:8]
            key = self._make_key("narratives", f"{clusters_hash}_{params_hash}")

            data = json.dumps(narratives, ensure_ascii=False)
            self.client.setex(key, ttl_seconds, data)

            logger.debug("Cached narratives", "key", key, "ttl", ttl_seconds)
        except Exception as e:
            logger.warning("Failed to cache narratives", "error", str(e))

    def invalidate_analysis_cache(self, texts: List[str]):
        """
        Инвалидация кэша для анализа конкретных текстов.

        Args:
            texts: Список текстов
        """
        if not self.client:
            return

        try:
            texts_hash = self._hash_texts(texts)

            # Удаляем все связанные ключи
            patterns = [
                f"news_analyzer:vectors:{texts_hash}*",
                f"news_analyzer:clusters:*{texts_hash}*",
                f"news_analyzer:narratives:*{texts_hash}*"
            ]

            for pattern in patterns:
                keys = self.client.keys(pattern)
                if keys:
                    self.client.delete(*keys)
                    logger.debug("Invalidated cache keys", "pattern", pattern, "count", len(keys))

        except Exception as e:
            logger.warning("Failed to invalidate cache", "error", str(e))

    def clear_all_cache(self):
        """Очистка всего кэша news-analyzer."""
        if not self.client:
            return

        try:
            keys = self.client.keys("news_analyzer:*")
            if keys:
                self.client.delete(*keys)
                logger.info("Cleared all cache", "keys_deleted", len(keys))
        except Exception as e:
            logger.error("Failed to clear cache", "error", str(e))

    def get_cache_stats(self) -> Dict[str, int]:
        """Получение статистики кэша."""
        stats = {
            "vectors": 0,
            "clusters": 0,
            "narratives": 0,
            "total": 0
        }

        if not self.client:
            return stats

        try:
            patterns = ["news_analyzer:vectors:*", "news_analyzer:clusters:*", "news_analyzer:narratives:*"]
            for i, pattern in enumerate(patterns):
                keys = self.client.keys(pattern)
                count = len(keys)
                stats[list(stats.keys())[i]] = count
                stats["total"] += count
        except Exception as e:
            logger.warning("Failed to get cache stats", "error", str(e))

        return stats

    def health_check(self) -> bool:
        """Проверка здоровья Redis соединения."""
        if not self.client:
            return False

        try:
            self.client.ping()
            return True
        except:
            return False