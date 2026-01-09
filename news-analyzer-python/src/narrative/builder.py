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
    
    def __init__(self, top_keywords: int = 20, top_titles: int = 10):
        """
        Инициализация builder.

        Args:
            top_keywords: Количество ключевых слов для извлечения (увеличено для лучшего описания тем)
            top_titles: Количество репрезентативных заголовков (увеличено для лучшего понимания)
        """
        self.top_keywords = top_keywords
        self.top_titles = top_titles
    
    def build_narratives(
        self,
        news_items: List[NewsItem],
        labels: List[int],
        vectorizer: "TextVectorizer",
        top_n: int = 5,
        processed_texts: Optional[List[str]] = None
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
                cluster_id,
                processed_texts
            )
            narratives.append(narrative)
        
        logger.info(f"Построено {len(narratives)} нарративов")
        return narratives
    
    def _build_single_narrative(
        self,
        news_items: List[NewsItem],
        indices: List[int],
        vectorizer: "TextVectorizer",
        cluster_id: int,
        processed_texts: Optional[List[str]] = None
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
        
        # Извлекаем ключевые слова через улучшенный TF-IDF внутри кластера
        # Используем предобработанные тексты, если они доступны, иначе - сырые заголовки
        if processed_texts:
            texts = [processed_texts[i] for i in indices]
        else:
            texts = [item.title for item in cluster_news]
        try:
            # Используем более качественные параметры для извлечения ключевых слов
            cluster_vectorizer = TfidfVectorizer(
                max_features=150,  # Увеличиваем для лучшего выбора
                ngram_range=(1, 3),  # Триграммы для более точных фраз
                min_df=1,  # Минимум 1 документ
                max_df=0.98,  # Максимум 98% документов для большего разнообразия
                sublinear_tf=True,  # Логарифмическое масштабирование
                use_idf=True,
                norm='l2'  # L2 нормализация для лучшей кластеризации
            )

            tfidf_matrix = cluster_vectorizer.fit_transform(texts)
            feature_names = cluster_vectorizer.get_feature_names_out()

            # Улучшенное ранжирование: комбинируем TF-IDF с частотой
            tfidf_scores = np.array(tfidf_matrix.sum(axis=0)).flatten()

            # Добавляем бонус за частоту встречаемости в заголовках
            word_freq_bonus = {}
            for text in texts:
                words = set(text.lower().split())  # Уникальные слова в каждом заголовке
                for word in words:
                    if word in feature_names:
                        idx = list(feature_names).index(word)
                        word_freq_bonus[idx] = word_freq_bonus.get(idx, 0) + 1

            # Комбинируем TF-IDF с бонусом частоты
            combined_scores = tfidf_scores.copy()
            for idx, bonus in word_freq_bonus.items():
                combined_scores[idx] *= (1 + bonus * 0.1)  # 10% бонус за каждое вхождение

            # Выбираем топ ключевых слов
            top_indices = combined_scores.argsort()[-self.top_keywords:][::-1]
            keywords = [feature_names[i] for i in top_indices]

            # Фильтруем слишком короткие или слишком длинные слова (исключаем 2-буквенные слова)
            keywords = [kw for kw in keywords if 3 <= len(kw) <= 50]

            # Если после фильтрации не осталось ключевых слов, используем fallback
            if not keywords:
                logger.warning("Все ключевые слова отфильтрованы, используем fallback")
                raise Exception("No keywords after filtering")

        except Exception as e:
            logger.warning(f"Ошибка при извлечении ключевых слов через TF-IDF: {e}")
            # Fallback: используем улучшенную частотную модель
            all_words = []
            # Расширенный список стоп-слов для fallback (включает все предлоги, союзы и короткие слова)
            extended_stopwords = {
                # Предлоги
                'в', 'из', 'на', 'для', 'по', 'а', 'от', 'с', 'у', 'о', 'к', 'до', 'после', 'перед', 'между', 'во', 'над', 'под', 'при', 'без', 'за', 'об', 'из-за',
                # Союзы
                'и', 'или', 'если', 'то', 'либо', 'нибудь', 'кое', 'как', 'что', 'это', 'его', 'её', 'их',
                # Другие короткие слова
                'не', 'да', 'ну', 'ой', 'ах', 'ох', 'ух', 'во', 'со', 'ко', 'го', 'то', 'же', 'бы', 'ли', 'ни'
            }

            for text in texts:
                words = text.lower().split()
                # Фильтруем стоп-слова и короткие слова (минимум 3 символа)
                filtered_words = [w for w in words if len(w) >= 3 and w not in extended_stopwords]
                all_words.extend(filtered_words)
            word_freq = Counter(all_words)
            keywords = [word for word, _ in word_freq.most_common(self.top_keywords)]
        
        # Репрезентативные новости (первые N)
        news_examples = []
        for item in cluster_news[:self.top_titles]:
            news_examples.append({
                "title": item.title,
                "link": item.link,
                "source_name": item.source_name or "Неизвестный источник"
            })

        return {
            "cluster_id": cluster_id,
            "size": len(cluster_news),
            "keywords": keywords,
            "news_examples": news_examples,
            "news_count": len(cluster_news)
        }
