"""Unit tests for vectorizer module."""

import pytest
import numpy as np
from unittest.mock import patch

from src.analyzer.vectorizer import TextVectorizer


class TestTextVectorizer:
    """Test cases for TextVectorizer class."""

    def setup_method(self):
        """Set up test fixtures."""
        self.vectorizer = TextVectorizer(
            max_features=100,
            min_df=1,
            max_df=1.0
        )

    def test_init(self):
        """Test TextVectorizer initialization."""
        assert self.vectorizer.max_features == 100
        assert self.vectorizer.min_df == 1
        assert self.vectorizer.max_df == 1.0

    def test_fit_transform_basic(self):
        """Test basic fit_transform functionality."""
        texts = [
            "Это первый тестовый текст",
            "Это второй тестовый текст",
            "Третий текст для тестирования"
        ]

        vectors = self.vectorizer.fit_transform(texts)

        # Проверяем тип результата
        assert isinstance(vectors, list)
        assert len(vectors) == len(texts)

        # Проверяем, что векторы имеют правильную длину
        if vectors and len(vectors) > 0:
            vector_length = len(vectors[0])
            assert vector_length <= 100  # max_features
            # Проверяем, что все векторы одинаковой длины
            assert all(len(v) == vector_length for v in vectors)

    def test_fit_transform_empty_texts(self):
        """Test fit_transform with empty texts."""
        texts = ["", "   ", "ещё текст"]
        vectors = self.vectorizer.fit_transform(texts)

        # Проверяем, что результат не пустой
        assert isinstance(vectors, list)
        assert len(vectors) == len(texts)

    def test_fit_transform_single_text(self):
        """Test fit_transform with single text."""
        texts = ["Один единственный текст для тестирования"]
        vectors = self.vectorizer.fit_transform(texts)

        assert isinstance(vectors, list)
        assert len(vectors) == 1
        assert len(vectors[0]) > 0

    def test_fit_transform_identical_texts(self):
        """Test fit_transform with identical texts."""
        texts = ["Одинаковый текст"] * 3
        vectors = self.vectorizer.fit_transform(texts)

        assert isinstance(vectors, list)
        assert len(vectors) == 3

        # Все векторы должны быть одинаковыми
        if vectors and len(vectors) > 1:
            assert np.allclose(vectors[0], vectors[1])
            assert np.allclose(vectors[1], vectors[2])

    def test_fit_transform_no_common_words(self):
        """Test fit_transform with texts having no common words."""
        texts = [
            "Кошка сидит на крыше",
            "Собака бежит по улице",
            "Птица летит в небе"
        ]

        vectors = self.vectorizer.fit_transform(texts)

        assert isinstance(vectors, list)
        assert len(vectors) == 3

        # Векторы должны быть различными
        if vectors and len(vectors) > 1:
            assert not np.allclose(vectors[0], vectors[1])

    def test_max_features_limit(self):
        """Test max_features parameter."""
        # Создаем много уникальных слов
        texts = []
        for i in range(20):
            words = [f"слово{j}_{i}" for j in range(10)]  # 10 уникальных слов на текст
            texts.append(" ".join(words))

        vectorizer = TextVectorizer(max_features=50)
        vectors = vectorizer.fit_transform(texts)

        # Проверяем, что размерность не превышает max_features
        if vectors and len(vectors) > 0:
            assert len(vectors[0]) <= 50

    def test_min_df_filtering(self):
        """Test min_df parameter filtering."""
        texts = [
            "редкое слово уникальное",  # "редкое" и "уникальное" встречаются 1 раз
            "обычное слово частое",     # "обычное" и "частое" встречаются 1 раз
            "обычное слово частое снова" # "обычное" и "частое" встречаются 2 раза
        ]

        # min_df=2 означает, что слова должны встречаться минимум в 2 документах
        vectorizer = TextVectorizer(min_df=2, max_features=100)
        vectors = vectorizer.fit_transform(texts)

        # "обычное" и "слово" должны быть включены, "редкое" и "уникальное" - нет
        assert isinstance(vectors, list)
        assert len(vectors) == 3

    def test_max_df_filtering(self):
        """Test max_df parameter filtering."""
        texts = [
            "частое слово повсюду",
            "частое слово повсюду",
            "частое слово повсюду",
            "редкое слово уникальное"
        ]

        # max_df=0.8 означает, что слова не должны встречаться более чем в 80% документов
        vectorizer = TextVectorizer(max_df=0.8, max_features=100)
        vectors = vectorizer.fit_transform(texts)

        # "частое", "слово", "повсюду" встречаются во всех документах (100% > 80%), должны быть отфильтрованы
        assert isinstance(vectors, list)
        assert len(vectors) == 4

    def test_vector_values_range(self):
        """Test that vector values are in expected range."""
        texts = ["Это тестовый текст для проверки значений векторов"]
        vectors = self.vectorizer.fit_transform(texts)

        if vectors and len(vectors) > 0:
            vector = vectors[0]
            # TF-IDF значения обычно неотрицательные
            assert all(v >= 0 for v in vector)

    def test_vector_sparsity(self):
        """Test vector sparsity (most values should be zero)."""
        texts = [
            "Кошка сидит на крыше",
            "Собака бежит по улице",
            "Птица летит в небе"
        ]

        vectors = self.vectorizer.fit_transform(texts)

        if vectors and len(vectors) > 0:
            vector = vectors[0]
            # Подсчитываем количество ненулевых значений
            non_zero_count = sum(1 for v in vector if v > 1e-10)
            sparsity_ratio = non_zero_count / len(vector)

            # TF-IDF векторы обычно разрежены (большинство значений близки к нулю)
            assert sparsity_ratio < 0.5  # Менее 50% ненулевых значений

    @patch('sklearn.feature_extraction.text.TfidfVectorizer.fit_transform')
    def test_sklearn_error_handling(self, mock_fit_transform):
        """Test error handling when sklearn fails."""
        mock_fit_transform.side_effect = Exception("sklearn error")

        texts = ["Тестовый текст"]
        with pytest.raises(Exception):
            self.vectorizer.fit_transform(texts)

    def test_memory_efficiency(self):
        """Test memory efficiency with large vocabulary."""
        # Создаем тексты с большим количеством уникальных слов
        texts = []
        for i in range(10):
            words = [f"word_{i}_{j}" for j in range(50)]  # 50 уникальных слов на текст
            texts.append(" ".join(words))

        vectorizer = TextVectorizer(max_features=100)  # Ограничиваем словарь
        vectors = vectorizer.fit_transform(texts)

        # Проверяем, что векторы созданы и имеют правильный размер
        assert isinstance(vectors, list)
        assert len(vectors) == 10
        if vectors and len(vectors[0]) > 0:
            assert len(vectors[0]) <= 100  # Не превышает max_features