"""Загрузка и управление конфигурацией из .env и config.yaml."""

import os
import sys
from pathlib import Path
from typing import Dict, Any, List, Tuple
import yaml
from dotenv import load_dotenv

from .validator import load_and_validate_config, ConfigValidationError
from ..utils.logger import get_logger

logger = get_logger(__name__)


class Settings:
    """Класс для хранения настроек приложения."""
    
    def __init__(self, config_dict: Dict[str, Any]):
        """Инициализация настроек из словаря конфигурации."""
        # БД
        db_config = config_dict.get("db", {})
        self.db_host = self._resolve_env(db_config.get("host", "db"))
        self.db_port = int(self._resolve_env(db_config.get("port", "5432")))
        self.db_user = self._resolve_env(db_config.get("user", "postgres"))
        self.db_password = self._resolve_env(db_config.get("password", ""))
        self.db_name = self._resolve_env(db_config.get("database", "news_bot"))
        self.db_table = db_config.get("table_name", "news")
        
        # Анализ
        analysis_config = config_dict.get("analysis", {})
        self.time_window_hours = analysis_config.get("time_window_hours", 24)
        self.min_cluster_size = analysis_config.get("min_cluster_size", 5)
        self.top_narratives = analysis_config.get("top_narratives", 5)
        self.use_titles_only = analysis_config.get("use_titles_only", True)
        
        # Предобработка
        preproc_config = config_dict.get("preprocessing", {})
        self.stopwords_extra = preproc_config.get("stopwords_extra", [])
        self.min_word_length = preproc_config.get("min_word_length", 3)
        self.max_word_length = preproc_config.get("max_word_length", 20)
        
        # Векторизация
        vec_config = config_dict.get("vectorization", {})
        self.max_features = vec_config.get("max_features", 5000)
        self.min_df = vec_config.get("min_df", 2)
        self.max_df = vec_config.get("max_df", 0.95)
        
        # Кластеризация
        cluster_config = config_dict.get("clustering", {})
        self.cluster_min_size = cluster_config.get("min_cluster_size", 5)
        self.cluster_min_samples = cluster_config.get("min_samples", 3)
        self.cluster_metric = cluster_config.get("metric", "cosine")
        
        # Вывод
        output_config = config_dict.get("output", {})
        self.reports_dir = Path(output_config.get("reports_dir", "./storage/reports"))
        self.logs_dir = Path(output_config.get("logs_dir", "./storage/logs"))
        self.date_format = output_config.get("date_format", "%Y-%m-%d")
        
        # Логирование
        self.log_level = os.getenv("LOG_LEVEL", "INFO")
    
    def _resolve_env(self, value: str) -> str:
        """Разрешает переменные окружения в значениях конфигурации."""
        if isinstance(value, str) and value.startswith("${") and value.endswith("}"):
            env_var = value[2:-1]
            return os.getenv(env_var, "")
        return value
    
    def get_db_connection_string(self) -> str:
        """Возвращает строку подключения к PostgreSQL."""
        return (
            f"host={self.db_host} "
            f"port={self.db_port} "
            f"user={self.db_user} "
            f"password={self.db_password} "
            f"dbname={self.db_name}"
        )


def load_settings(config_path: str = "config.yaml", validate: bool = True) -> Settings:
    """
    Загружает и валидирует конфигурацию из .env и config.yaml.

    Args:
        config_path: Путь к файлу config.yaml
        validate: Выполнять ли валидацию конфигурации

    Returns:
        Settings: Объект с настройками

    Raises:
        ConfigValidationError: Если валидация не пройдена
    """
    # Загружаем переменные окружения из .env
    env_path = Path(".env")
    if env_path.exists():
        load_dotenv(env_path)

    # Загружаем и валидируем конфигурацию
    config_file = Path(config_path)
    config_dict, is_valid, errors, warnings = load_and_validate_config(config_file)

    if not config_file.exists():
        raise FileNotFoundError(
            f"Файл конфигурации {config_path} не найден. "
            f"Скопируйте config.yaml.example в config.yaml и настройте его."
        )

    # Выводим отчет о валидации
    if validate:
        if warnings:
            logger.warning("Предупреждения конфигурации:")
            for warning in warnings:
                logger.warning(f"  - {warning}")

        if not is_valid:
            error_msg = f"Конфигурация не валидна:\n" + "\n".join(f"  - {err}" for err in errors)
            logger.error(error_msg)
            raise ConfigValidationError(error_msg)

        if is_valid and not warnings:
            logger.info("✅ Конфигурация валидна")

    # Заменяем переменные окружения в значениях
    config_dict = _substitute_env_vars(config_dict)

    return Settings(config_dict)


def validate_config_file(config_path: str = "config.yaml") -> Tuple[bool, List[str], List[str]]:
    """
    Валидирует файл конфигурации без создания объекта Settings.

    Args:
        config_path: Путь к файлу config.yaml

    Returns:
        (is_valid, errors, warnings)
    """
    config_file = Path(config_path)
    _, is_valid, errors, warnings = load_and_validate_config(config_file)
    return is_valid, errors, warnings


def print_config_validation_report(config_path: str = "config.yaml"):
    """Выводит отчет о валидации конфигурации."""
    from .validator import ConfigValidator

    is_valid, errors, warnings = validate_config_file(config_path)

    print("=" * 60)
    print("Проверка конфигурации news-analyzer-python")
    print("=" * 60)

    if errors:
        print(f"\n❌ Ошибки ({len(errors)}):")
        for error in errors:
            print(f"  - {error}")

    if warnings:
        print(f"\n⚠️  Предупреждения ({len(warnings)}):")
        for warning in warnings:
            print(f"  - {warning}")

    if is_valid and not warnings:
        print("\n✅ Конфигурация корректна, без предупреждений")
        return True

    if is_valid and warnings:
        print(f"\n⚠️  Конфигурация валидна, но есть {len(warnings)} предупреждений")
        return True

    print(f"\n❌ Конфигурация не валидна ({len(errors)} ошибок)")
    return False


def _substitute_env_vars(obj: Any) -> Any:
    """Рекурсивно заменяет переменные окружения в конфигурации."""
    if isinstance(obj, dict):
        return {k: _substitute_env_vars(v) for k, v in obj.items()}
    elif isinstance(obj, list):
        return [_substitute_env_vars(item) for item in obj]
    elif isinstance(obj, str) and obj.startswith("${") and obj.endswith("}"):
        env_var = obj[2:-1]
        return os.getenv(env_var, "")
    return obj
