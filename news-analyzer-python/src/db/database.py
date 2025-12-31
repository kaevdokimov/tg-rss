"""Подключение к PostgreSQL и работа с данными."""

import psycopg2
from psycopg2.extras import RealDictCursor, Json
from psycopg2 import sql
from typing import List, Optional, Dict, Any
from datetime import datetime, timedelta
from dataclasses import dataclass
import json

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


@dataclass
class AnalysisResult:
    """Структура результата анализа из БД."""
    id: int
    analysis_date: datetime
    total_news: int
    narratives_count: int
    narratives: dict  # JSON данные
    created_at: datetime


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
    
    def get_news_count_last_hours(
        self,
        hours: int = 24,
        table_name: str = "news"
    ) -> int:
        """
        Получает количество новостей за последние N часов.

        Args:
            hours: Количество часов для выборки
            table_name: Имя таблицы с новостями

        Returns:
            Количество новостей
        """
        if not self._conn:
            raise RuntimeError("Подключение к БД не установлено. Вызовите connect() или используйте контекстный менеджер.")

        # Вычисляем временную границу
        time_threshold = datetime.now() - timedelta(hours=hours)

        # Формируем запрос с безопасной подстановкой имени таблицы
        query = sql.SQL("""
            SELECT COUNT(*)
            FROM {}
            WHERE published_at >= %s
              AND title IS NOT NULL
              AND title != ''
        """).format(sql.Identifier(table_name))

        try:
            with self._conn.cursor() as cursor:
                cursor.execute(query, (time_threshold,))
                count = cursor.fetchone()[0]
                return count

        except psycopg2.Error as e:
            logger.error(f"Ошибка при получении количества новостей: {e}")
            raise

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
        
        # Формируем запрос с безопасной подстановкой имени таблицы
        if use_titles_only:
            # Используем только заголовки для анализа
            query = sql.SQL("""
                SELECT 
                    id,
                    title,
                    description,
                    link,
                    published_at,
                    full_text,
                    source_id
                FROM {}
                WHERE published_at >= %s
                  AND title IS NOT NULL
                  AND title != ''
                ORDER BY published_at DESC
            """).format(sql.Identifier(table_name))
        else:
            # Используем заголовки и описание
            query = sql.SQL("""
                SELECT 
                    id,
                    title,
                    description,
                    link,
                    published_at,
                    full_text,
                    source_id
                FROM {}
                WHERE published_at >= %s
                  AND (title IS NOT NULL AND title != '')
                ORDER BY published_at DESC
            """).format(sql.Identifier(table_name))
        
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
        
        # Безопасная подстановка имени таблицы
        query = sql.SQL("""
            SELECT chat_id, username
            FROM {}
            ORDER BY chat_id
        """).format(sql.Identifier(table_name))
        
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
    
    def save_analysis_result(
        self,
        analysis_date: datetime,
        total_news: int,
        narratives: List[Dict[str, Any]],
        table_name: str = "news_analysis"
    ) -> int:
        """
        Сохраняет результат анализа в БД.
        
        Args:
            analysis_date: Дата и время анализа
            total_news: Общее количество проанализированных новостей
            narratives: Список нарративов (тем)
            table_name: Имя таблицы для сохранения
            
        Returns:
            ID сохраненной записи
        """
        if not self._conn:
            raise RuntimeError("Подключение к БД не установлено. Вызовите connect() или используйте контекстный менеджер.")
        
        narratives_count = len(narratives)
        
        # Безопасная подстановка имени таблицы
        query = sql.SQL("""
            INSERT INTO {} (analysis_date, total_news, narratives_count, narratives)
            VALUES (%s, %s, %s, %s)
            ON CONFLICT (analysis_date) DO UPDATE
            SET total_news = EXCLUDED.total_news,
                narratives_count = EXCLUDED.narratives_count,
                narratives = EXCLUDED.narratives,
                created_at = CURRENT_TIMESTAMP
            RETURNING id
        """).format(sql.Identifier(table_name))
        
        try:
            with self._conn.cursor() as cursor:
                # Преобразуем narratives в JSON для PostgreSQL
                narratives_json = Json(narratives)
                cursor.execute(
                    query,
                    (analysis_date, total_news, narratives_count, narratives_json)
                )
                result = cursor.fetchone()
                self._conn.commit()
                
                analysis_id = result[0]
                logger.info(
                    f"Результат анализа сохранен в БД: ID={analysis_id}, "
                    f"дата={analysis_date}, новостей={total_news}, тем={narratives_count}"
                )
                return analysis_id
                
        except psycopg2.Error as e:
            self._conn.rollback()
            logger.error(f"Ошибка при сохранении результата анализа: {e}")
            raise
    
    def get_analysis_results(
        self,
        start_date: Optional[datetime] = None,
        end_date: Optional[datetime] = None,
        limit: Optional[int] = None,
        table_name: str = "news_analysis"
    ) -> List[AnalysisResult]:
        """
        Получает исторические результаты анализа из БД.
        
        Args:
            start_date: Начальная дата для фильтрации (опционально)
            end_date: Конечная дата для фильтрации (опционально)
            limit: Максимальное количество записей (опционально)
            table_name: Имя таблицы
            
        Returns:
            Список результатов анализа
        """
        if not self._conn:
            raise RuntimeError("Подключение к БД не установлено. Вызовите connect() или используйте контекстный менеджер.")
        
        # Формируем запрос с фильтрами
        conditions = []
        params = []
        
        if start_date:
            conditions.append("analysis_date >= %s")
            params.append(start_date)
        
        if end_date:
            conditions.append("analysis_date <= %s")
            params.append(end_date)
        
        # Безопасная подстановка имени таблицы
        base_query = sql.SQL("""
            SELECT id, analysis_date, total_news, narratives_count, narratives, created_at
            FROM {}
        """).format(sql.Identifier(table_name))
        
        if conditions:
            where_clause = sql.SQL(" WHERE {}").format(sql.SQL(" AND ").join([sql.SQL(c) for c in conditions]))
        else:
            where_clause = sql.SQL("")
        
        order_clause = sql.SQL(" ORDER BY analysis_date DESC")
        
        if limit:
            limit_clause = sql.SQL(" LIMIT %s")
            params.append(limit)
            query = base_query + where_clause + order_clause + limit_clause
        else:
            query = base_query + where_clause + order_clause
        
        try:
            with self._conn.cursor(cursor_factory=RealDictCursor) as cursor:
                cursor.execute(query, tuple(params) if params else None)
                
                rows = cursor.fetchall()
                
                results = []
                for row in rows:
                    # Преобразуем JSONB в dict
                    narratives_data = row["narratives"]
                    if isinstance(narratives_data, str):
                        narratives_data = json.loads(narratives_data)
                    
                    result = AnalysisResult(
                        id=row["id"],
                        analysis_date=row["analysis_date"],
                        total_news=row["total_news"],
                        narratives_count=row["narratives_count"],
                        narratives=narratives_data,
                        created_at=row["created_at"]
                    )
                    results.append(result)
                
                logger.info(f"Получено {len(results)} результатов анализа из БД")
                return results
                
        except psycopg2.Error as e:
            logger.error(f"Ошибка при получении результатов анализа: {e}")
            raise
    
    def get_latest_analysis_result(
        self,
        table_name: str = "news_analysis"
    ) -> Optional[AnalysisResult]:
        """
        Получает последний результат анализа.
        
        Args:
            table_name: Имя таблицы
            
        Returns:
            Последний результат анализа или None
        """
        results = self.get_analysis_results(limit=1, table_name=table_name)
        return results[0] if results else None
    
    def ensure_analysis_table_exists(self, table_name: str = "news_analysis") -> bool:
        """
        Проверяет существование таблицы для результатов анализа и создает её, если не существует.
        
        Args:
            table_name: Имя таблицы
            
        Returns:
            True если таблица существует или была создана
        """
        if not self._conn:
            raise RuntimeError("Подключение к БД не установлено. Вызовите connect() или используйте контекстный менеджер.")
        
        try:
            # Проверяем существование таблицы
            check_query = sql.SQL("""
                SELECT EXISTS (
                    SELECT FROM information_schema.tables 
                    WHERE table_schema = 'public' 
                    AND table_name = %s
                )
            """).format()
            
            with self._conn.cursor() as cursor:
                cursor.execute(check_query, (table_name,))
                exists = cursor.fetchone()[0]
                
                if exists:
                    logger.info(f"Таблица {table_name} уже существует")
                    return True
                
                # Создаем таблицу
                create_table_query = sql.SQL("""
                    CREATE TABLE IF NOT EXISTS {} (
                        id SERIAL PRIMARY KEY,
                        analysis_date TIMESTAMP NOT NULL,
                        total_news INTEGER NOT NULL,
                        narratives_count INTEGER NOT NULL,
                        narratives JSONB NOT NULL,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        CONSTRAINT unique_analysis_date UNIQUE (analysis_date)
                    )
                """).format(sql.Identifier(table_name))
                
                cursor.execute(create_table_query)
                
                # Создаем индексы
                index1_name = f"idx_{table_name}_date"
                index2_name = f"idx_{table_name}_created_at"
                
                create_index1_query = sql.SQL("""
                    CREATE INDEX IF NOT EXISTS {} ON {} (analysis_date DESC)
                """).format(
                    sql.Identifier(index1_name),
                    sql.Identifier(table_name)
                )
                
                create_index2_query = sql.SQL("""
                    CREATE INDEX IF NOT EXISTS {} ON {} (created_at DESC)
                """).format(
                    sql.Identifier(index2_name),
                    sql.Identifier(table_name)
                )
                
                cursor.execute(create_index1_query)
                cursor.execute(create_index2_query)
                
                cursor.execute(create_query)
                self._conn.commit()
                logger.info(f"Таблица {table_name} успешно создана")
                return True
                
        except psycopg2.Error as e:
            self._conn.rollback()
            logger.error(f"Ошибка при создании таблицы {table_name}: {e}")
            return False
