"""Подключение к PostgreSQL и работа с данными."""

import psycopg2
from psycopg2.extras import RealDictCursor
from typing import List, Optional
from datetime import datetime, timedelta
from dataclasses import dataclass

from ..utils.logger import get_logger

logger = get_logger(__name__)


@dataclass
class NewsItem:
    """Структура новости из БД."""
    id: int
    title: str
    description: str
    link: str
    published_at: datetime
    full_text: Optional[str] = None
    source_id: Optional[int] = None


@dataclass
class User:
    """Структура пользователя из БД."""
    chat_id: int
    username: str


class Database:
    """Класс для работы с PostgreSQL."""
    
    def __init__(self, connection_string: str):
        """
        Инициализация подключения к БД.
        
        Args:
            connection_string: Строка подключения в формате psycopg2
        """
        self.connection_string = connection_string
        self._conn: Optional[psycopg2.extensions.connection] = None
    
    def connect(self) -> None:
        """Устанавливает подключение к БД."""
        try:
            self._conn = psycopg2.connect(self.connection_string)
            logger.info("Подключение к БД установлено")
        except psycopg2.Error as e:
            logger.error(f"Ошибка подключения к БД: {e}")
            raise
    
    def disconnect(self) -> None:
        """Закрывает подключение к БД."""
        if self._conn:
            self._conn.close()
            self._conn = None
            logger.info("Подключение к БД закрыто")
    
    def __enter__(self):
        """Контекстный менеджер: вход."""
        self.connect()
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Контекстный менеджер: выход."""
        self.disconnect()
    
    def get_news_last_hours(
        self,
        hours: int = 24,
        table_name: str = "news",
        use_titles_only: bool = True
    ) -> List[NewsItem]:
        """
        Получает новости за последние N часов.
        
        Args:
            hours: Количество часов для выборки
            table_name: Имя таблицы с новостями
            use_titles_only: Использовать только заголовки (True) или заголовки + описание (False)
            
        Returns:
            Список новостей
        """
        if not self._conn:
            raise RuntimeError("Подключение к БД не установлено. Вызовите connect() или используйте контекстный менеджер.")
        
        # Вычисляем временную границу
        time_threshold = datetime.now() - timedelta(hours=hours)
        
        # Формируем запрос
        if use_titles_only:
            # Используем только заголовки для анализа
            query = f"""
                SELECT 
                    id,
                    title,
                    description,
                    link,
                    published_at,
                    full_text,
                    source_id
                FROM {table_name}
                WHERE published_at >= %s
                  AND title IS NOT NULL
                  AND title != ''
                ORDER BY published_at DESC
            """
        else:
            # Используем заголовки и описание
            query = f"""
                SELECT 
                    id,
                    title,
                    description,
                    link,
                    published_at,
                    full_text,
                    source_id
                FROM {table_name}
                WHERE published_at >= %s
                  AND (title IS NOT NULL AND title != '')
                ORDER BY published_at DESC
            """
        
        try:
            with self._conn.cursor(cursor_factory=RealDictCursor) as cursor:
                cursor.execute(query, (time_threshold,))
                rows = cursor.fetchall()
                
                news_items = []
                for row in rows:
                    news_item = NewsItem(
                        id=row["id"],
                        title=row["title"],
                        description=row.get("description", "") or "",
                        link=row["link"],
                        published_at=row["published_at"],
                        full_text=row.get("full_text"),
                        source_id=row.get("source_id")
                    )
                    news_items.append(news_item)
                
                logger.info(f"Получено {len(news_items)} новостей за последние {hours} часов")
                return news_items
                
        except psycopg2.Error as e:
            logger.error(f"Ошибка при получении новостей: {e}")
            raise
    
    def get_all_users(self, table_name: str = "users") -> List[User]:
        """
        Получает всех пользователей из БД.
        
        Args:
            table_name: Имя таблицы с пользователями
            
        Returns:
            Список пользователей
        """
        if not self._conn:
            raise RuntimeError("Подключение к БД не установлено. Вызовите connect() или используйте контекстный менеджер.")
        
        query = f"""
            SELECT chat_id, username
            FROM {table_name}
            ORDER BY chat_id
        """
        
        try:
            with self._conn.cursor(cursor_factory=RealDictCursor) as cursor:
                cursor.execute(query)
                rows = cursor.fetchall()
                
                users = []
                for row in rows:
                    user = User(
                        chat_id=row["chat_id"],
                        username=row.get("username", "") or ""
                    )
                    users.append(user)
                
                logger.info(f"Получено {len(users)} пользователей из БД")
                return users
                
        except psycopg2.Error as e:
            logger.error(f"Ошибка при получении пользователей: {e}")
            raise
    
    def test_connection(self) -> bool:
        """
        Проверяет подключение к БД.
        
        Returns:
            True если подключение успешно
        """
        try:
            if not self._conn:
                self.connect()
            with self._conn.cursor() as cursor:
                cursor.execute("SELECT 1")
                cursor.fetchone()
            logger.info("Тест подключения к БД успешен")
            return True
        except psycopg2.Error as e:
            logger.error(f"Ошибка при тесте подключения: {e}")
            return False
