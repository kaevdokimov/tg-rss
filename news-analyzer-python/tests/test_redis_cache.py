"""Unit tests for Redis cache module."""

import pytest
from unittest.mock import patch, MagicMock
import json

from src.cache.redis_cache import RedisCache


class TestRedisCache:
    """Test cases for RedisCache class."""

    def setup_method(self):
        """Set up test fixtures."""
        self.cache = RedisCache(host="localhost", port=6379, db=0)

    def test_init(self):
        """Test RedisCache initialization."""
        assert self.cache.client is not None

    @patch('redis.Redis')
    def test_connection_failure(self, mock_redis):
        """Test handling of connection failure."""
        mock_redis_instance = MagicMock()
        mock_redis_instance.ping.side_effect = Exception("Connection failed")
        mock_redis.return_value = mock_redis_instance

        cache = RedisCache()
        # При ошибке подключения client должен быть None
        assert cache.client is None

    def test_hash_texts(self):
        """Test text hashing function."""
        texts = ["Первый текст", "Второй текст"]
        hash1 = self.cache._hash_texts(texts)
        hash2 = self.cache._hash_texts(texts)

        # Хэши должны быть одинаковыми для одинаковых входных данных
        assert hash1 == hash2
        assert len(hash1) == 16  # 16 символов как в коде

        # Хэши должны быть разными для разных входных данных
        different_texts = ["Другой текст", "Ещё один текст"]
        hash3 = self.cache._hash_texts(different_texts)
        assert hash1 != hash3

    def test_make_key(self):
        """Test key generation."""
        prefix = "test"
        data_hash = "abcd1234"
        key = self.cache._make_key(prefix, data_hash)

        expected = "news_analyzer:test:abcd1234"
        assert key == expected

    @patch.object(RedisCache, 'health_check')
    def test_get_vectorized_texts_no_cache(self, mock_health):
        """Test getting vectorized texts when Redis is unavailable."""
        mock_health.return_value = False

        cache = RedisCache()
        result = cache.get_vectorized_texts(["test text"])

        assert result is None

    @patch.object(RedisCache, 'health_check')
    def test_set_vectorized_texts_no_cache(self, mock_health):
        """Test setting vectorized texts when Redis is unavailable."""
        mock_health.return_value = False

        cache = RedisCache()
        # Не должно вызвать ошибку
        cache.set_vectorized_texts(["test"], [[1.0, 2.0]], 3600)

    @patch.object(RedisCache, 'health_check')
    def test_get_clustering_result_no_cache(self, mock_health):
        """Test getting clustering result when Redis is unavailable."""
        mock_health.return_value = False

        cache = RedisCache()
        result = cache.get_clustering_result("hash", {"param": "value"})

        assert result is None

    @patch.object(RedisCache, 'health_check')
    def test_set_clustering_result_no_cache(self, mock_health):
        """Test setting clustering result when Redis is unavailable."""
        mock_health.return_value = False

        cache = RedisCache()
        # Не должно вызвать ошибку
        cache.set_clustering_result("hash", {"param": "value"}, {"result": "data"}, 1800)

    @patch.object(RedisCache, 'health_check')
    def test_get_narratives_no_cache(self, mock_health):
        """Test getting narratives when Redis is unavailable."""
        mock_health.return_value = False

        cache = RedisCache()
        result = cache.get_narratives("hash", {"param": "value"})

        assert result is None

    @patch.object(RedisCache, 'health_check')
    def test_set_narratives_no_cache(self, mock_health):
        """Test setting narratives when Redis is unavailable."""
        mock_health.return_value = False

        cache = RedisCache()
        # Не должно вызвать ошибку
        cache.set_narratives("hash", {"param": "value"}, [{"narrative": "data"}], 1800)

    def test_invalidate_analysis_cache_no_cache(self):
        """Test cache invalidation when Redis is unavailable."""
        cache = RedisCache()
        cache.client = None  # Имитируем отключение Redis

        # Не должно вызвать ошибку
        cache.invalidate_analysis_cache(["test text"])

    def test_clear_all_cache_no_cache(self):
        """Test clearing all cache when Redis is unavailable."""
        cache = RedisCache()
        cache.client = None  # Имитируем отключение Redis

        # Не должно вызвать ошибку
        cache.clear_all_cache()

    def test_get_cache_stats_no_cache(self):
        """Test getting cache stats when Redis is unavailable."""
        cache = RedisCache()
        cache.client = None  # Имитируем отключение Redis

        stats = cache.get_cache_stats()

        expected_stats = {
            "vectors": 0,
            "clusters": 0,
            "narratives": 0,
            "total": 0
        }
        assert stats == expected_stats

    def test_health_check_no_cache(self):
        """Test health check when Redis is unavailable."""
        cache = RedisCache()
        cache.client = None  # Имитируем отключение Redis

        result = cache.health_check()
        assert result is False

    @patch('redis.Redis.keys')
    @patch('redis.Redis.delete')
    def test_invalidate_analysis_cache_with_cache(self, mock_delete, mock_keys):
        """Test cache invalidation when Redis is available."""
        mock_keys.return_value = [b"key1", b"key2"]

        cache = RedisCache()
        cache.invalidate_analysis_cache(["test text"])

        # Проверяем, что keys был вызван
        assert mock_keys.called
        # Проверяем, что delete был вызван с ключами
        mock_delete.assert_called_once_with(b"key1", b"key2")

    @patch('redis.Redis.keys')
    def test_clear_all_cache_with_cache(self, mock_keys):
        """Test clearing all cache when Redis is available."""
        mock_keys.return_value = [b"key1", b"key2", b"key3"]

        cache = RedisCache()
        cache.clear_all_cache()

        # Проверяем, что keys был вызван для паттерна news_analyzer:*
        mock_keys.assert_called_once_with("news_analyzer:*")

    @patch('redis.Redis.keys')
    def test_get_cache_stats_with_cache(self, mock_keys):
        """Test getting cache stats when Redis is available."""
        # Имитируем наличие ключей разных типов
        mock_keys.side_effect = [
            [b"news_analyzer:vectors:key1"],  # 1 vector key
            [b"news_analyzer:clusters:key2", b"news_analyzer:clusters:key3"],  # 2 cluster keys
            [b"news_analyzer:narratives:key4", b"news_analyzer:narratives:key5", b"news_analyzer:narratives:key6"]  # 3 narrative keys
        ]

        cache = RedisCache()
        stats = cache.get_cache_stats()

        expected_stats = {
            "vectors": 1,
            "clusters": 2,
            "narratives": 3,
            "total": 6
        }
        assert stats == expected_stats

    @patch('redis.Redis.ping')
    def test_health_check_with_cache(self, mock_ping):
        """Test health check when Redis is available."""
        mock_ping.return_value = True

        cache = RedisCache()
        result = cache.health_check()

        assert result is True
        mock_ping.assert_called_once()

    @patch('redis.Redis.ping')
    def test_health_check_with_cache_error(self, mock_ping):
        """Test health check when Redis ping fails."""
        mock_ping.side_effect = Exception("Connection error")

        cache = RedisCache()
        result = cache.health_check()

        assert result is False

    def test_json_serialization_clustering_result(self):
        """Test JSON serialization/deserialization of clustering results."""
        test_data = {
            "labels": [0, 0, 1, 1, -1],
            "n_clusters": 2,
            "n_noise": 1,
            "unique_labels": [0, 1, -1]
        }

        # Тестируем сериализацию
        json_str = json.dumps(test_data, ensure_ascii=False)

        # Тестируем десериализацию
        parsed_data = json.loads(json_str)

        assert parsed_data == test_data

    def test_json_serialization_narratives(self):
        """Test JSON serialization/deserialization of narratives."""
        test_narratives = [
            {
                "theme": "Политика",
                "keywords": ["президент", "правительство", "выборы"],
                "size": 15,
                "examples": ["Президент выступил с речью", "Правительство приняло закон"]
            },
            {
                "theme": "Экономика",
                "keywords": ["экономика", "кризис", "инфляция"],
                "size": 8,
                "examples": ["Экономика показывает рост", "Инфляция снижается"]
            }
        ]

        # Тестируем сериализацию
        json_str = json.dumps(test_narratives, ensure_ascii=False)

        # Тестируем десериализацию
        parsed_narratives = json.loads(json_str)

        assert parsed_narratives == test_narratives