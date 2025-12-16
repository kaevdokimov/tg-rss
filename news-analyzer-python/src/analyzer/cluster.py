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
        
        # Преобразуем в numpy array
        X = np.array(vectors)
        
        # Оптимизация: для больших объемов данных используем более быстрые параметры
        # core_dist_n_jobs позволяет использовать несколько ядер CPU
        # но может быть слишком агрессивным, поэтому используем умеренное значение
        n_jobs = min(4, os.cpu_count() or 1)  # Ограничиваем количество ядер
        
        # Создаем и обучаем кластеризатор
        self.clusterer = hdbscan.HDBSCAN(
            min_cluster_size=self.min_cluster_size,
            min_samples=self.min_samples,
            metric=self.metric,
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
            f"{n_noise} новостей не кластеризовано (шум)"
        )
        
        return labels.tolist()
    
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
