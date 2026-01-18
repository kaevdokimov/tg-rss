"""Продвинутая кластеризация новостей с оценкой качества."""

import os
from typing import List, Dict, Any, Tuple, Optional
import numpy as np
import hdbscan

from ..utils.logger import get_logger
from .clustering_quality import ClusteringQualityMetrics

logger = get_logger(__name__)


class NewsClusterer:
    """Продвинутый класс для кластеризации новостей с оценкой качества."""

    def __init__(
        self,
        min_cluster_size: int = 5,
        min_samples: Optional[int] = None,
        metric: str = "cosine",
        cluster_selection_method: str = "eom",
        allow_single_cluster: bool = False,
        cluster_selection_epsilon: float = 0.0,
        alpha: float = 1.0,
        evaluate_quality: bool = True
    ):
        """
        Инициализация продвинутого кластеризатора.

        Args:
            min_cluster_size: Минимальный размер кластера
            min_samples: Минимальные образцы для кластера (по умолчанию min_cluster_size//2)
            metric: Метрика расстояния ("cosine", "euclidean", "manhattan")
            cluster_selection_method: Метод выбора кластеров ("eom", "leaf")
            allow_single_cluster: Разрешить единственный кластер
            cluster_selection_epsilon: Порог для выбора кластеров
            alpha: Параметр для расстояний
            evaluate_quality: Оценивать качество кластеризации
        """
        self.min_cluster_size = min_cluster_size
        self.min_samples = min_samples if min_samples is not None else max(1, min_cluster_size // 2)
        self.metric = metric
        self.cluster_selection_method = cluster_selection_method
        self.allow_single_cluster = allow_single_cluster
        self.cluster_selection_epsilon = cluster_selection_epsilon
        self.alpha = alpha
        self.evaluate_quality = evaluate_quality

        self.clusterer = None
        self._fitted = False
        self.quality_evaluator = ClusteringQualityMetrics() if evaluate_quality else None
        self.last_quality_metrics = None
    
    def fit_predict(self, vectors: List[List[float]]) -> Tuple[List[int], int, int, List[int]]:
        """
        Выполняет продвинутую кластеризацию векторов с оценкой качества.

        Args:
            vectors: Список векторов (каждый вектор - список чисел)

        Returns:
            Кортеж (labels, n_clusters, n_noise, unique_labels)
        """
        logger.info(f"Продвинутая кластеризация {len(vectors)} векторов...")

        # Преобразуем в numpy array с оптимизацией памяти
        X = np.array(vectors, dtype=np.float32)

        # Предварительная обработка метрики
        metric = self.metric
        if metric == "cosine":
            # Нормализуем векторы для cosine distance
            norms = np.linalg.norm(X, axis=1, keepdims=True)
            norms[norms == 0] = 1
            X_normalized = X / norms
            metric = "euclidean"
            logger.debug("Векторы нормализованы для cosine distance")
        else:
            X_normalized = X

        # Определяем количество потоков
        n_jobs = min(4, os.cpu_count() or 1)  # Не более 4 потоков для стабильности

        # Создаем кластеризатор с продвинутыми параметрами
        self.clusterer = hdbscan.HDBSCAN(
            min_cluster_size=self.min_cluster_size,
            min_samples=self.min_samples,
            metric=metric,
            cluster_selection_method=self.cluster_selection_method,
            allow_single_cluster=self.allow_single_cluster,
            cluster_selection_epsilon=self.cluster_selection_epsilon,
            alpha=self.alpha,
            core_dist_n_jobs=n_jobs,
            prediction_data=True
        )

        # Выполняем кластеризацию
        labels = self.clusterer.fit_predict(X_normalized)
        self._fitted = True

        # Статистика кластеризации
        unique_labels = set(labels.tolist())
        n_clusters = len(unique_labels) - (1 if -1 in unique_labels else 0)
        n_noise = int(np.sum(labels == -1))

        logger.info(
            f"Кластеризация завершена: {n_clusters} кластеров, "
            f"{n_noise} шумовых точек из {len(vectors)} новостей"
        )

        # Детальная статистика кластеров
        if n_clusters > 0:
            cluster_sizes = []
            for label in unique_labels:
                if label != -1:
                    size = np.sum(labels == label)
                    cluster_sizes.append(size)

            if cluster_sizes:
                sizes_array = np.array(cluster_sizes)
                logger.info(
                    f"Статистика кластеров: мин={sizes_array.min()}, макс={sizes_array.max()}, "
                    f"среднее={sizes_array.mean():.1f}, std={sizes_array.std():.1f}"
                )

        # Оцениваем качество кластеризации
        if self.evaluate_quality and self.quality_evaluator:
            try:
                self.last_quality_metrics = self.quality_evaluator.evaluate_clustering(
                    vectors, labels.tolist(), n_clusters, n_noise
                )

                quality_score = self.last_quality_metrics.get('overall_quality_score', 0)
                quality_grade = self.last_quality_metrics.get('quality_grade', 'UNKNOWN')

                logger.info("Качество кластеризации",
                    "score", round(quality_score, 3),
                    "grade", quality_grade,
                    "silhouette", round(self.last_quality_metrics.get('silhouette_score', 0), 3)
                )

                # Предупреждения о качестве
                if quality_score < 0.5:
                    logger.warning("Низкое качество кластеризации",
                        "score", round(quality_score, 3),
                        "recommendations", "Увеличьте min_cluster_size или улучшите предобработку"
                    )

            except Exception as e:
                logger.error("Ошибка оценки качества кластеризации", "error", str(e))

        return labels.tolist(), n_clusters, n_noise, list(unique_labels)

    def get_quality_metrics(self) -> Optional[Dict[str, Any]]:
        """Получить метрики качества последней кластеризации."""
        return self.last_quality_metrics

    def optimize_parameters(self, vectors: List[List[float]],
                          parameter_ranges: Dict[str, List]) -> Dict[str, Any]:
        """
        Оптимизация параметров кластеризации.

        Args:
            vectors: Векторы для тестирования
            parameter_ranges: Диапазоны параметров для тестирования

        Returns:
            Лучшие параметры и метрики
        """
        logger.info("Оптимизация параметров кластеризации...")

        best_score = 0
        best_params = {}
        best_metrics = {}

        # Тестируем различные комбинации параметров
        from itertools import product

        param_combinations = list(product(*parameter_ranges.values()))
        param_names = list(parameter_ranges.keys())

        for combo in param_combinations:
            params = dict(zip(param_names, combo))

            try:
                # Создаем кластеризатор с тестовыми параметрами
                test_clusterer = NewsClusterer(
                    min_cluster_size=params.get('min_cluster_size', self.min_cluster_size),
                    min_samples=params.get('min_samples', self.min_samples),
                    metric=params.get('metric', self.metric),
                    evaluate_quality=True
                )

                # Выполняем кластеризацию
                labels, n_clusters, n_noise, _ = test_clusterer.fit_predict(vectors)
                metrics = test_clusterer.get_quality_metrics()

                if metrics:
                    score = metrics.get('overall_quality_score', 0)
                    if score > best_score:
                        best_score = score
                        best_params = params
                        best_metrics = metrics

            except Exception as e:
                logger.debug("Ошибка при тестировании параметров", "params", params, "error", str(e))
                continue

        logger.info("Оптимизация завершена",
            "best_score", round(best_score, 3),
            "best_params", best_params)

        return {
            'best_params': best_params,
            'best_score': best_score,
            'best_metrics': best_metrics
        }
    
    def get_cluster_info(self, labels: List[int]) -> Dict[int, Dict[str, Any]]:
        """
        Возвращает информацию о кластерах.
        
        Args:
            labels: Список меток кластеров
            
        Returns:
            Словарь {cluster_id: {size: int, indices: List[int]}}
        """
        cluster_info = {}
        
        for idx, label in enumerate(labels):
            if label == -1:  # Пропускаем шум
                continue
            
            if label not in cluster_info:
                cluster_info[label] = {"size": 0, "indices": []}
            
            cluster_info[label]["size"] += 1
            cluster_info[label]["indices"].append(idx)
        
        return cluster_info
