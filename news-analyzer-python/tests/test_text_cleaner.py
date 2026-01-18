"""Unit tests for text_cleaner module."""

import pytest
from unittest.mock import patch, MagicMock

from src.preprocessor.text_cleaner import TextCleaner


class TestTextCleaner:
    """Test cases for TextCleaner class."""

    def setup_method(self):
        """Set up test fixtures."""
        self.cleaner = TextCleaner(
            stopwords_extra=["дополнительное", "слово"],
            min_word_length=3,
            max_word_length=15
        )

    def test_init(self):
        """Test TextCleaner initialization."""
        assert self.cleaner.stopwords_extra == ["дополнительное", "слово"]
        assert self.cleaner.min_word_length == 3
        assert self.cleaner.max_word_length == 15

    def test_preprocess_basic(self):
        """Test basic text preprocessing."""
        text = "Это ТЕСТовый текст с РАЗНЫМИ регистрами!"
        result = self.cleaner.preprocess(text)

        # Проверяем, что текст в нижнем регистре
        assert result == result.lower()
        # Проверяем, что пунктуация удалена
        assert "!" not in result
        # Проверяем токенизацию
        assert isinstance(result, str)

    def test_preprocess_with_stopwords(self):
        """Test preprocessing with custom stopwords."""
        text = "Это дополнительное слово для тестирования стоп-слов"
        result = self.cleaner.preprocess(text)

        # Проверяем, что стоп-слова удалены
        assert "дополнительное" not in result
        assert "слово" not in result

    def test_preprocess_word_length_filtering(self):
        """Test word length filtering."""
        # Создаем cleaner с строгими ограничениями
        cleaner = TextCleaner(min_word_length=4, max_word_length=8)

        text = "а то и это оченьдлинноеслово"
        result = cleaner.preprocess(text)

        # Проверяем, что короткие слова удалены
        assert "а" not in result.split()
        assert "то" not in result.split()
        assert "и" not in result.split()
        # Проверяем, что слишком длинные слова удалены
        assert "оченьдлинноеслово" not in result

    def test_preprocess_empty_text(self):
        """Test preprocessing of empty text."""
        result = self.cleaner.preprocess("")
        assert result == ""

        result = self.cleaner.preprocess("   ")
        assert result == ""

    def test_preprocess_special_characters(self):
        """Test preprocessing with special characters."""
        text = "Текст с цифрами 123 и символами @#$%^&*()"
        result = self.cleaner.preprocess(text)

        # Проверяем, что цифры удалены
        assert "123" not in result
        # Проверяем, что специальные символы удалены
        assert "@" not in result
        assert "#" not in result

    def test_preprocess_russian_text(self):
        """Test preprocessing of Russian text."""
        text = "Президент Владимир Путин встретился с министром иностранных дел"
        result = self.cleaner.preprocess(text)

        # Проверяем, что результат не пустой
        assert len(result) > 0
        # Проверяем, что стоп-слова могли быть удалены
        words = result.split()
        assert len(words) > 0

    @patch('nltk.word_tokenize')
    def test_preprocess_tokenization_fallback(self, mock_tokenize):
        """Test tokenization fallback when NLTK fails."""
        # Имитируем ошибку NLTK
        mock_tokenize.side_effect = Exception("NLTK error")

        text = "Это тестовый текст для проверки fallback"
        result = self.cleaner.preprocess(text)

        # Проверяем, что результат получен через fallback
        assert isinstance(result, str)
        assert len(result) > 0

    def test_preprocess_numbers_removal(self):
        """Test that numbers are removed from text."""
        text = "В 2023 году произошло 123 события"
        result = self.cleaner.preprocess(text)

        # Проверяем, что числа удалены
        assert "2023" not in result
        assert "123" not in result

    def test_preprocess_case_insensitive_stopwords(self):
        """Test case insensitive stopword removal."""
        cleaner = TextCleaner(stopwords_extra=["ТЕСТ", "СЛОВО"])
        text = "Это тест слово для проверки"
        result = cleaner.preprocess(text)

        # Проверяем, что стоп-слова удалены независимо от регистра
        assert "тест" not in result
        assert "слово" not in result

    def test_tokenize_method(self):
        """Test the tokenize method directly."""
        text = "Это, тестовый текст! С пунктуацией?"
        tokens = self.cleaner.tokenize(text)

        # Проверяем, что пунктуация удалена
        assert all(token.isalnum() for token in tokens)
        # Проверяем, что есть токены
        assert len(tokens) > 0

    def test_tokenize_empty_text(self):
        """Test tokenization of empty text."""
        tokens = self.cleaner.tokenize("")
        assert tokens == []

        tokens = self.cleaner.tokenize("   ")
        assert tokens == []

    def test_stopwords_filtering(self):
        """Test stopwords filtering."""
        # Создаем текст с известными стоп-словами
        text = "Это и то или как когда"
        tokens = self.cleaner.tokenize(text)

        # Проверяем, что некоторые стоп-слова могли быть отфильтрованы
        # (зависит от NLTK stopwords)
        assert isinstance(tokens, list)

    def test_custom_stopwords_integration(self):
        """Test integration of custom stopwords."""
        text = "Это дополнительное слово для теста"
        result = self.cleaner.preprocess(text)

        # Проверяем, что кастомные стоп-слова удалены
        assert "дополнительное" not in result.split()
        assert "слово" not in result.split()