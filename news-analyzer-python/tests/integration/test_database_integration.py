"""Integration tests for database operations."""

import pytest
import psycopg2
from datetime import datetime, timedelta

from src.db import Database


class TestDatabaseIntegration:
    """Integration tests for database operations with real PostgreSQL."""

    def test_database_connection(self, db_config):
        """Test database connection and basic operations."""
        db = Database(db_config)
        assert db.connect()

        # Test connection
        assert db.test_connection()

        db.disconnect()

    def test_news_crud_operations(self, db_config):
        """Test CRUD operations for news."""
        db = Database(db_config)
        db.connect()

        try:
            # Create test source
            source_id = 999
            db.save_source(db_source=type('Source', (), {
                'name': 'Test Source',
                'url': 'https://test.com',
                'status': 'active'
            })())

            # Create test news
            news_id = db.save_news(
                source_id=source_id,
                title="Test News Title",
                description="Test news description for integration testing",
                link="https://test.com/news/123",
                published_at=datetime.now()
            )

            assert news_id > 0

            # Read news
            news_count = db.get_news_count_last_hours(hours=1, table_name="news")
            assert news_count >= 1

            # Clean up
            conn = db.connection
            cursor = conn.cursor()
            cursor.execute("DELETE FROM news WHERE id = %s", (news_id,))
            cursor.execute("DELETE FROM sources WHERE id = %s", (source_id,))
            conn.commit()

        finally:
            db.disconnect()

    def test_user_operations(self, db_config):
        """Test user-related database operations."""
        db = Database(db_config)
        db.connect()

        try:
            # Test user existence check
            exists = db.user_exists(db_conn=db.connection, chat_id=12345)
            assert isinstance(exists, bool)

            # Test saving user (if not exists)
            test_chat_id = 999999
            db.save_user(type('User', (), {
                'chat_id': test_chat_id,
                'username': 'testuser',
                'first_name': 'Test',
                'last_name': 'User'
            })())

            # Verify user was created
            exists_after = db.user_exists(db_conn=db.connection, chat_id=test_chat_id)
            assert exists_after

            # Clean up
            conn = db.connection
            cursor = conn.cursor()
            cursor.execute("DELETE FROM users WHERE chat_id = %s", (test_chat_id,))
            conn.commit()

        finally:
            db.disconnect()

    def test_subscription_operations(self, db_config):
        """Test subscription-related database operations."""
        db = Database(db_config)
        db.connect()

        try:
            # Create test user and source
            test_chat_id = 888888
            test_source_id = 888

            db.save_user(type('User', (), {
                'chat_id': test_chat_id,
                'username': 'testuser',
                'first_name': 'Test',
                'last_name': 'User'
            })())

            db.save_source(type('Source', (), {
                'name': 'Test Source',
                'url': 'https://test.com',
                'status': 'active'
            })())

            # Create subscription
            db.save_subscription(type('Subscription', (), {
                'chat_id': test_chat_id,
                'source_id': test_source_id
            })())

            # Test subscription checks
            is_subscribed = db.is_user_subscribed(db_conn=db.connection,
                                                chat_id=test_chat_id,
                                                source_id=test_source_id)
            assert is_subscribed

            # Get subscriptions
            subscriptions = db.get_user_subscriptions_with_details(db_conn=db.connection,
                                                                 chat_id=test_chat_id)
            assert len(subscriptions) >= 1

            # Delete subscription
            db.delete_subscription(type('Subscription', (), {
                'chat_id': test_chat_id,
                'source_id': test_source_id
            })())

            # Verify deletion
            is_subscribed_after = db.is_user_subscribed(db_conn=db.connection,
                                                      chat_id=test_chat_id,
                                                      source_id=test_source_id)
            assert not is_subscribed_after

            # Clean up
            conn = db.connection
            cursor = conn.cursor()
            cursor.execute("DELETE FROM users WHERE chat_id = %s", (test_chat_id,))
            cursor.execute("DELETE FROM sources WHERE id = %s", (test_source_id,))
            conn.commit()

        finally:
            db.disconnect()

    def test_analysis_operations(self, db_config):
        """Test analysis result storage and retrieval."""
        db = Database(db_config)
        db.connect()

        try:
            # Ensure analysis table exists
            db.ensure_analysis_table_exists()

            # Save analysis result
            test_narratives = [
                {
                    "theme": "Test Theme",
                    "keywords": ["test", "theme"],
                    "size": 5,
                    "examples": ["Example 1", "Example 2"]
                }
            ]

            analysis_id = db.save_analysis_result(
                analysis_date=datetime.now(),
                total_news=10,
                narratives=test_narratives
            )

            assert analysis_id > 0

            # Retrieve analysis
            recent_analysis = db.get_recent_analysis(hours=1)
            assert len(recent_analysis) >= 1

            # Verify the saved analysis
            found = False
            for analysis in recent_analysis:
                if analysis.id == analysis_id:
                    assert analysis.total_news == 10
                    assert len(analysis.narratives) == 1
                    found = True
                    break

            assert found, "Saved analysis not found"

            # Clean up
            conn = db.connection
            cursor = conn.cursor()
            cursor.execute("DELETE FROM news_analysis WHERE id = %s", (analysis_id,))
            conn.commit()

        finally:
            db.disconnect()

    def test_admin_statistics(self, db_config):
        """Test admin statistics retrieval."""
        db = Database(db_config)
        db.connect()

        try:
            # Get admin stats
            stats = db.get_admin_stats()

            # Check structure
            assert hasattr(stats, 'total_users')
            assert hasattr(stats, 'total_news')
            assert hasattr(stats, 'total_sources')

            # Values should be non-negative
            assert stats.total_users >= 0
            assert stats.total_news >= 0
            assert stats.total_sources >= 0

        finally:
            db.disconnect()

    def test_concurrent_connections(self, db_config):
        """Test multiple concurrent database connections."""
        import threading
        import time

        results = []
        errors = []

        def worker(worker_id):
            try:
                db = Database(db_config)
                db.connect()

                # Simple query
                count = db.get_news_count_last_hours(hours=24, table_name="news")
                results.append((worker_id, count))

                db.disconnect()
            except Exception as e:
                errors.append((worker_id, str(e)))

        # Start 5 concurrent connections
        threads = []
        for i in range(5):
            t = threading.Thread(target=worker, args=(i,))
            threads.append(t)
            t.start()

        # Wait for all threads
        for t in threads:
            t.join()

        # Verify results
        assert len(results) == 5, f"Expected 5 results, got {len(results)}"
        assert len(errors) == 0, f"Got errors: {errors}"

        # All results should be similar (same data)
        counts = [r[1] for r in results]
        assert all(c >= 0 for c in counts), "All counts should be non-negative"

    def test_transaction_rollback(self, db_config):
        """Test transaction rollback on error."""
        db = Database(db_config)
        db.connect()

        try:
            conn = db.connection

            # Start transaction
            conn.autocommit = False

            try:
                # Insert test data
                cursor = conn.cursor()
                cursor.execute("""
                    INSERT INTO news (source_id, title, description, link, published_at)
                    VALUES (1, 'Test Title', 'Test Description', 'https://test.com', NOW())
                """)

                # Simulate error
                raise Exception("Test error for rollback")

            except Exception:
                # Rollback transaction
                conn.rollback()
                conn.autocommit = True

            # Verify no data was inserted
            cursor = conn.cursor()
            cursor.execute("SELECT COUNT(*) FROM news WHERE title = 'Test Title'")
            count = cursor.fetchone()[0]
            assert count == 0, "Transaction was not rolled back properly"

        finally:
            db.disconnect()

    def test_database_performance(self, db_config):
        """Test database performance under load."""
        import time

        db = Database(db_config)
        db.connect()

        try:
            # Measure simple query performance
            start_time = time.time()

            for _ in range(10):
                db.get_news_count_last_hours(hours=1, table_name="news")

            end_time = time.time()
            total_time = end_time - start_time

            # Should complete in reasonable time
            avg_time = total_time / 10
            assert avg_time < 0.1, f"Query too slow: {avg_time:.3f}s average"

        finally:
            db.disconnect()