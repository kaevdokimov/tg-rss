"""Настройка логирования для приложения."""

import logging
import sys
from pathlib import Path
from typing import Optional


_logger: Optional[logging.Logger] = None


def setup_logger(
    name: str = "news_analyzer",
    log_level: str = "INFO",
    log_dir: Optional[Path] = None,
    log_to_file: bool = True
) -> logging.Logger:
    """
    Настраивает и возвращает логгер.
    
    Args:
        name: Имя логгера
        log_level: Уровень логирования (DEBUG, INFO, WARNING, ERROR, CRITICAL)
        log_dir: Директория для логов (если None, логи только в консоль)
        log_to_file: Сохранять ли логи в файл
        
    Returns:
        Настроенный логгер
    """
    global _logger
    
    logger = logging.getLogger(name)
    logger.setLevel(getattr(logging, log_level.upper(), logging.INFO))
    
    # Очищаем существующие обработчики
    logger.handlers.clear()
    
    # Формат логов
    formatter = logging.Formatter(
        "%(asctime)s - %(name)s - %(levelname)s - %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )
    
    # Обработчик для консоли
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(logging.DEBUG)
    console_handler.setFormatter(formatter)
    logger.addHandler(console_handler)
    
    # Обработчик для файла
    if log_to_file and log_dir:
        log_dir.mkdir(parents=True, exist_ok=True)
        log_file = log_dir / f"{name}.log"
        file_handler = logging.FileHandler(log_file, encoding="utf-8")
        file_handler.setLevel(logging.DEBUG)
        file_handler.setFormatter(formatter)
        logger.addHandler(file_handler)
    
    _logger = logger
    return logger


def get_logger(name: str = "news_analyzer") -> logging.Logger:
    """
    Возвращает существующий логгер или создает новый.
    
    Args:
        name: Имя логгера
        
    Returns:
        Логгер
    """
    global _logger
    if _logger is None:
        return setup_logger(name)
    return logging.getLogger(name)
