"""Векторизация текста с помощью TF-IDF."""

from typing import List
from sklearn.feature_extraction.text import TfidfVectorizer

from ..utils.logger import get_logger

logger = get_logger(__name__)


class TextVectorizer:
    """Класс для векторизации текста через TF-IDF."""
    
    def __init__(
        self,
        max_features: int = 5000,
        min_df: int = 2,
        max_df: float = 0.95
    ):
        """
        Инициализация векторизатора.
        
        Args:
            max_features: Максимальное количество признаков
            min_df: Минимальная частота документа
            max_df: Максимальная частота документа (доля)
        """
        self.max_features = max_features
        self.min_df = min_df
        self.max_df = max_df
        self.vectorizer = None
        self._fitted = False
    
    def fit_transform(self, texts: List[str]) -> List[List[float]]:
        """
        Обучает векторизатор и преобразует тексты в векторы.
        
        Args:
            texts: Список предобработанных текстов
            
        Returns:
            Список векторов (каждый вектор - список чисел)
        """
        logger.info(f"Векторизация {len(texts)} текстов...")
        
        self.vectorizer = TfidfVectorizer(
            max_features=self.max_features,
            min_df=self.min_df,
            max_df=self.max_df,
            ngram_range=(1, 2),  # Униграммы и биграммы
            lowercase=False  # Уже в нижнем регистре
        )
        
        # Преобразуем в numpy array, затем в список списков
        vectors = self.vectorizer.fit_transform(texts)
        self._fitted = True
        
        logger.info(f"Создано {vectors.shape[1]} признаков")
        return vectors.toarray().tolist()
    
    def get_feature_names(self) -> List[str]:
        """
        Возвращает названия признаков (слова).
        
        Returns:
            Список названий признаков
        """
        if not self._fitted or self.vectorizer is None:
            raise RuntimeError("Векторизатор еще не обучен. Вызовите fit_transform() сначала.")
        return self.vectorizer.get_feature_names_out().tolist()
