"""Настройка логирования для приложения."""

import logging
import sys
import json
import os
from pathlib import Path
from typing import Optional, Dict, Any
from datetime import datetime


_logger: Optional[logging.Logger] = None


class JSONFormatter(logging.Formatter):
    """JSON formatter для структурированного логирования."""

    def format(self, record: logging.LogRecord) -> str:
        """Форматирует запись лога в JSON."""
        # Базовые поля
        log_entry = {
            "timestamp": datetime.fromtimestamp(record.created).isoformat(),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
        }

        # Добавляем дополнительные поля из extra
        if hasattr(record, 'extra_fields'):
            log_entry.update(record.extra_fields)

        # Добавляем информацию об исключении
        if record.exc_info:
            log_entry["exception"] = self.formatException(record.exc_info)

        # Добавляем стандартные поля Python logging, если они есть
        standard_fields = ['filename', 'lineno', 'funcName', 'module']
        for field in standard_fields:
            value = getattr(record, field, None)
            if value:
                log_entry[field] = value

        return json.dumps(log_entry, ensure_ascii=False)


class StructuredLogger:
    """Структурированный логгер с поддержкой дополнительных полей."""

    def __init__(self, logger: logging.Logger):
        self.logger = logger

    def _log(self, level: int, message: str, extra: Optional[Dict[str, Any]] = None):
        """Внутренний метод логирования."""
        if extra:
            self.logger.log(level, message, extra={"extra_fields": extra})
        else:
            self.logger.log(level, message)

    def debug(self, message: str, **kwargs):
        """DEBUG уровень логирования."""
        self._log(logging.DEBUG, message, kwargs)

    def info(self, message: str, **kwargs):
        """INFO уровень логирования."""
        self._log(logging.INFO, message, kwargs)

    def warning(self, message: str, **kwargs):
        """WARNING уровень логирования."""
        self._log(logging.WARNING, message, kwargs)

    def error(self, message: str, **kwargs):
        """ERROR уровень логирования."""
        self._log(logging.ERROR, message, kwargs)

    def critical(self, message: str, **kwargs):
        """CRITICAL уровень логирования."""
        self._log(logging.CRITICAL, message, kwargs)

    def exception(self, message: str, **kwargs):
        """Логирование исключения с traceback."""
        self.logger.exception(message, extra={"extra_fields": kwargs} if kwargs else None)


def setup_logger(
    name: str = "news_analyzer",
    log_level: str = "INFO",
    log_dir: Optional[Path] = None,
    log_to_file: bool = True,
    json_format: bool = None
) -> logging.Logger:
    """
    Настраивает и возвращает логгер.

    Args:
        name: Имя логгера
        log_level: Уровень логирования (DEBUG, INFO, WARNING, ERROR, CRITICAL)
        log_dir: Директория для логов (если None, логи только в консоль)
        log_to_file: Сохранять ли логи в файл
        json_format: Использовать JSON формат (если None, определяется по LOG_FORMAT=JSON)

    Returns:
        Настроенный логгер
    """
    global _logger

    logger = logging.getLogger(name)
    logger.setLevel(getattr(logging, log_level.upper(), logging.INFO))

    # Очищаем существующие обработчики
    logger.handlers.clear()

    # Определяем формат логов
    if json_format is None:
        json_format = os.getenv("LOG_FORMAT", "").upper() == "JSON"

    if json_format:
        formatter = JSONFormatter()
    else:
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


def get_structured_logger(name: str = "news_analyzer") -> StructuredLogger:
    """
    Возвращает структурированный логгер.

    Args:
        name: Имя логгера

    Returns:
        StructuredLogger
    """
    logger = get_logger(name)
    return StructuredLogger(logger)
