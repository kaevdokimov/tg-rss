"""Configuration for integration tests with testcontainers."""

import pytest
from testcontainers.postgres import PostgresContainer
from testcontainers.redis import RedisContainer
import psycopg2
import redis
import time
from typing import Generator, Tuple


@pytest.fixture(scope="session")
def postgres_container() -> Generator[Tuple[str, str, str, str], None, None]:
    """PostgreSQL container for integration tests."""
    container = PostgresContainer(
        image="postgres:15-alpine",
        username="testuser",
        password="testpass",
        dbname="testdb",
        port=5432
    )

    container.start()

    # Wait for PostgreSQL to be ready
    max_attempts = 30
    for attempt in range(max_attempts):
        try:
            conn = psycopg2.connect(
                host=container.get_container_host_ip(),
                port=container.get_exposed_port(5432),
                user="testuser",
                password="testpass",
                dbname="testdb"
            )
            conn.close()
            break
        except psycopg2.OperationalError:
            if attempt == max_attempts - 1:
                raise
            time.sleep(1)

    host = container.get_container_host_ip()
    port = container.get_exposed_port(5432)

    yield host, port, "testuser", "testpass"

    container.stop()


@pytest.fixture(scope="session")
def redis_container() -> Generator[Tuple[str, int], None, None]:
    """Redis container for integration tests."""
    container = RedisContainer(
        image="redis:7-alpine",
        port=6379
    )

    container.start()

    # Wait for Redis to be ready
    max_attempts = 30
    for attempt in range(max_attempts):
        try:
            r = redis.Redis(
                host=container.get_container_host_ip(),
                port=container.get_exposed_port(6379),
                db=0
            )
            r.ping()
            r.close()
            break
        except redis.ConnectionError:
            if attempt == max_attempts - 1:
                raise
            time.sleep(1)

    host = container.get_container_host_ip()
    port = container.get_exposed_port(6379)

    yield host, port

    container.stop()


@pytest.fixture
def db_config(postgres_container):
    """Database configuration for tests."""
    host, port, user, password = postgres_container
    return {
        "host": host,
        "port": port,
        "user": user,
        "password": password,
        "database": "testdb",
        "table_name": "news"
    }


@pytest.fixture
def redis_config(redis_container):
    """Redis configuration for tests."""
    host, port = redis_container
    return {
        "host": host,
        "port": port,
        "db": 0
    }