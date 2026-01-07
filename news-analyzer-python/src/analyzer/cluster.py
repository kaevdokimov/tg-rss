"""Кластеризация новостей с помощью HDBSCAN."""

import os
from typing import List, Dict, Any
import numpy as np
import hdbscan

from ..utils.logger import get_logger

logger = get_logger(__name__)


class NewsClusterer:
    """Класс для кластеризации новостей."""
    
    def __init__(
        self,
        min_cluster_size: int = 5,
        min_samples: int = 3,
        metric: str = "cosine"
    ):
        """
        Инициализация кластеризатора.
        
        Args:
            min_cluster_size: Минимальный размер кластера
            min_samples: Минимальные образцы для кластера
            metric: Метрика расстояния
        """
        self.min_cluster_size = min_cluster_size
        self.min_samples = min_samples
        self.metric = metric
        self.clusterer = None
        self._fitted = False
    
    def fit_predict(self, vectors: List[List[float]]) -> List[int]:
        """
        Выполняет кластеризацию векторов.
        
        Args:
            vectors: Список векторов (каждый вектор - список чисел)
            
        Returns:
            Список меток кластеров (-1 означает шум/некластеризованные точки)
        """
        logger.info(f"Кластеризация {len(vectors)} векторов...")
        
        # Преобразуем в numpy array с оптимизацией памяти
        X = np.array(vectors, dtype=np.float32)
        
        # Исправление для совместимости с новыми версиями sklearn:
        # Метрика 'cosine' не поддерживается напрямую, поэтому нормализуем векторы
        # и используем 'euclidean', что эквивалентно cosine для нормализованных векторов
        metric = self.metric
        if metric == "cosine":
            # Нормализуем векторы (L2 нормализация)
            norms = np.linalg.norm(X, axis=1, keepdims=True)
            # Избегаем деления на ноль
            norms[norms == 0] = 1
            X = X / norms
            metric = "euclidean"
            logger.debug("Векторы нормализованы для использования cosine distance через euclidean")
        
        # Оптимизация: для больших объемов данных используем более быстрые параметры
        # core_dist_n_jobs позволяет использовать несколько ядер CPU
        # Для контейнера с ограничением 0.1 CPU используем 1 поток
        n_jobs = 1  # Фиксируем в 1 для стабильности в контейнере
        
        # Создаем и обучаем кластеризатор
        self.clusterer = hdbscan.HDBSCAN(
            min_cluster_size=self.min_cluster_size,
            min_samples=self.min_samples,
            metric=metric,
            cluster_selection_method="eom",  # Excess of Mass
            core_dist_n_jobs=n_jobs,  # Параллельная обработка для ускорения
            prediction_data=True  # Сохраняем данные для предсказаний
        )
        
        labels = self.clusterer.fit_predict(X)
        self._fitted = True
        
        # Статистика
        unique_labels = set(labels)
        n_clusters = len(unique_labels) - (1 if -1 in unique_labels else 0)
        n_noise = list(labels).count(-1)

        logger.info(
            f"Найдено {n_clusters} кластеров, "
            f"{n_noise} новостей не кластеризовано (шум) из {len(vectors)} векторов"
        )

        # Отладочная информация о распределении кластеров
        if n_clusters > 0:
            cluster_sizes = []
            for label in unique_labels:
                if label != -1:
                    size = list(labels).count(label)
                    cluster_sizes.append(size)
                    logger.debug(f"Кластер {label}: {size} новостей")

            if cluster_sizes:
                logger.info(
                    f"Размер кластеров: мин={min(cluster_sizes)}, макс={max(cluster_sizes)}, "
                    f"среднее={sum(cluster_sizes)/len(cluster_sizes):.1f}"
                )
        else:
            logger.warning(
                f"Кластеризация не удалась: {n_clusters} кластеров из {len(vectors)} векторов. "
                f"Параметры: min_cluster_size={self.min_cluster_size}, min_samples={self.min_samples}"
            )

        return labels.tolist(), n_clusters, n_noise, unique_labels
    
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
