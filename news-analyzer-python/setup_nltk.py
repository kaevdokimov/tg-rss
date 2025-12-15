#!/usr/bin/env python3
"""
Скрипт для загрузки необходимых данных NLTK.

Запустите этот скрипт перед первым использованием:
    python setup_nltk.py
"""

import nltk
import sys

def main():
    """Загружает необходимые данные NLTK."""
    print("Загрузка данных NLTK...")
    
    try:
        # Загружаем стоп-слова для русского языка
        print("Загрузка стоп-слов для русского языка...")
        nltk.download("stopwords", quiet=False)
        
        # Загружаем токенизатор
        print("Загрузка токенизатора punkt...")
        nltk.download("punkt", quiet=False)
        
        print("\n✅ Все необходимые данные NLTK загружены успешно!")
        
    except Exception as e:
        print(f"\n❌ Ошибка при загрузке данных NLTK: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
