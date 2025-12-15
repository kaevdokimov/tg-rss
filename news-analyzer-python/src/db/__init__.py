"""Модуль для работы с базой данных PostgreSQL."""

from .database import Database, NewsItem, User

__all__ = ["Database", "NewsItem", "User"]
