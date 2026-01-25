"""Валидация конфигурации с поддержкой environment-specific настроек."""

import os
import re
from typing import Dict, List, Any, Optional, Tuple
from pathlib import Path
import yaml
import json

from ..utils.logger import get_logger

logger = get_logger(__name__)


class ConfigValidationError(Exception):
    """Ошибка валидации конфигурации."""
    pass


class EnvironmentConfig:
    """Управление environment-specific конфигурациями."""

    def __init__(self, base_config_path: Path):
        self.base_config_path = base_config_path
        self.environments = {}

    def load_environment_configs(self) -> Dict[str, Dict[str, Any]]:
        """Загружает конфигурации для всех доступных окружений."""
        # Определяем путь к environments относительно этого файла
        config_module_dir = Path(__file__).parent.resolve()
        environments_dir = config_module_dir / "environments"

        if not environments_dir.exists():
            logger.info("Директория environments не найдена, создаем базовую структуру")
            self._create_default_environments(environments_dir)

        # Загружаем конфигурации для каждого окружения
        environments = {}
        for env_file in environments_dir.glob("*.yaml"):
            env_name = env_file.stem
            try:
                with open(env_file, 'r', encoding='utf-8') as f:
                    env_config = yaml.safe_load(f)
                    environments[env_name] = env_config
                    logger.debug(f"Загружена конфигурация для окружения: {env_name}")
            except Exception as e:
                logger.error(f"Ошибка загрузки конфигурации {env_file}: {e}")

        self.environments = environments
        return environments

    def _create_default_environments(self, env_dir: Path):
        """Создает дефолтные конфигурации для разных окружений."""
        env_dir.mkdir(parents=True, exist_ok=True)

        # Development
        dev_config = {
            "environment": "development",
            "debug": True,
            "log_level": "DEBUG",
            "performance": {
                "max_features": 1000,
                "use_lemmatization": False,
                "async_processing": False
            },
            "limits": {
                "max_news_limit": 500,
                "min_news_threshold": 5
            }
        }

        # Production
        prod_config = {
            "environment": "production",
            "debug": False,
            "log_level": "INFO",
            "performance": {
                "max_features": 5000,
                "use_lemmatization": True,
                "async_processing": True
            },
            "limits": {
                "max_news_limit": 1200,
                "min_news_threshold": 10
            }
        }

        # Testing
        test_config = {
            "environment": "testing",
            "debug": True,
            "log_level": "DEBUG",
            "performance": {
                "max_features": 500,
                "use_lemmatization": False,
                "async_processing": False
            },
            "limits": {
                "max_news_limit": 100,
                "min_news_threshold": 2
            }
        }

        # CI/CD
        ci_config = {
            "environment": "ci",
            "debug": False,
            "log_level": "WARNING",
            "performance": {
                "max_features": 1000,
                "use_lemmatization": False,
                "async_processing": True
            },
            "limits": {
                "max_news_limit": 200,
                "min_news_threshold": 5
            }
        }

        configs = {
            "development.yaml": dev_config,
            "production.yaml": prod_config,
            "testing.yaml": test_config,
            "ci.yaml": ci_config
        }

        for filename, config in configs.items():
            config_file = env_dir / filename
            try:
                with open(config_file, 'w', encoding='utf-8') as f:
                    yaml.dump(config, f, default_flow_style=False, allow_unicode=True)
                logger.info(f"Создана конфигурация: {config_file}")
            except Exception as e:
                logger.error(f"Ошибка создания конфигурации {config_file}: {e}")

    def get_environment_config(self, env_name: str) -> Optional[Dict[str, Any]]:
        """Получает конфигурацию для указанного окружения."""
        return self.environments.get(env_name)

    def detect_environment(self) -> str:
        """Автоматически определяет текущее окружение."""
        # Проверяем переменные окружения
        if os.getenv("CI", "").lower() == "true":
            return "ci"
        if os.getenv("PYTEST_CURRENT_TEST"):
            return "testing"
        if os.getenv("ENV") in self.environments:
            return os.getenv("ENV")

        # Определяем по другим признакам
        if os.path.exists("/app"):  # Docker container
            return "production"
        if os.path.exists(".git"):  # Development
            return "development"

        return "development"  # Default


