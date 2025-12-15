#!/usr/bin/env python3
"""
Скрипт для загрузки необходимых данных NLTK.

Запустите этот скрипт перед первым использованием:
    python setup_nltk.py
"""

import nltk
import sys
import os

def main():
    """Загружает необходимые данные NLTK."""
    print("Загрузка данных NLTK...")
    
    # Устанавливаем директорию для данных NLTK
    nltk_data_dir = os.getenv("NLTK_DATA", "/app/nltk_data")
    if not os.path.exists(nltk_data_dir):
        try:
            os.makedirs(nltk_data_dir, exist_ok=True)
        except Exception as e:
            print(f"⚠️  Не удалось создать директорию {nltk_data_dir}: {e}", file=sys.stderr)
            # Пробуем использовать директорию по умолчанию
            nltk_data_dir = None
    
    try:
        # Загружаем стоп-слова для русского языка
        print("Загрузка стоп-слов для русского языка...")
        if nltk_data_dir:
            nltk.download("stopwords", quiet=False, download_dir=nltk_data_dir)
        else:
            nltk.download("stopwords", quiet=False)
        
        # Загружаем токенизатор
        print("Загрузка токенизатора punkt...")
        if nltk_data_dir:
            nltk.download("punkt", quiet=False, download_dir=nltk_data_dir)
        else:
            nltk.download("punkt", quiet=False)
        
        print(f"\n✅ Все необходимые данные NLTK загружены успешно!")
        if nltk_data_dir:
            print(f"📁 Данные сохранены в: {nltk_data_dir}")
        
    except Exception as e:
        print(f"\n❌ Ошибка при загрузке данных NLTK: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
