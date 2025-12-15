"""Получение новостей из базы данных."""

from typing import List
from ..db.database import Database, NewsItem
from ..config.settings import Settings
from ..utils.logger import get_logger

logger = get_logger(__name__)


class NewsFetcher:
    """Класс для получения новостей из БД."""
    
    def __init__(self, db: Database, settings: Settings):
        """
        Инициализация fetcher.
        
        Args:
            db: Объект подключения к БД
            settings: Настройки приложения
        """
        self.db = db
        self.settings = settings
    
    def fetch_recent_news(self) -> List[NewsItem]:
        """
        Получает новости за последние N часов согласно настройкам.
        
        Returns:
            Список новостей
        """
        logger.info(
            f"Начинаем получение новостей за последние {self.settings.time_window_hours} часов"
        )
        
        news_items = self.db.get_news_last_hours(
            hours=self.settings.time_window_hours,
            table_name=self.settings.db_table,
            use_titles_only=self.settings.use_titles_only
        )
        
        if not news_items:
            logger.warning("Новости не найдены за указанный период")
        else:
            logger.info(f"Успешно получено {len(news_items)} новостей")
        
        return news_items
