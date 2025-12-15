"""Форматирование отчетов в JSON."""

import json
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Any

from ..utils.logger import get_logger
from ..utils.helpers import ensure_dir

logger = get_logger(__name__)


class ReportFormatter:
    """Класс для форматирования отчетов."""
    
    def __init__(self, reports_dir: Path, date_format: str = "%Y-%m-%d"):
        """
        Инициализация formatter.
        
        Args:
            reports_dir: Директория для сохранения отчетов
            date_format: Формат даты в имени файла
        """
        self.reports_dir = ensure_dir(reports_dir)
        self.date_format = date_format
    
    def save_report(
        self,
        narratives: List[Dict[str, Any]],
        total_news: int,
        analysis_date: datetime
    ) -> Path:
        """
        Сохраняет отчет в JSON файл.
        
        Args:
            narratives: Список нарративов
            total_news: Общее количество новостей
            analysis_date: Дата анализа
            
        Returns:
            Путь к сохраненному файлу
        """
        report = {
            "analysis_date": analysis_date.isoformat(),
            "total_news": total_news,
            "narratives_count": len(narratives),
            "narratives": narratives
        }
        
        # Формируем имя файла
        filename = f"report_{analysis_date.strftime(self.date_format)}.json"
        filepath = self.reports_dir / filename
        
        # Сохраняем
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(report, f, ensure_ascii=False, indent=2)
        
        logger.info(f"Отчет сохранен: {filepath}")
        return filepath
