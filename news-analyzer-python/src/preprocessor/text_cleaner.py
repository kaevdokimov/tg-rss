"""Очистка и предобработка текста для русского языка."""

import re
from typing import List
import nltk
from nltk.corpus import stopwords
from nltk.tokenize import word_tokenize

from ..utils.logger import get_logger

logger = get_logger(__name__)

# Загружаем стоп-слова для русского языка
try:
    russian_stopwords = set(stopwords.words("russian"))
except LookupError:
    logger.warning("NLTK русские стоп-слова не найдены. Запустите: python -m nltk.downloader stopwords")
    russian_stopwords = set()


class TextCleaner:
    """Класс для очистки и предобработки текста."""
    
    def __init__(
        self,
        stopwords_extra: List[str] = None,
        min_word_length: int = 3,
        max_word_length: int = 20
    ):
        """
        Инициализация очистителя текста.
        
        Args:
            stopwords_extra: Дополнительные стоп-слова
            min_word_length: Минимальная длина слова
            max_word_length: Максимальная длина слова
        """
        self.stopwords = russian_stopwords.copy()
        if stopwords_extra:
            self.stopwords.update(stopwords_extra)
        
        self.min_word_length = min_word_length
        self.max_word_length = max_word_length
    
    def clean_text(self, text: str) -> str:
        """
        Очищает текст: удаляет лишние символы, приводит к нижнему регистру.
        
        Args:
            text: Исходный текст
            
        Returns:
            Очищенный текст
        """
        if not text:
            return ""
        
        # Приводим к нижнему регистру
        text = text.lower()
        
        # Удаляем URL
        text = re.sub(r"http\S+|www\.\S+", "", text)
        
        # Удаляем email
        text = re.sub(r"\S+@\S+", "", text)
        
        # Оставляем только буквы, цифры и пробелы
        text = re.sub(r"[^а-яёa-z0-9\s]", " ", text)
        
        # Удаляем множественные пробелы
        text = re.sub(r"\s+", " ", text)
        
        return text.strip()
    
    def tokenize(self, text: str) -> List[str]:
        """
        Токенизирует текст и фильтрует стоп-слова.
        
        Args:
            text: Очищенный текст
            
        Returns:
            Список токенов
        """
        if not text:
            return []
        
        try:
            # Токенизация
            tokens = word_tokenize(text, language="russian")
        except LookupError:
            logger.warning("NLTK punkt не найден. Запустите: python -m nltk.downloader punkt")
            # Простая токенизация по пробелам
            tokens = text.split()
        
        # Фильтрация
        filtered_tokens = []
        for token in tokens:
            # Проверяем длину
            if len(token) < self.min_word_length or len(token) > self.max_word_length:
                continue
            
            # Проверяем стоп-слова
            if token in self.stopwords:
                continue
            
            # Проверяем, что это не только цифры
            if token.isdigit():
                continue
            
            filtered_tokens.append(token)
        
        return filtered_tokens
    
    def preprocess(self, text: str) -> str:
        """
        Полная предобработка: очистка + токенизация + объединение.
        
        Args:
            text: Исходный текст
            
        Returns:
            Предобработанный текст (токены через пробел)
        """
        cleaned = self.clean_text(text)
        tokens = self.tokenize(cleaned)
        return " ".join(tokens)
