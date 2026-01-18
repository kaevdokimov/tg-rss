"""Метрики качества кластеризации для оценки результатов HDBSCAN."""

import numpy as np
from typing import List, Dict, Any, Optional, Tuple
from sklearn.metrics import silhouette_score, calinski_harabasz_score, davies_bouldin_score
from collections import Counter
import math

from ..utils.logger import get_logger

logger = get_logger(__name__)


class ClusteringQualityMetrics:
    """Класс для расчета метрик качества кластеризации."""

    def __init__(self):
        """Инициализация оценщика качества."""
        self.metrics = {}

    def evaluate_clustering(self, vectors: List[List[float]],
                          labels: List[int],
                          n_clusters: int,
                          n_noise: int) -> Dict[str, Any]:
        """
        Полная оценка качества кластеризации.

        Args:
            vectors: Векторы текстов
            labels: Метки кластеров
            n_clusters: Количество кластеров
            n_noise: Количество шумовых точек

        Returns:
            Словарь с метриками качества
        """
        try:
            X = np.array(vectors)

            metrics = {
                'n_clusters': n_clusters,
                'n_noise': n_noise,
                'noise_ratio': n_noise / len(labels) if labels else 0,
                'total_samples': len(labels),
                'clustered_samples': len(labels) - n_noise,
                'clustering_ratio': (len(labels) - n_noise) / len(labels) if labels else 0
            }

            # Статистика размеров кластеров
            cluster_stats = self._calculate_cluster_stats(labels)
            metrics.update(cluster_stats)

            # Внешние метрики качества (требуют значительного количества данных)
            if len(X) > 10 and n_clusters > 1 and n_clusters < len(X) - n_noise:
                try:
                    external_metrics = self._calculate_external_metrics(X, labels, n_clusters)
                    metrics.update(external_metrics)
                except Exception as e:
                    logger.warning("Failed to calculate external metrics", "error", str(e))

            # Внутренние метрики качества
            internal_metrics = self._calculate_internal_metrics(labels, n_clusters, n_noise)
            metrics.update(internal_metrics)

            # Оценка стабильности кластеров
            stability_metrics = self._assess_cluster_stability(labels, n_clusters)
            metrics.update(stability_metrics)

            # Финальная оценка
            quality_score = self._calculate_overall_quality_score(metrics)
            metrics['overall_quality_score'] = quality_score
            metrics['quality_grade'] = self._get_quality_grade(quality_score)

            logger.info("Clustering quality evaluation completed",
                "clusters", n_clusters,
                "noise_ratio", round(metrics['noise_ratio'], 3),
                "quality_score", round(quality_score, 3),
                "grade", metrics['quality_grade'])

            return metrics

        except Exception as e:
            logger.error("Error in clustering quality evaluation", "error", str(e))
            return {
                'error': str(e),
                'n_clusters': n_clusters,
                'n_noise': n_noise,
                'quality_score': 0.0,
                'quality_grade': 'ERROR'
            }

    def _calculate_cluster_stats(self, labels: List[int]) -> Dict[str, Any]:
        """Расчет статистики размеров кластеров."""
        cluster_sizes = Counter(labels)
        # Удаляем шумовые точки (-1)
        if -1 in cluster_sizes:
            del cluster_sizes[-1]

        if not cluster_sizes:
            return {
                'avg_cluster_size': 0,
                'max_cluster_size': 0,
                'min_cluster_size': 0,
                'cluster_size_std': 0,
                'cluster_size_variance': 0,
                'dominant_cluster_ratio': 0,
                'size_distribution': {}
            }

        sizes = list(cluster_sizes.values())
        total_clustered = sum(sizes)

        return {
            'avg_cluster_size': np.mean(sizes),
            'max_cluster_size': max(sizes),
            'min_cluster_size': min(sizes),
            'cluster_size_std': np.std(sizes),
            'cluster_size_variance': np.var(sizes),
            'dominant_cluster_ratio': max(sizes) / total_clustered if total_clustered > 0 else 0,
            'size_distribution': dict(cluster_sizes)
        }

    def _calculate_external_metrics(self, X: np.ndarray, labels: List[int], n_clusters: int) -> Dict[str, float]:
        """Расчет внешних метрик качества (требуют вычислений по всем парам точек)."""
        # Фильтруем шумовые точки для расчета метрик
        valid_indices = [i for i, label in enumerate(labels) if label != -1]
        if len(valid_indices) < 2:
            return {}

        X_filtered = X[valid_indices]
        labels_filtered = [labels[i] for i in valid_indices]

        metrics = {}

        try:
            # Silhouette Score (мера того, насколько объект похож на свой кластер по сравнению с другими кластерами)
            # Диапазон: [-1, 1], где 1 - хорошо разделенные кластеры
            if len(set(labels_filtered)) > 1:
                silhouette = silhouette_score(X_filtered, labels_filtered)
                metrics['silhouette_score'] = float(silhouette)
        except Exception as e:
            logger.debug("Silhouette score calculation failed", "error", str(e))

        try:
            # Calinski-Harabasz Index (отношение суммы межкластерной дисперсии к внутрикластерной)
            # Чем выше, тем лучше
            if len(set(labels_filtered)) > 1:
                ch_score = calinski_harabasz_score(X_filtered, labels_filtered)
                metrics['calinski_harabasz_score'] = float(ch_score)
        except Exception as e:
            logger.debug("Calinski-Harabasz score calculation failed", "error", str(e))

        try:
            # Davies-Bouldin Index (среднее отношение внутрикластерного расстояния к межкластерному)
            # Чем ниже, тем лучше
            if len(set(labels_filtered)) > 1:
                db_score = davies_bouldin_score(X_filtered, labels_filtered)
                metrics['davies_bouldin_score'] = float(db_score)
        except Exception as e:
            logger.debug("Davies-Bouldin score calculation failed", "error", str(e))

        return metrics

    def _calculate_internal_metrics(self, labels: List[int], n_clusters: int, n_noise: int) -> Dict[str, Any]:
        """Расчет внутренних метрик качества."""
        total_samples = len(labels)

        # Метрика баланса кластеров
        if n_clusters > 0:
            expected_size = (total_samples - n_noise) / n_clusters
            cluster_sizes = Counter(labels)
            if -1 in cluster_sizes:
                del cluster_sizes[-1]

            sizes = list(cluster_sizes.values())
            balance_score = 1.0 - (np.std(sizes) / expected_size) if expected_size > 0 else 0
            balance_score = max(0, min(1, balance_score))  # Нормализуем в [0, 1]
        else:
            balance_score = 0.0

        # Метрика покрытия (доля кластеризованных точек)
        coverage_score = (total_samples - n_noise) / total_samples if total_samples > 0 else 0

        # Метрика компактности (основана на количестве кластеров относительно размера данных)
        if total_samples > 0:
            compactness_score = min(1.0, max(0.1, 1.0 - (n_clusters / math.log(total_samples + 1))))
        else:
            compactness_score = 0.0

        return {
            'balance_score': balance_score,
            'coverage_score': coverage_score,
            'compactness_score': compactness_score,
            'efficiency_score': (balance_score + coverage_score + compactness_score) / 3.0
        }

    def _assess_cluster_stability(self, labels: List[int], n_clusters: int) -> Dict[str, Any]:
        """Оценка стабильности кластеров."""
        cluster_sizes = Counter(labels)
        if -1 in cluster_sizes:
            del cluster_sizes[-1]

        stability_metrics = {
            'single_point_clusters': 0,
            'large_clusters': 0,
            'small_clusters': 0,
            'medium_clusters': 0
        }

        total_clustered = sum(cluster_sizes.values())

        for size in cluster_sizes.values():
            if size == 1:
                stability_metrics['single_point_clusters'] += 1
            elif size > total_clustered * 0.3:  # > 30% данных
                stability_metrics['large_clusters'] += 1
            elif size < 5:  # < 5 точек
                stability_metrics['small_clusters'] += 1
            else:
                stability_metrics['medium_clusters'] += 1

        # Расчет индекса стабильности
        if n_clusters > 0:
            unstable_ratio = (stability_metrics['single_point_clusters'] +
                            stability_metrics['large_clusters']) / n_clusters
            stability_score = max(0, 1.0 - unstable_ratio)
        else:
            stability_score = 0.0

        stability_metrics['stability_score'] = stability_score
        return stability_metrics

    def _calculate_overall_quality_score(self, metrics: Dict[str, Any]) -> float:
        """Расчет общей оценки качества кластеризации."""
        weights = {
            'silhouette_score': 0.25,
            'calinski_harabasz_score': 0.15,
            'coverage_score': 0.20,
            'balance_score': 0.15,
            'stability_score': 0.15,
            'efficiency_score': 0.10
        }

        score = 0.0
        total_weight = 0.0

        for metric, weight in weights.items():
            if metric in metrics:
                value = metrics[metric]

                # Нормализуем значения в [0, 1]
                if metric == 'silhouette_score':
                    # [-1, 1] -> [0, 1]
                    normalized = (value + 1) / 2
                elif metric == 'davies_bouldin_score':
                    # [0, inf) -> [1, 0] (чем меньше, тем лучше)
                    normalized = max(0, 1 - value / 2)
                elif metric in ['calinski_harabasz_score']:
                    # [0, inf) -> [0, 1] (чем больше, тем лучше)
                    normalized = min(1.0, value / 1000)  # Предполагаем, что 1000 - хороший порог
                else:
                    # Уже в [0, 1]
                    normalized = value

                score += normalized * weight
                total_weight += weight

        return score / total_weight if total_weight > 0 else 0.0

    def _get_quality_grade(self, score: float) -> str:
        """Получение оценки качества в текстовом виде."""
        if score >= 0.8:
            return "EXCELLENT"
        elif score >= 0.7:
            return "GOOD"
        elif score >= 0.6:
            return "FAIR"
        elif score >= 0.4:
            return "POOR"
        else:
            return "BAD"

    def compare_clusterings(self, metrics1: Dict[str, Any], metrics2: Dict[str, Any]) -> Dict[str, Any]:
        """
        Сравнение двух кластеризаций.

        Args:
            metrics1: Метрики первой кластеризации
            metrics2: Метрики второй кластеризации

        Returns:
            Сравнение метрик
        """
        comparison = {}

        for key in set(metrics1.keys()) | set(metrics2.keys()):
            if key in metrics1 and key in metrics2:
                val1, val2 = metrics1[key], metrics2[key]
                if isinstance(val1, (int, float)) and isinstance(val2, (int, float)):
                    comparison[f"{key}_diff"] = val2 - val1
                    comparison[f"{key}_better"] = "second" if val2 > val1 else "first"
                else:
                    comparison[f"{key}_comparison"] = f"{val1} vs {val2}"

        # Определяем победителя
        score1 = metrics1.get('overall_quality_score', 0)
        score2 = metrics2.get('overall_quality_score', 0)

        comparison['winner'] = "second" if score2 > score1 else "first"
        comparison['score_diff'] = score2 - score1

        return comparison