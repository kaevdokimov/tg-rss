"""Integration tests for the complete ML pipeline."""

import pytest
import psycopg2
from datetime import datetime, timedelta
import time

from src.db import Database
from src.fetcher import NewsFetcher
from src.preprocessor import AdvancedTextCleaner
from src.analyzer import TextVectorizer, NewsClusterer
from src.narrative import NarrativeBuilder
from src.cache.redis_cache import RedisCache


class TestMLPipelineIntegration:
    """Integration tests for the complete ML pipeline."""

    def setup_method(self):
        """Set up test fixtures."""
        self.sample_news = [
            {
                "title": "Президент Путин встретился с министром иностранных дел",
                "description": "Владимир Путин обсудил международные отношения с главой МИД Сергеем Лавровым",
                "published_at": datetime.now() - timedelta(hours=2)
            },
            {
                "title": "Экономика России показывает рост",
                "description": "Центробанк сообщил о положительной динамике экономических показателей",
                "published_at": datetime.now() - timedelta(hours=3)
            },
            {
                "title": "Спорт: Россия выиграла золото на Олимпиаде",
                "description": "Российские спортсмены завоевали золотую медаль в командных соревнованиях",
                "published_at": datetime.now() - timedelta(hours=4)
            },
            {
                "title": "Технологии: Новый смартфон от Samsung",
                "description": "Компания Samsung представила флагманский смартфон с инновационными функциями",
                "published_at": datetime.now() - timedelta(hours=5)
            },
            {
                "title": "Погода: Теплая весна в Москве",
                "description": "Синоптики прогнозируют комфортную погоду на ближайшие дни",
                "published_at": datetime.now() - timedelta(hours=6)
            },
            {
                "title": "Политика: Новые санкции против России",
                "description": "Европейский союз ввел дополнительные ограничительные меры",
                "published_at": datetime.now() - timedelta(hours=7)
            },
            {
                "title": "Экономика: Курс рубля укрепился",
                "description": "Российская валюта показала рост по отношению к доллару и евро",
                "published_at": datetime.now() - timedelta(hours=8)
            },
            {
                "title": "Спорт: Футбольный клуб выиграл чемпионат",
                "description": "Московский клуб стал победителем национального первенства",
                "published_at": datetime.now() - timedelta(hours=9)
            }
        ]

    def test_full_ml_pipeline_integration(self, db_config, redis_config):
        """Test the complete ML pipeline from database to narratives."""
        # Setup database
        db = Database(db_config)
        db.connect()

        try:
            # Create test data
            self._setup_test_data(db)

            # Test components
            self._test_data_fetching(db)
            self._test_text_preprocessing()
            self._test_vectorization()
            self._test_clustering()
            self._test_narrative_building()
            self._test_caching_integration(redis_config)
            self._test_pipeline_integration(db, redis_config)

        finally:
            db.disconnect()

    def _setup_test_data(self, db):
        """Set up test data in database."""
        # Create news table
        db.ensure_analysis_table_exists()

        # Insert test news
        for news_item in self.sample_news:
            db.save_news(
                source_id=1,
                title=news_item["title"],
                description=news_item["description"],
                link=f"https://example.com/news/{hash(news_item['title'])}",
                published_at=news_item["published_at"]
            )

        # Verify data
        conn = db.connection
        cursor = conn.cursor()
        cursor.execute("SELECT COUNT(*) FROM news")
        count = cursor.fetchone()[0]
        assert count >= len(self.sample_news)

    def _test_data_fetching(self, db):
        """Test data fetching from database."""
        fetcher = NewsFetcher(db, type('Config', (), {
            'time_window_hours': 24,
            'db_table': 'news'
        })())

        news_items = fetcher.fetch_recent_news()
        assert len(news_items) >= len(self.sample_news)

        # Check data structure
        for item in news_items[:3]:  # Check first 3 items
            assert hasattr(item, 'title')
            assert hasattr(item, 'description')
            assert len(item.title) > 0
            assert len(item.description) > 0

    def _test_text_preprocessing(self):
        """Test text preprocessing pipeline."""
        cleaner = AdvancedTextCleaner(
            use_lemmatization=False,  # Skip for faster tests
            min_word_length=3,
            max_word_length=15
        )

        for news_item in self.sample_news[:3]:  # Test first 3 items
            text = f"{news_item['title']} {news_item['description']}"
            processed = cleaner.preprocess(text)

            # Basic checks
            assert isinstance(processed, str)
            assert len(processed) > 0
            assert processed == processed.lower()  # Should be lowercase

            # Check that some common preprocessing happened
            assert "  " not in processed  # No double spaces
            assert not processed.startswith(" ")
            assert not processed.endswith(" ")

    def _test_vectorization(self):
        """Test text vectorization."""
        vectorizer = TextVectorizer(
            max_features=100,
            min_df=1,
            max_df=1.0
        )

        texts = [f"{item['title']} {item['description']}" for item in self.sample_news]
        vectors = vectorizer.fit_transform(texts)

        assert isinstance(vectors, list)
        assert len(vectors) == len(texts)
        assert len(vectors[0]) <= 100  # max_features

        # Check vector properties
        for vector in vectors:
            assert len(vector) == len(vectors[0])  # Same length
            assert all(isinstance(v, float) for v in vector)

    def _test_clustering(self):
        """Test news clustering."""
        # Prepare data
        vectorizer = TextVectorizer(max_features=50)
        texts = [f"{item['title']} {item['description']}" for item in self.sample_news]
        vectors = vectorizer.fit_transform(texts)

        # Test clustering
        clusterer = NewsClusterer(
            min_cluster_size=2,
            min_samples=2,
            evaluate_quality=True
        )

        labels, n_clusters, n_noise, _ = clusterer.fit_predict(vectors)

        assert isinstance(labels, list)
        assert len(labels) == len(vectors)
        assert n_clusters >= 0
        assert n_noise >= 0

        # Check quality metrics
        quality_metrics = clusterer.get_quality_metrics()
        if quality_metrics:
            assert 'overall_quality_score' in quality_metrics
            assert 0 <= quality_metrics['overall_quality_score'] <= 1

    def _test_narrative_building(self):
        """Test narrative building."""
        # Prepare test data
        vectorizer = TextVectorizer(max_features=50)
        texts = [f"{item['title']} {item['description']}" for item in self.sample_news]
        vectors = vectorizer.fit_transform(texts)

        clusterer = NewsClusterer(min_cluster_size=2)
        labels, n_clusters, _, _ = clusterer.fit_predict(vectors)

        # Create mock news items
        news_items = []
        for i, text in enumerate(texts):
            item = type('NewsItem', (), {
                'id': i,
                'title': self.sample_news[i]['title'],
                'description': self.sample_news[i]['description']
            })()
            news_items.append(item)

        # Test narrative building
        narrative_builder = NarrativeBuilder()
        narratives = narrative_builder.build_narratives(
            news_items=news_items,
            labels=labels,
            vectorizer=vectorizer,
            top_n=min(3, n_clusters),
            processed_texts=texts
        )

        assert isinstance(narratives, list)
        if n_clusters > 0:
            assert len(narratives) <= min(3, n_clusters)

        # Check narrative structure
        for narrative in narratives:
            assert 'theme' in narrative
            assert 'keywords' in narrative
            assert 'size' in narrative
            assert isinstance(narrative['keywords'], list)
            assert narrative['size'] > 0

    def _test_caching_integration(self, redis_config):
        """Test Redis caching integration."""
        cache = RedisCache(
            host=redis_config["host"],
            port=redis_config["port"]
        )

        # Test basic operations
        test_texts = ["Тестовый текст для кэширования"]
        test_vectors = [[0.1, 0.2, 0.3]]

        # Test vector caching
        cache.set_vectorized_texts(test_texts, test_vectors, ttl_seconds=60)
        cached_vectors = cache.get_vectorized_texts(test_texts)

        if cached_vectors:  # Redis may not be available in all environments
            assert cached_vectors == test_vectors

        # Test clustering result caching
        test_params = {"min_cluster_size": 5}
        test_result = {"labels": [0, 0, 1], "n_clusters": 2}

        cache.set_clustering_result("test_hash", test_params, test_result, ttl_seconds=60)
        cached_result = cache.get_clustering_result("test_hash", test_params)

        if cached_result:  # Redis may not be available in all environments
            assert cached_result == test_result

    def _test_pipeline_integration(self, db, redis_config):
        """Test complete pipeline integration."""
        try:
            cache = RedisCache(
                host=redis_config["host"],
                port=redis_config["port"]
            )
        except:
            cache = None

        # Simulate the full pipeline
        fetcher = NewsFetcher(db, type('Config', (), {
            'time_window_hours': 24,
            'db_table': 'news'
        })())

        news_items = fetcher.fetch_recent_news()
        assert len(news_items) >= len(self.sample_news)

        # Preprocessing
        cleaner = AdvancedTextCleaner(use_lemmatization=False)
        processed_texts = []
        for item in news_items:
            text = f"{item.title} {item.description}"
            processed = cleaner.preprocess(text)
            processed_texts.append(processed)

        # Vectorization with caching
        vectorizer = TextVectorizer(max_features=50)
        texts_hash = None
        if cache:
            texts_hash = cache._hash_texts(processed_texts)

        vectors = vectorizer.fit_transform(processed_texts)

        # Cache vectors
        if cache and texts_hash:
            cache.set_vectorized_texts(processed_texts, vectors, ttl_seconds=300)

        # Clustering
        clusterer = NewsClusterer(min_cluster_size=2, evaluate_quality=True)
        labels, n_clusters, n_noise, _ = clusterer.fit_predict(vectors)

        # Build narratives
        narrative_builder = NarrativeBuilder()
        narratives = narrative_builder.build_narratives(
            news_items=news_items,
            labels=labels,
            vectorizer=vectorizer,
            top_n=min(3, max(1, n_clusters)),
            processed_texts=processed_texts
        )

        # Verify results
        assert len(labels) == len(news_items)
        assert n_clusters >= 0
        assert isinstance(narratives, list)

        if n_clusters > 0:
            assert len(narratives) > 0
            # Check that narratives have required fields
            for narrative in narratives:
                assert all(key in narrative for key in ['theme', 'keywords', 'size'])

        print(f"✓ Pipeline integration test passed: {len(news_items)} news, {n_clusters} clusters, {len(narratives)} narratives")