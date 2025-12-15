#!/usr/bin/env python3
"""
Скрипт для проверки совместимости зависимостей с Python 3.14.

Запуск:
    python test_dependencies.py
"""

import sys

def test_imports():
    """Проверяет импорт всех зависимостей."""
    errors = []
    
    dependencies = [
        ("pandas", "pandas"),
        ("numpy", "numpy"),
        ("scikit-learn", "sklearn"),
        ("nltk", "nltk"),
        ("hdbscan", "hdbscan"),
        ("psycopg2", "psycopg2"),
        ("dotenv", "dotenv"),
        ("yaml", "yaml"),
        ("tqdm", "tqdm"),
        ("requests", "requests"),
    ]
    
    print("Проверка совместимости зависимостей с Python 3.14...")
    print(f"Python версия: {sys.version}")
    print("=" * 60)
    
    for package_name, import_name in dependencies:
        try:
            __import__(import_name)
            print(f"✅ {package_name:20s} - OK")
        except ImportError as e:
            print(f"❌ {package_name:20s} - ОШИБКА: {e}")
            errors.append((package_name, str(e)))
        except Exception as e:
            print(f"⚠️  {package_name:20s} - ПРЕДУПРЕЖДЕНИЕ: {e}")
    
    print("=" * 60)
    
    if errors:
        print(f"\n❌ Найдено {len(errors)} ошибок:")
        for package, error in errors:
            print(f"  - {package}: {error}")
        return False
    else:
        print("\n✅ Все зависимости успешно импортированы!")
        return True

if __name__ == "__main__":
    success = test_imports()
    sys.exit(0 if success else 1)
