"""Продвинутая очистка и предобработка текста для русского языка."""

import re
from typing import List, Optional, Dict, Set
import nltk
from nltk.corpus import stopwords
from nltk.tokenize import word_tokenize
from collections import Counter
import unicodedata

try:
    import pymorphy2
    MORPH_AVAILABLE = True
except ImportError:
    MORPH_AVAILABLE = False
    logger.warning("pymorphy2 не установлен. Лемматизация будет недоступна.")

from ..utils.logger import get_logger

logger = get_logger(__name__)

# Для обратной совместимости
TextCleaner = AdvancedTextCleaner

# Флаг для отслеживания загрузки NLTK данных
_nltk_initialized = False

def _ensure_nltk_data():
    """Обеспечивает загрузку необходимых NLTK данных."""
    global _nltk_initialized
    if _nltk_initialized:
        return

    try:
        # Проверяем punkt
        nltk.data.find('tokenizers/punkt')
    except LookupError:
        logger.warning("Загружаем NLTK punkt данные...")
        try:
            nltk.download('punkt', quiet=True)
            logger.info("✓ NLTK punkt данные загружены")
        except Exception as e:
            logger.error(f"Не удалось загрузить NLTK punkt: {e}")

    try:
        # Проверяем stopwords
        nltk.data.find('corpora/stopwords')
    except LookupError:
        logger.warning("Загружаем NLTK stopwords...")
        try:
            nltk.download('stopwords', quiet=True)
            logger.info("✓ NLTK stopwords загружены")
        except Exception as e:
            logger.error(f"Не удалось загрузить NLTK stopwords: {e}")

    _nltk_initialized = True

# Продвинутые стоп-слова для русского языка (расширенный набор)
try:
    russian_stopwords = set(stopwords.words("russian"))
    # Расширяем стоп-слова специфичными для новостей
    news_stopwords = {
        "новость", "сообщает", "заявил", "отметил", "сообщил", "сказал",
        "сообщает", "передает", "пишет", "рассказал", "подчеркнул",
        "отметила", "заявила", "сообщила", "сказала", "добавил", "добавила",
        "сообщили", "заявили", "отметили", "рассказали", "подчеркнули",
        "сообщение", "информация", "данные", "сведения", "материал",
        "статья", "публикация", "текст", "источник", "агентство",
        "корреспондент", "журналист", "редакция", "издание",
        "газета", "журнал", "сайт", "портал", "ресурс",
        "фото", "видео", "инфографика", "графика", "иллюстрация"
    }
    russian_stopwords.update(news_stopwords)
except LookupError:
    logger.warning("NLTK русские стоп-слова не найдены. Запустите: python -m nltk.downloader stopwords")
    russian_stopwords = set()

# Регулярные выражения для очистки текста
URL_PATTERN = re.compile(r'https?://[^\s]+|www\.[^\s]+')
EMAIL_PATTERN = re.compile(r'\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b')
PHONE_PATTERN = re.compile(r'\b\d{3}[-.]?\d{3}[-.]?\d{4}\b|\+\d{1,3}[-.\s]?\d{3}[-.\s]?\d{3}[-.\s]?\d{4}')
HTML_PATTERN = re.compile(r'<[^>]+>')
MULTISPACE_PATTERN = re.compile(r'\s+')

