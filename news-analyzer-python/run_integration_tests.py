#!/usr/bin/env python3
"""Скрипт для запуска интеграционных тестов."""

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


def check_docker():
    """Проверка доступности Docker."""
    try:
        result = subprocess.run(["docker", "version"], capture_output=True, text=True)
        if result.returncode == 0:
            print("✓ Docker доступен")
            return True
        else:
            print("✗ Docker недоступен")
            return False
    except FileNotFoundError:
        print("✗ Docker не установлен")
        return False


def main():
    """Основная функция запуска интеграционных тестов."""
    print("🧪 Запуск интеграционных тестов news-analyzer-python")

    # Переходим в директорию проекта
    project_dir = Path(__file__).parent
    os.chdir(project_dir)

    success_count = 0
    total_tests = 0

    # 1. Проверка Docker
    total_tests += 1
    if check_docker():
        success_count += 1
    else:
        print("⚠️ Docker недоступен. Интеграционные тесты не будут запущены.")
        return 1

    # 2. Проверка зависимостей интеграционных тестов
    total_tests += 1
    import_test = """
import sys
sys.path.insert(0, 'src')
try:
    from testcontainers.postgres import PostgresContainer
    from testcontainers.redis import RedisContainer
    from src.db import Database
    from src.cache.redis_cache import RedisCache
    from src.monitoring.api import app
    print("✓ Integration test dependencies available")
except ImportError as e:
    print(f"✗ Missing dependency: {e}")
    sys.exit(1)
"""
    if run_command(f'python3 -c "{import_test}"', "Проверка зависимостей интеграционных тестов"):
        success_count += 1

    # 3. Запуск интеграционных тестов
    total_tests += 1
    if run_command("python3 -m pytest tests/integration/ -v --tb=short --maxfail=3",
                   "Запуск интеграционных тестов"):
        success_count += 1

    # 4. Запуск тестов с покрытием для интеграционных тестов
    total_tests += 1
    coverage_cmd = "python3 -m pytest tests/integration/ --cov=src --cov-report=term-missing"
    if run_command(coverage_cmd, "Интеграционные тесты с покрытием"):
        success_count += 1
    else:
        print("⚠️  Coverage для интеграционных тестов не удался, пропускаем")

    # Итоговый результат
    print(f"\n{'='*60}")
    print("📊 РЕЗУЛЬТАТЫ ИНТЕГРАЦИОННЫХ ТЕСТОВ")
    print('='*60)
    print(f"✅ Успешно: {success_count}/{total_tests}")
    print(f"❌ Провалено: {total_tests - success_count}/{total_tests}")

    if success_count == total_tests:
        print("🎉 Все интеграционные тесты пройдены!")
        print("\n💡 Интеграционные тесты проверяют:")
        print("  • Полный ML пайплайн (данные → векторы → кластеры → нарративы)")
        print("  • Работа с PostgreSQL и Redis")
        print("  • API endpoints")
        print("  • Производительность и надежность")
        return 0
    else:
        print("⚠️  Некоторые интеграционные тесты провалены.")
        print("💡 Возможные причины:")
        print("  • Docker не запущен")
        print("  • Недостаточно ресурсов для контейнеров")
        print("  • Проблемы с сетевыми портами")
        print("  • Зависимости не установлены")
        return 1

if __name__ == "__main__":
    sys.exit(main())