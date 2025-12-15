"""Генерация текстового резюме отчета."""

from datetime import datetime
from typing import List, Dict, Any

from ..utils.logger import get_logger

logger = get_logger(__name__)


class SummaryGenerator:
    """Класс для генерации текстового резюме."""
    
    def generate(
        self,
        narratives: List[Dict[str, Any]],
        total_news: int,
        analysis_date: datetime
    ) -> str:
        """
        Генерирует текстовое резюме отчета.
        
        Args:
            narratives: Список нарративов
            total_news: Общее количество новостей
            analysis_date: Дата анализа
            
        Returns:
            Текстовое резюме
        """
        lines = []
        lines.append("=" * 60)
        lines.append(f"КАРТА ДНЯ - {analysis_date.strftime('%d.%m.%Y')}")
        lines.append("=" * 60)
        lines.append("")
        lines.append(f"Всего новостей проанализировано: {total_news}")
        lines.append(f"Выявлено основных тем: {len(narratives)}")
        lines.append("")
        
        for idx, narrative in enumerate(narratives, 1):
            lines.append(f"ТЕМА #{idx} (новостей: {narrative['size']})")
            lines.append("-" * 60)
            lines.append(f"Ключевые слова: {', '.join(narrative['keywords'][:5])}")
            lines.append("")
            lines.append("Примеры заголовков:")
            for title in narrative['titles'][:3]:
                lines.append(f"  • {title}")
            lines.append("")
        
        lines.append("=" * 60)
        
        return "\n".join(lines)
