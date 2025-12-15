"""Вспомогательные функции."""

from pathlib import Path
from datetime import datetime
from typing import Union


def ensure_dir(path: Union[str, Path]) -> Path:
    """
    Создает директорию, если она не существует.
    
    Args:
        path: Путь к директории
        
    Returns:
        Path: Путь к директории
    """
    dir_path = Path(path)
    dir_path.mkdir(parents=True, exist_ok=True)
    return dir_path


def format_datetime(dt: datetime, fmt: str = "%Y-%m-%d %H:%M:%S") -> str:
    """
    Форматирует datetime в строку.
    
    Args:
        dt: Объект datetime
        fmt: Формат строки
        
    Returns:
        Отформатированная строка
    """
    return dt.strftime(fmt)
