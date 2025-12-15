"""Загрузка и управление конфигурацией из .env и config.yaml."""

import os
from pathlib import Path
from typing import Dict, Any, List
import yaml
from dotenv import load_dotenv


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


def load_settings(config_path: str = "config.yaml") -> Settings:
    """
    Загружает конфигурацию из .env и config.yaml.
    
    Args:
        config_path: Путь к файлу config.yaml
        
    Returns:
        Settings: Объект с настройками
    """
    # Загружаем переменные окружения из .env
    env_path = Path(".env")
    if env_path.exists():
        load_dotenv(env_path)
    
    # Загружаем config.yaml
    config_file = Path(config_path)
    if not config_file.exists():
        raise FileNotFoundError(
            f"Файл конфигурации {config_path} не найден. "
            f"Скопируйте config.yaml.example в config.yaml и настройте его."
        )
    
    with open(config_file, "r", encoding="utf-8") as f:
        config_dict = yaml.safe_load(f) or {}
    
    # Заменяем переменные окружения в значениях
    config_dict = _substitute_env_vars(config_dict)
    
    return Settings(config_dict)


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