class AdvancedTextCleaner:
    """Продвинутый очистчик текста с лемматизацией и нормализацией."""

    def __init__(self,
                 use_lemmatization: bool = True,
                 remove_urls: bool = True,
                 remove_emails: bool = True,
                 remove_numbers: bool = True,
                 normalize_unicode: bool = True,
                 custom_stopwords: Optional[Set[str]] = None,
                 min_word_length: int = 3,
                 max_word_length: int = 20):
        """
        Инициализация продвинутого очистчика текста.

        Args:
            use_lemmatization: Использовать лемматизацию
            remove_urls: Удалять URL'ы
            remove_emails: Удалять email адреса
            remove_numbers: Удалять числа
            normalize_unicode: Нормализовать Unicode символы
            custom_stopwords: Дополнительные стоп-слова
            min_word_length: Минимальная длина слова
            max_word_length: Максимальная длина слова
        """
        self.use_lemmatization = use_lemmatization and MORPH_AVAILABLE
        self.remove_urls = remove_urls
        self.remove_emails = remove_emails
        self.remove_numbers = remove_numbers
        self.normalize_unicode = normalize_unicode
        self.min_word_length = min_word_length
        self.max_word_length = max_word_length

        # Объединяем стоп-слова
        self.stopwords = russian_stopwords.copy()
        if custom_stopwords:
            self.stopwords.update(custom_stopwords)

        # Инициализируем лемматизатор
        self.morph = None
        if self.use_lemmatization:
            try:
                self.morph = pymorphy2.MorphAnalyzer()
                logger.info("✓ Лемматизатор pymorphy2 инициализирован")
            except Exception as e:
                logger.warning(f"Не удалось инициализировать лемматизатор: {e}")
                self.use_lemmatization = False

        logger.info("AdvancedTextCleaner инициализирован", "lemmatization", self.use_lemmatization)

    def preprocess(self, text: str) -> str:
        """
        Полная предобработка текста.

        Args:
            text: Исходный текст

        Returns:
            Очищенный и обработанный текст
        """
        if not text or not text.strip():
            return ""

        try:
            # 1. Нормализация Unicode
            if self.normalize_unicode:
                text = self._normalize_unicode(text)

            # 2. Удаление HTML тегов
            text = HTML_PATTERN.sub(' ', text)

            # 3. Удаление URL'ов
            if self.remove_urls:
                text = URL_PATTERN.sub(' ', text)

            # 4. Удаление email'ов
            if self.remove_emails:
                text = EMAIL_PATTERN.sub(' ', text)

            # 5. Удаление телефонных номеров
            text = PHONE_PATTERN.sub(' ', text)

            # 6. Приведение к нижнему регистру
            text = text.lower()

            # 7. Удаление пунктуации и специальных символов (кроме кириллицы и пробелов)
            text = re.sub(r'[^\w\s\u0400-\u04FF]', ' ', text)

            # 8. Удаление чисел
            if self.remove_numbers:
                text = re.sub(r'\d+', ' ', text)

            # 9. Нормализация пробелов
            text = MULTISPACE_PATTERN.sub(' ', text).strip()

            # 10. Токенизация
            tokens = self._tokenize(text)

            # 11. Фильтрация по длине слов
            tokens = [token for token in tokens
                     if self.min_word_length <= len(token) <= self.max_word_length]

            # 12. Удаление стоп-слов
            tokens = [token for token in tokens if token not in self.stopwords]

            # 13. Лемматизация (если доступна)
            if self.use_lemmatization and self.morph:
                tokens = [self._lemmatize(token) for token in tokens]

            # 14. Финальная фильтрация пустых токенов
            tokens = [token for token in tokens if token.strip()]

            return ' '.join(tokens)

        except Exception as e:
            logger.error("Ошибка при предобработке текста", "error", str(e), "text_length", len(text))
            # Fallback: базовая очистка
            return self._basic_preprocess(text)

    def _normalize_unicode(self, text: str) -> str:
        """Нормализация Unicode символов."""
        try:
            # NFKC нормализация (композиция + совместимость)
            text = unicodedata.normalize('NFKC', text)
            # Заменяем похожие символы на стандартные
            replacements = {
                '"': '"', '"': '"', ''': "'", ''': "'",
                '–': '-', '—': '-', '…': '...',
                '№': 'N', '°': 'grad'
            }
            for old, new in replacements.items():
                text = text.replace(old, new)
            return text
        except Exception:
            return text

    def _tokenize(self, text: str) -> List[str]:
        """Умная токенизация с fallback."""
        try:
            # Сначала пытаемся использовать NLTK
            _ensure_nltk_data()
            tokens = word_tokenize(text, language='russian')
            return tokens
        except Exception as e:
            logger.debug("NLTK tokenization failed, using fallback", "error", str(e))
            # Fallback: простая токенизация по пробелам
            return text.split()

    def _lemmatize(self, word: str) -> str:
        """Лемматизация слова."""
        if not self.morph:
            return word

        try:
            parsed = self.morph.parse(word)[0]
            return parsed.normal_form
        except Exception:
            return word

    def _basic_preprocess(self, text: str) -> str:
        """Базовая предобработка в случае ошибки."""
        text = text.lower()
        text = re.sub(r'[^\w\s\u0400-\u04FF]', ' ', text)
        text = MULTISPACE_PATTERN.sub(' ', text).strip()
        words = text.split()
        words = [w for w in words if self.min_word_length <= len(w) <= self.max_word_length]
        return ' '.join(words)

    def get_text_stats(self, text: str) -> Dict[str, int]:
        """Получение статистики текста."""
        stats = {
            'original_length': len(text),
            'words_count': len(text.split()),
            'sentences_count': len(re.split(r'[.!?]+', text)) - 1,
            'urls_count': len(URL_PATTERN.findall(text)),
            'emails_count': len(EMAIL_PATTERN.findall(text)),
        }

        processed = self.preprocess(text)
        stats.update({
            'processed_length': len(processed),
            'processed_words': len(processed.split()) if processed else 0,
            'compression_ratio': len(processed) / len(text) if text else 0
        })

        return stats


class TextCleaner:
    """Класс для очистки и предобработки текста."""
    
    def __init__(
        self,
        stopwords_extra: Optional[List[str]] = None,
        min_word_length: int = 2,
        max_word_length: int = 30
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
        
        # Оставляем буквы, цифры, пробелы и дефисы (для составных слов)
        text = re.sub(r"[^а-яёa-z0-9\s\-]", " ", text)
        
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

        # Обеспечиваем загрузку NLTK данных
        _ensure_nltk_data()

        try:
            # Токенизация
            tokens = word_tokenize(text, language="russian")
        except LookupError:
            logger.warning("NLTK punkt все еще не найден после загрузки, используем fallback")
            # Простая токенизация по пробелам
            tokens = text.split()
        
        # Фильтрация
        filtered_tokens = []
        for token in tokens:
            # Проверяем длину
            if len(token) < self.min_word_length or len(token) > self.max_word_length:
                continue
            
            # Проверяем стоп-слова (менее строго)
            if token in self.stopwords:
                continue

            # Разрешаем слова с цифрами (важные для новостей: "2024", "5g", etc.)
            # Полностью цифровые слова пропускаем, но слова с буквами и цифрами оставляем
            if token.isdigit() and not any(c.isalpha() for c in token):
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
