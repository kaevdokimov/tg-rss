#!/usr/bin/env python3
"""Скрипт для запуска тестов."""

import subprocess
import sys
import os
from pathlib import Path

def run_command(cmd, description):
    """Запуск команды с выводом результата."""
    print(f"\n{'='*60}")
    print(f"Запуск: {description}")
    print('='*60)

    try:
        result = subprocess.run(cmd, shell=True, check=True, capture_output=True, text=True)
        print("✓ УСПЕШНО")
        if result.stdout:
            print("Вывод:")
            print(result.stdout)
        return True
    except subprocess.CalledProcessError as e:
        print(f"✗ ОШИБКА (код: {e.returncode})")
        if e.stdout:
            print("Вывод:")
            print(e.stdout)
        if e.stderr:
            print("Ошибки:")
            print(e.stderr)
        return False

def main():
    """Основная функция запуска тестов."""
    print("🧪 Запуск тестовой инфраструктуры news-analyzer-python")

    # Переходим в директорию проекта
    project_dir = Path(__file__).parent
    os.chdir(project_dir)

    success_count = 0
    total_tests = 0

    # 1. Проверка синтаксиса
    total_tests += 1
    if run_command("python3 -m py_compile src/**/*.py", "Проверка синтаксиса Python файлов"):
        success_count += 1

    # 2. Проверка импортов
    total_tests += 1
    import_test = """
import sys
sys.path.insert(0, 'src')
try:
    from src.cache.redis_cache import RedisCache
    from src.monitoring.metrics import metrics_manager
    from src.preprocessor.text_cleaner import TextCleaner
    from src.analyzer.vectorizer import TextVectorizer
    print("✓ Все импорты успешны")
except ImportError as e:
    print(f"✗ Ошибка импорта: {e}")
    sys.exit(1)
"""
    if run_command(f'python3 -c "{import_test}"', "Проверка импортов модулей"):
        success_count += 1

    # 3. Запуск unit тестов
    total_tests += 1
    if run_command("python3 -m pytest tests/ -v --tb=short", "Запуск unit тестов"):
        success_count += 1

    # 4. Запуск тестов с покрытием (если установлен coverage)
    total_tests += 1
    coverage_cmd = "python3 -m pytest tests/ --cov=src --cov-report=term-missing --cov-report=html:htmlcov"
    if run_command(coverage_cmd, "Запуск тестов с покрытием кода"):
        success_count += 1
    else:
        print("⚠️  Coverage не установлен или не настроен, пропускаем")

    # 5. Проверка линтера (если установлен flake8)
    total_tests += 1
    if run_command("python3 -m flake8 src/ --max-line-length=120 --ignore=E501,W503", "Проверка кода линтером"):
        success_count += 1
    else:
        print("⚠️  flake8 не установлен, пропускаем проверку линтера")

    # Итоговый результат
    print(f"\n{'='*60}")
    print("📊 РЕЗУЛЬТАТЫ ТЕСТИРОВАНИЯ")
    print('='*60)
    print(f"✅ Успешно: {success_count}/{total_tests}")
    print(f"❌ Провалено: {total_tests - success_count}/{total_tests}")

    if success_count == total_tests:
        print("🎉 Все тесты пройдены!")
        return 0
    else:
        print("⚠️  Некоторые тесты провалены. Проверьте вывод выше.")
        return 1

if __name__ == "__main__":
    sys.exit(main())