class ConfigValidator:
    """Валидатор конфигурации с подробными проверками."""

    def __init__(self):
        self.errors = []
        self.warnings = []

    def validate_config(self, config: Dict[str, Any]) -> Tuple[bool, List[str], List[str]]:
        """
        Полная валидация конфигурации.

        Returns:
            (is_valid, errors, warnings)
        """
        self.errors = []
        self.warnings = []

        # Валидация основных секций
        self._validate_database_config(config)
        self._validate_analysis_config(config)
        self._validate_preprocessing_config(config)
        self._validate_vectorization_config(config)
        self._validate_clustering_config(config)
        self._validate_output_config(config)

        # Проверки совместимости
        self._validate_compatibility(config)

        # Оптимизации и рекомендации
        self._validate_performance(config)

        is_valid = len(self.errors) == 0
        return is_valid, self.errors.copy(), self.warnings.copy()

    def _validate_database_config(self, config: Dict[str, Any]):
        """Валидация конфигурации базы данных."""
        db_config = config.get("db", {})

        # Обязательные поля
        required_fields = ["host", "port", "user", "password", "database"]
        for field in required_fields:
            value = db_config.get(field)
            # Пропускаем валидацию если значение - это переменная окружения
            if isinstance(value, str) and value.startswith("${"):
                continue
            if not value:
                self.errors.append(f"db.{field} - обязательное поле не заполнено")

        # Валидация порта
        port = db_config.get("port")
        if port:
            # Пропускаем валидацию если это переменная окружения
            if isinstance(port, str) and port.startswith("${"):
                pass
            else:
                try:
                    port_num = int(port)
                    if not (1024 <= port_num <= 65535):
                        self.errors.append(f"db.port - порт должен быть в диапазоне 1024-65535")
                except (ValueError, TypeError):
                    self.errors.append(f"db.port - некорректный номер порта")

        # Проверка подключения (опционально)
        if db_config.get("test_connection", True):
            self.warnings.append("Рекомендуется отключить test_connection в production для ускорения запуска")

    def _validate_analysis_config(self, config: Dict[str, Any]):
        """Валидация конфигурации анализа."""
        analysis_config = config.get("analysis", {})

        # Валидация временного окна
        time_window = analysis_config.get("time_window_hours", 24)
        if not isinstance(time_window, (int, float)) or time_window <= 0:
            self.errors.append("analysis.time_window_hours - должно быть положительным числом")
        elif time_window > 168:  # Неделя
            self.warnings.append("analysis.time_window_hours - большое окно анализа может привести к перегрузке")

        # Валидация количества нарративов
        top_narratives = analysis_config.get("top_narratives", 5)
        if not isinstance(top_narratives, int) or top_narratives <= 0:
            self.errors.append("analysis.top_narratives - должно быть положительным целым числом")
        elif top_narratives > 20:
            self.warnings.append("analysis.top_narratives - большое количество нарративов может снизить производительность")

    def _validate_preprocessing_config(self, config: Dict[str, Any]):
        """Валидация конфигурации предобработки."""
        preprocessing_config = config.get("preprocessing", {})

        # Валидация длины слов
        min_len = preprocessing_config.get("min_word_length", 3)
        max_len = preprocessing_config.get("max_word_length", 20)

        if not isinstance(min_len, int) or min_len <= 0:
            self.errors.append("preprocessing.min_word_length - должно быть положительным целым числом")

        if not isinstance(max_len, int) or max_len <= 0:
            self.errors.append("preprocessing.max_word_length - должно быть положительным целым числом")

        if min_len >= max_len:
            self.errors.append("preprocessing.min_word_length должно быть меньше max_word_length")

        # Проверка стоп-слов
        stopwords = preprocessing_config.get("stopwords_extra", [])
        if not isinstance(stopwords, list):
            self.errors.append("preprocessing.stopwords_extra - должно быть списком")

        # Проверка на слишком много стоп-слов
        if len(stopwords) > 1000:
            self.warnings.append("preprocessing.stopwords_extra - слишком много стоп-слов может снизить качество анализа")

    def _validate_vectorization_config(self, config: Dict[str, Any]):
        """Валидация конфигурации векторизации."""
        vectorization_config = config.get("vectorization", {})

        # Валидация max_features
        max_features = vectorization_config.get("max_features", 5000)
        if not isinstance(max_features, int) or max_features <= 0:
            self.errors.append("vectorization.max_features - должно быть положительным целым числом")
        elif max_features > 10000:
            self.warnings.append("vectorization.max_features - большое количество признаков может привести к перегрузке памяти")

        # Валидация параметров TF-IDF
        min_df = vectorization_config.get("min_df", 2)
        max_df = vectorization_config.get("max_df", 0.95)

        if not isinstance(min_df, (int, float)) or min_df < 0:
            self.errors.append("vectorization.min_df - должно быть неотрицательным числом")

        if not isinstance(max_df, (int, float)):
            self.errors.append("vectorization.max_df - должно быть числом")
        elif isinstance(max_df, float) and not (0 < max_df <= 1):
            self.errors.append("vectorization.max_df - должно быть числом от 0 до 1 (или целым числом для абсолютного порога)")

        # Проверка совместимости min_df и max_df
        # min_df может быть целым (абсолютное значение) или float (относительное)
        # max_df может быть float от 0 до 1 (относительное) или целым (абсолютное)
        # Сравнение имеет смысл только если оба параметра одного типа
        if isinstance(min_df, int) and isinstance(max_df, int):
            if min_df >= max_df:
                self.errors.append("vectorization.min_df должно быть меньше max_df")
        elif isinstance(min_df, float) and isinstance(max_df, float):
            if min_df >= max_df:
                self.errors.append("vectorization.min_df должно быть меньше max_df")

    def _validate_clustering_config(self, config: Dict[str, Any]):
        """Валидация конфигурации кластеризации."""
        clustering_config = config.get("clustering", {})

        # Валидация параметров HDBSCAN
        min_cluster_size = clustering_config.get("min_cluster_size", 5)
        min_samples = clustering_config.get("min_samples", 3)

        if not isinstance(min_cluster_size, int) or min_cluster_size <= 0:
            self.errors.append("clustering.min_cluster_size - должно быть положительным целым числом")

        if min_samples is not None:
            if not isinstance(min_samples, int) or min_samples <= 0:
                self.errors.append("clustering.min_samples - должно быть положительным целым числом")
            if min_samples > min_cluster_size:
                self.errors.append("clustering.min_samples не должно превышать min_cluster_size")

        # Валидация метрики
        valid_metrics = ["cosine", "euclidean", "manhattan"]
        metric = clustering_config.get("metric", "cosine")
        if metric not in valid_metrics:
            self.errors.append(f"clustering.metric - должна быть одной из: {valid_metrics}")

    def _validate_output_config(self, config: Dict[str, Any]):
        """Валидация конфигурации вывода."""
        output_config = config.get("output", {})

        # Валидация директорий
        reports_dir = output_config.get("reports_dir", "./storage/reports")
        logs_dir = output_config.get("logs_dir", "./storage/logs")

        for dir_path, dir_name in [(reports_dir, "reports_dir"), (logs_dir, "logs_dir")]:
            try:
                Path(dir_path).resolve()
            except Exception:
                self.errors.append(f"output.{dir_name} - некорректный путь к директории")

    def _validate_compatibility(self, config: Dict[str, Any]):
        """Проверка совместимости настроек."""
        # Проверка совместимости async и lemmatization
        if config.get("analysis", {}).get("use_lemmatization", False):
            if config.get("performance", {}).get("async_processing", False):
                self.warnings.append("Использование лемматизации с async processing может снизить производительность")

        # Проверка совместимости больших объемов данных с ограниченными ресурсами
        max_features = config.get("vectorization", {}).get("max_features", 5000)
        max_news = config.get("limits", {}).get("max_news_limit", 1200)

        if max_features > 5000 and max_news > 1000:
            self.warnings.append("Большое количество признаков и новостей может привести к нехватке памяти")

    def _validate_performance(self, config: Dict[str, Any]):
        """Проверка и рекомендации по производительности."""
        performance_config = config.get("performance", {})

        # Рекомендации для production
        if performance_config.get("async_processing", False):
            if not performance_config.get("use_lemmatization", True):
                self.warnings.append("Рекомендуется включить лемматизацию для лучшего качества анализа")

        # Проверка ресурсов
        max_features = config.get("vectorization", {}).get("max_features", 5000)
        if max_features > 8000:
            self.warnings.append("max_features > 8000 может потребовать > 4GB RAM")

    def get_validation_report(self) -> str:
        """Получить отчет о валидации."""
        report_lines = ["Конфигурация валидации - Отчет"]

        if self.errors:
            report_lines.append(f"\n❌ Ошибки ({len(self.errors)}):")
            for error in self.errors:
                report_lines.append(f"  - {error}")

        if self.warnings:
            report_lines.append(f"\n⚠️  Предупреждения ({len(self.warnings)}):")
            for warning in self.warnings:
                report_lines.append(f"  - {warning}")

        if not self.errors and not self.warnings:
            report_lines.append("\n✅ Конфигурация валидна, без предупреждений")

        return "\n".join(report_lines)


