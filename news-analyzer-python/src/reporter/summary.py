"""Генерация текстового резюме отчета."""

from datetime import datetime
from typing import List, Dict, Any, Optional

from ..utils.logger import get_logger

logger = get_logger(__name__)


class SummaryGenerator:
    """Класс для генерации текстового резюме."""
    
    def generate(
        self,
        narratives: List[Dict[str, Any]],
        total_news: int,
        analysis_date: datetime,
        clustering_metrics: Optional[Dict[str, Any]] = None
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
        lines.append(f"Всего новостей: {total_news}")
        lines.append(f"Выявлено тем: {len(narratives)}")

        # Проверяем, есть ли темы
        if len(narratives) == 0:
            lines.append("")
            lines.append("⚠️ Темы не найдены")
            lines.append("──────────────────────────────")
            lines.append("Автоматический анализ новостей")
            lines.append("")
            lines.append("=" * 60)
            return "\n".join(lines)

        # Добавляем метрики качества кластеризации
        if clustering_metrics:
            lines.append(f"📊 Метрики кластеризации:")
            lines.append(f"   • Кластеров: {clustering_metrics.get('total_clusters', 0)}")
            lines.append(f"   • Шумовых точек: {clustering_metrics.get('noise_points', 0)} ({clustering_metrics.get('noise_percentage', 0):.1f}%)")
            if clustering_metrics.get('total_clusters', 0) > 0:
                lines.append(f"   • Средний размер кластера: {clustering_metrics.get('avg_cluster_size', 0):.1f}")
                lines.append(f"   • Максимальный кластер: {clustering_metrics.get('max_cluster_size', 0)} новостей")
                lines.append(f"   • Минимальный кластер: {clustering_metrics.get('min_cluster_size', 0)} новостей")
            lines.append("")

        lines.append("")

        for idx, narrative in enumerate(narratives, 1):
            lines.append(f"ТЕМА #{idx} (новостей: {narrative['size']})")
            lines.append("-" * 60)
            lines.append(f"Ключевые слова: {', '.join(narrative['keywords'][:5])}")
            lines.append("")
            lines.append("Примеры новостей:")
            for news_item in narrative.get('news_examples', [])[:3]:
                lines.append(f"  Заголовок: {news_item['title']}")
                lines.append(f"  Источник: {news_item['source_name']}")
                lines.append(f"  Ссылка: {news_item['link']}")
                lines.append("")
        
        lines.append("=" * 60)
        
        return "\n".join(lines)
