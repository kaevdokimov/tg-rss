"""Prometheus метрики для news-analyzer."""

from prometheus_client import Counter, Histogram, Gauge, Info
import time
from typing import Dict, Any


# Счетчики
news_analysis_total = Counter(
    'news_analysis_total',
    'Total number of news analysis runs',
    ['status']  # success, error
)

news_processed_total = Counter(
    'news_processed_total',
    'Total number of news items processed'
)

clusters_created_total = Counter(
    'clusters_created_total',
    'Total number of clusters created'
)

telegram_reports_sent_total = Counter(
    'telegram_reports_sent_total',
    'Total number of Telegram reports sent',
    ['status']  # success, error
)

# Гистограммы (для измерения длительности)
analysis_duration = Histogram(
    'analysis_duration_seconds',
    'Time spent on news analysis',
    buckets=[1, 5, 10, 30, 60, 120, 300, 600]  # в секундах
)

preprocessing_duration = Histogram(
    'preprocessing_duration_seconds',
    'Time spent on text preprocessing',
    buckets=[0.1, 0.5, 1, 2, 5, 10, 30]
)

vectorization_duration = Histogram(
    'vectorization_duration_seconds',
    'Time spent on text vectorization',
    buckets=[0.1, 0.5, 1, 2, 5, 10, 30]
)

clustering_duration = Histogram(
    'clustering_duration_seconds',
    'Time spent on news clustering',
    buckets=[0.1, 0.5, 1, 2, 5, 10, 30]
)

narrative_building_duration = Histogram(
    'narrative_building_duration_seconds',
    'Time spent on narrative building',
    buckets=[0.1, 0.5, 1, 2, 5, 10]
)

# Gauges (текущие значения)
news_count_current = Gauge(
    'news_count_current',
    'Current number of news items in analysis'
)

clusters_count_current = Gauge(
    'clusters_count_current',
    'Current number of clusters created'
)

noise_ratio_current = Gauge(
    'noise_ratio_current',
    'Current ratio of noise points in clustering (0.0-1.0)'
)

avg_cluster_size_current = Gauge(
    'avg_cluster_size_current',
    'Current average cluster size'
)

# Info метрики
app_version = Info(
    'news_analyzer_version',
    'News analyzer application version'
)

# Кастомные метрики для качества кластеризации
clustering_quality_silhouette = Gauge(
    'clustering_quality_silhouette',
    'Silhouette score for clustering quality (-1 to 1)'
)

clustering_quality_calinski = Gauge(
    'clustering_quality_calinski_harabasz',
    'Calinski-Harabasz index for clustering quality'
)

# Метрики базы данных
db_connection_duration = Histogram(
    'db_connection_duration_seconds',
    'Time spent establishing database connections',
    buckets=[0.01, 0.05, 0.1, 0.5, 1, 2, 5]
)

db_query_duration = Histogram(
    'db_query_duration_seconds',
    'Time spent on database queries',
    ['query_type'],  # select, insert, update
    buckets=[0.01, 0.05, 0.1, 0.5, 1, 2, 5]
)

# Метрики кэша
cache_hits_total = Counter(
    'cache_hits_total',
    'Total number of cache hits',
    ['cache_type']  # vectorizer, cluster, narrative
)

cache_misses_total = Counter(
    'cache_misses_total',
    'Total number of cache misses',
    ['cache_type']  # vectorizer, cluster, narrative
)

cache_size_current = Gauge(
    'cache_size_current',
    'Current size of cache',
    ['cache_type']  # vectorizer, cluster, narrative
)

cache_hits_total = Counter(
    'cache_hits_total',
    'Total number of cache hits',
    ['cache_type']  # vectors, clusters, narratives
)

cache_misses_total = Counter(
    'cache_misses_total',
    'Total number of cache misses',
    ['cache_type']  # vectors, clusters, narratives
)


class MetricsManager:
    """Менеджер для управления метриками."""

    def __init__(self):
        self._start_times = {}

    def start_analysis(self) -> str:
        """Начинает отсчет времени анализа."""
        analysis_id = str(time.time())
        self._start_times[analysis_id] = time.time()
        return analysis_id

    def end_analysis(self, analysis_id: str, status: str = "success"):
        """Завершает анализ и обновляет метрики."""
        if analysis_id in self._start_times:
            duration = time.time() - self._start_times[analysis_id]
            analysis_duration.observe(duration)
            news_analysis_total.labels(status=status).inc()
            del self._start_times[analysis_id]

    def record_preprocessing(self, duration: float, news_count: int):
        """Записывает метрики предобработки."""
        preprocessing_duration.observe(duration)
        news_count_current.set(news_count)

    def record_vectorization(self, duration: float):
        """Записывает метрики векторизации."""
        vectorization_duration.observe(duration)

    def record_clustering(self, duration: float, n_clusters: int, n_noise: int, total_news: int):
        """Записывает метрики кластеризации."""
        clustering_duration.observe(duration)
        clusters_created_total.inc(n_clusters)

        clusters_count_current.set(n_clusters)
        if total_news > 0:
            noise_ratio_current.set(n_noise / total_news)

        # Расчет среднего размера кластера
        if n_clusters > 0:
            avg_cluster_size_current.set((total_news - n_noise) / n_clusters)

    def record_narrative_building(self, duration: float):
        """Записывает метрики построения нарративов."""
        narrative_building_duration.observe(duration)

    def record_telegram_report(self, status: str = "success"):
        """Записывает метрики отправки Telegram отчетов."""
        telegram_reports_sent_total.labels(status=status).inc()

    def update_clustering_quality(self, silhouette_score: float = None, calinski_score: float = None):
        """Обновляет метрики качества кластеризации."""
        if silhouette_score is not None:
            clustering_quality_silhouette.set(silhouette_score)
        if calinski_score is not None:
            clustering_quality_calinski.set(calinski_score)

    def record_db_connection(self, duration: float):
        """Записывает метрики подключения к БД."""
        db_connection_duration.observe(duration)

    def record_db_query(self, duration: float, query_type: str):
        """Записывает метрики запросов к БД."""
        db_query_duration.labels(query_type=query_type).observe(duration)

    def record_cache_hit(self, cache_type: str):
        """Записывает попадание в кэш."""
        cache_hits_total.labels(cache_type=cache_type).inc()

    def record_cache_miss(self, cache_type: str):
        """Записывает промах кэша."""
        cache_misses_total.labels(cache_type=cache_type).inc()

    def update_cache_size(self, cache_type: str, size: int):
        """Обновляет размер кэша."""
        cache_size_current.labels(cache_type=cache_type).set(size)

    def record_news_processed(self, count: int):
        """Записывает количество обработанных новостей."""
        news_processed_total.inc(count)

    def record_cache_hit(self, cache_type: str):
        """Записывает попадание в кэш."""
        cache_hits_total.labels(cache_type=cache_type).inc()

    def record_cache_miss(self, cache_type: str):
        """Записывает промах кэша."""
        cache_misses_total.labels(cache_type=cache_type).inc()

    def update_cache_size(self, cache_type: str, size: int):
        """Обновляет размер кэша."""
        cache_size_current.labels(cache_type=cache_type).set(size)


# Глобальный экземпляр менеджера метрик
metrics_manager = MetricsManager()


def init_metrics(version: str = "1.0.0"):
    """Инициализирует метрики с версией приложения."""
    app_version.info({"version": version})