def load_and_validate_config(config_path: Path, env_name: str = None) -> Tuple[Dict[str, Any], bool, List[str], List[str]]:
    """
    Загружает и валидирует конфигурацию с учетом окружения.

    Returns:
        (config, is_valid, errors, warnings)
    """
    # Загружаем базовую конфигурацию
    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            config = yaml.safe_load(f)
    except Exception:
        # Возвращаем обобщенное сообщение без деталей исключения
        return {}, False, ["Ошибка загрузки конфигурации"], []

    # Определяем окружение
    if not env_name:
        env_detector = EnvironmentConfig(config_path)
        env_name = env_detector.detect_environment()

    # Загружаем environment-specific настройки
    env_detector = EnvironmentConfig(config_path)
    env_configs = env_detector.load_environment_configs()
    env_config = env_configs.get(env_name, {})

    # Мерджим конфигурации (environment overrides base)
    def merge_configs(base: Dict[str, Any], override: Dict[str, Any]) -> Dict[str, Any]:
        result = base.copy()
        for key, value in override.items():
            if isinstance(value, dict) and key in result and isinstance(result[key], dict):
                result[key] = merge_configs(result[key], value)
            else:
                result[key] = value
        return result

    merged_config = merge_configs(config, env_config)

    # Валидируем
    validator = ConfigValidator()
    is_valid, errors, warnings = validator.validate_config(merged_config)

    return merged_config, is_valid, errors, warnings