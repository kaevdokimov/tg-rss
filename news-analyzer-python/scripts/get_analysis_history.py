#!/usr/bin/env python3
"""
Скрипт для получения исторических данных анализа.

Использование:
    python scripts/get_analysis_history.py                    # Последние 10 результатов
    python scripts/get_analysis_history.py --limit 20         # Последние 20 результатов
    python scripts/get_analysis_history.py --start 2025-12-01  # С определенной даты
    python scripts/get_analysis_history.py --start 2025-12-01 --end 2025-12-15  # За период
"""

import sys
import argparse
from pathlib import Path
from datetime import datetime

# Добавляем src в путь для импортов
sys.path.insert(0, str(Path(__file__).parent.parent / "src"))

from src.config import load_settings
from src.db import Database


def main():
    parser = argparse.ArgumentParser(description="Получение исторических данных анализа")
    parser.add_argument(
        "--start",
        type=str,
        help="Начальная дата (формат: YYYY-MM-DD или YYYY-MM-DD HH:MM:SS)"
    )
    parser.add_argument(
        "--end",
        type=str,
        help="Конечная дата (формат: YYYY-MM-DD или YYYY-MM-DD HH:MM:SS)"
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=10,
        help="Максимальное количество записей (по умолчанию: 10)"
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Вывести результаты в формате JSON"
    )
    
    args = parser.parse_args()
    
    # Парсим даты
    start_date = None
    end_date = None
    
    if args.start:
        try:
            if len(args.start) == 10:  # YYYY-MM-DD
                start_date = datetime.strptime(args.start, "%Y-%m-%d")
            else:  # YYYY-MM-DD HH:MM:SS
                start_date = datetime.strptime(args.start, "%Y-%m-%d %H:%M:%S")
        except ValueError as e:
            print(f"Ошибка парсинга начальной даты: {e}", file=sys.stderr)
            sys.exit(1)
    
    if args.end:
        try:
            if len(args.end) == 10:  # YYYY-MM-DD
                end_date = datetime.strptime(args.end, "%Y-%m-%d")
            else:  # YYYY-MM-DD HH:MM:SS
                end_date = datetime.strptime(args.end, "%Y-%m-%d %H:%M:%S")
        except ValueError as e:
            print(f"Ошибка парсинга конечной даты: {e}", file=sys.stderr)
            sys.exit(1)
    
    # Загружаем конфигурацию
    try:
        settings = load_settings()
    except Exception as e:
        print(f"Ошибка загрузки конфигурации: {e}", file=sys.stderr)
        sys.exit(1)
    
    # Подключаемся к БД
    db = Database(settings.get_db_connection_string())
    try:
        db.connect()
        
        # Получаем результаты
        results = db.get_analysis_results(
            start_date=start_date,
            end_date=end_date,
            limit=args.limit
        )
        
        if not results:
            print("Результаты анализа не найдены")
            return
        
        if args.json:
            # Выводим в формате JSON
            import json
            output = []
            for result in results:
                output.append({
                    "id": result.id,
                    "analysis_date": result.analysis_date.isoformat(),
                    "total_news": result.total_news,
                    "narratives_count": result.narratives_count,
                    "narratives": result.narratives,
                    "created_at": result.created_at.isoformat()
                })
            print(json.dumps(output, ensure_ascii=False, indent=2))
        else:
            # Выводим в читаемом формате
            print(f"Найдено результатов: {len(results)}\n")
            print("=" * 80)
            
            for result in results:
                print(f"ID: {result.id}")
                print(f"Дата анализа: {result.analysis_date.strftime('%Y-%m-%d %H:%M:%S')}")
                print(f"Новостей проанализировано: {result.total_news}")
                print(f"Найдено тем: {result.narratives_count}")
                print(f"Создано: {result.created_at.strftime('%Y-%m-%d %H:%M:%S')}")
                print("\nТоп-темы:")
                
                for idx, narrative in enumerate(result.narratives[:5], 1):
                    print(f"  {idx}. Тема #{narrative.get('cluster_id', '?')} "
                          f"({narrative.get('size', 0)} новостей)")
                    keywords = narrative.get('keywords', [])[:5]
                    if keywords:
                        print(f"     Ключевые слова: {', '.join(keywords)}")
                
                print("=" * 80)
                print()
        
    except Exception as e:
        print(f"Ошибка: {e}", file=sys.stderr)
        sys.exit(1)
    finally:
        db.disconnect()


if __name__ == "__main__":
    main()
