"""Построение нарративов (тем) из кластеров новостей."""

from typing import List, Dict, Any, TYPE_CHECKING
from collections import Counter
import numpy as np
from sklearn.feature_extraction.text import TfidfVectorizer

from ..db.database import NewsItem
from ..utils.logger import get_logger

if TYPE_CHECKING:
    from ..analyzer.vectorizer import TextVectorizer

logger = get_logger(__name__)


class NarrativeBuilder:
    """Класс для построения нарративов из кластеров."""
    
    def __init__(self, top_keywords: int = 10, top_titles: int = 5):
        """
        Инициализация builder.
        
        Args:
            top_keywords: Количество ключевых слов для извлечения
            top_titles: Количество репрезентативных заголовков
        """
        self.top_keywords = top_keywords
        self.top_titles = top_titles
    
    def build_narratives(
        self,
        news_items: List[NewsItem],
        labels: List[int],
        vectorizer: "TextVectorizer",
        top_n: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Строит нарративы из кластеров.
        
        Args:
            news_items: Список новостей
            labels: Метки кластеров
            vectorizer: Обученный векторизатор
            top_n: Количество топ-нарративов
            
        Returns:
            Список нарративов (тем)
        """
        logger.info("Построение нарративов из кластеров...")
        
        # Группируем новости по кластерам
        clusters = {}
        for idx, label in enumerate(labels):
            if label == -1:  # Пропускаем шум
                continue
            
            if label not in clusters:
                clusters[label] = []
            clusters[label].append(idx)
        
        # Сортируем кластеры по размеру
        sorted_clusters = sorted(
            clusters.items(),
            key=lambda x: len(x[1]),
            reverse=True
        )
        
        narratives = []
        for cluster_id, indices in sorted_clusters[:top_n]:
            narrative = self._build_single_narrative(
                news_items,
                indices,
                vectorizer,
                cluster_id
            )
            narratives.append(narrative)
        
        logger.info(f"Построено {len(narratives)} нарративов")
        return narratives
    
    def _build_single_narrative(
        self,
        news_items: List[NewsItem],
        indices: List[int],
        vectorizer: "TextVectorizer",
        cluster_id: int
    ) -> Dict[str, Any]:
        """
        Строит один нарратив из кластера.
        
        Args:
            news_items: Все новости
            indices: Индексы новостей в кластере
            vectorizer: Обученный векторизатор
            cluster_id: ID кластера
            
        Returns:
            Словарь с информацией о нарративе
        """
        cluster_news = [news_items[i] for i in indices]
        
        # Извлекаем ключевые слова через TF-IDF внутри кластера
        texts = [item.title for item in cluster_news]
        cluster_vectorizer = TfidfVectorizer(max_features=50, ngram_range=(1, 2))
        try:
            tfidf_matrix = cluster_vectorizer.fit_transform(texts)
            feature_names = cluster_vectorizer.get_feature_names_out()
            
            # Суммируем TF-IDF по всем документам кластера
            scores = np.array(tfidf_matrix.sum(axis=0)).flatten()
            top_indices = scores.argsort()[-self.top_keywords:][::-1]
            keywords = [feature_names[i] for i in top_indices]
        except Exception as e:
            logger.warning(f"Ошибка при извлечении ключевых слов: {e}")
            # Fallback: используем частоту слов
            all_words = []
            for text in texts:
                words = text.lower().split()
                all_words.extend(words)
            word_freq = Counter(all_words)
            keywords = [word for word, _ in word_freq.most_common(self.top_keywords)]
        
        # Репрезентативные заголовки (первые N)
        titles = [item.title for item in cluster_news[:self.top_titles]]
        
        return {
            "cluster_id": cluster_id,
            "size": len(cluster_news),
            "keywords": keywords,
            "titles": titles,
            "news_count": len(cluster_news)
        }
