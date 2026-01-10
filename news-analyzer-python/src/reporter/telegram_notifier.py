"""Отправка отчетов в Telegram бот."""

import json
import os
from pathlib import Path
from typing import Dict, Any, Optional, List
import requests
from datetime import datetime

from ..utils.logger import get_logger

logger = get_logger(__name__)


class TelegramNotifier:
    """Класс для отправки отчетов в Telegram."""
    
    def __init__(
        self,
        bot_token: str,
        parse_mode: Optional[str] = None
    ):
        """
        Инициализация notifier.
        
        Args:
            bot_token: Токен Telegram бота
            parse_mode: Режим парсинга (Markdown, HTML, или None)
        """
        self.bot_token = bot_token
        self.parse_mode = parse_mode
        self.api_url = f"https://api.telegram.org/bot{bot_token}"
    
    def send_message(self, chat_id: int, text: str, disable_notification: bool = False) -> bool:
        """
        Отправляет текстовое сообщение в Telegram конкретному пользователю.

        Args:
            chat_id: ID чата для отправки
            text: Текст сообщения
            disable_notification: Отключить уведомление

        Returns:
            True если успешно, False в противном случае
        """
        try:
            url = f"{self.api_url}/sendMessage"

            # Полностью убираем parse_mode из запроса для совместимости
            payload = {
                "chat_id": chat_id,
                "text": text,
                "disable_notification": disable_notification
            }

            logger.debug(f"Отправка сообщения в Telegram (chat_id: {chat_id}, длина: {len(text)})")

            response = requests.post(url, json=payload, timeout=10)
            response.raise_for_status()

            logger.debug(f"Сообщение успешно отправлено в Telegram (chat_id: {chat_id})")
            return True

        except requests.exceptions.HTTPError as e:
            # Более детальное логирование для диагностики проблем
            logger.error(f"HTTP ошибка при отправке сообщения (chat_id: {chat_id}): {e}")
            logger.error(f"URL: {url}")
            logger.error(f"Статус код: {e.response.status_code}")
            logger.error(f"Ответ сервера: {e.response.text}")

            # Логируем первые 500 символов текста для диагностики
            text_preview = text[:500] + "..." if len(text) > 500 else text
            logger.error(f"Текст сообщения (первые 500 символов): {repr(text_preview)}")

            # Обрабатываем специфичные ошибки Telegram API
            if e.response.status_code == 403:
                logger.warning(f"Бот заблокирован пользователем (chat_id: {chat_id})")
            elif e.response.status_code == 400:
                logger.error(f"Некорректный запрос для chat_id {chat_id}: {e}")
                # Пробуем отправить без parse_mode, если была проблема с форматированием
                if self.parse_mode is not None:
                    logger.info(f"Повторная попытка без parse_mode для chat_id {chat_id}")
                    try:
                        payload_no_parse = {
                            "chat_id": chat_id,
                            "text": text,
                            "disable_notification": disable_notification
                        }
                        response_retry = requests.post(url, json=payload_no_parse, timeout=10)
                        response_retry.raise_for_status()
                        logger.info(f"Сообщение успешно отправлено без parse_mode (chat_id: {chat_id})")
                        return True
                    except Exception as retry_e:
                        logger.error(f"Повторная попытка также неудачна: {retry_e}")
            else:
                logger.error(f"HTTP ошибка при отправке сообщения (chat_id: {chat_id}): {e}")
            return False
        except requests.exceptions.RequestException as e:
            logger.error(f"Ошибка сети при отправке сообщения в Telegram (chat_id: {chat_id}): {e}")
            return False
        except Exception as e:
            logger.error(f"Неожиданная ошибка при отправке в Telegram (chat_id: {chat_id}): {e}")
            return False
    
    def send_message_to_all(self, chat_ids: List[int], text: str, disable_notification: bool = False) -> Dict[int, bool]:
        """
        Отправляет сообщение нескольким пользователям.
        
        Args:
            chat_ids: Список ID чатов для отправки
            text: Текст сообщения
            disable_notification: Отключить уведомление
            
        Returns:
            Словарь {chat_id: success} с результатами отправки
        """
        import time
        import os
        
        results = {}
        successful = 0
        failed = 0
        
        logger.info(f"Отправка сообщения {len(chat_ids)} пользователям...")
        
        # Оптимизация: добавляем задержку между отправками для снижения нагрузки на API
        # Telegram API имеет лимит: 30 сообщений в секунду для бота
        # Используем задержку 0.05 секунды (20 сообщений в секунду) для безопасности
        # Для контейнера используем ANALYZER_TELEGRAM_DELAY, если установлена
        delay_between_messages = float(os.getenv("ANALYZER_TELEGRAM_DELAY",
                                                os.getenv("TELEGRAM_SEND_DELAY", "0.05")))
        
        for idx, chat_id in enumerate(chat_ids):
            success = self.send_message(chat_id, text, disable_notification)
            results[chat_id] = success
            if success:
                successful += 1
            else:
                failed += 1
            
            # Добавляем задержку между отправками (кроме последнего сообщения)
            if idx < len(chat_ids) - 1:
                time.sleep(delay_between_messages)
        
        logger.info(f"Отправка завершена: успешно {successful}, ошибок {failed}")
        return results
    
    def send_report(self, chat_id: int, report_path: Path) -> bool:
        """
        Отправляет отчет из JSON файла в Telegram конкретному пользователю.
        
        Args:
            chat_id: ID чата для отправки
            report_path: Путь к JSON файлу отчета
            
        Returns:
            True если успешно, False в противном случае
        """
        try:
            # Загружаем отчет
            with open(report_path, "r", encoding="utf-8") as f:
                report = json.load(f)
            
            # Форматируем сообщение
            message = self._format_report_message(report)
            
            # Отправляем
            return self.send_message(chat_id, message)
            
        except FileNotFoundError:
            logger.error(f"Файл отчета не найден: {report_path}")
            return False
        except json.JSONDecodeError as e:
            logger.error(f"Ошибка при парсинге JSON отчета: {e}")
            return False
        except Exception as e:
            logger.error(f"Ошибка при отправке отчета: {e}")
            return False
    
    def send_report_to_all(self, chat_ids: List[int], report_path: Path) -> Dict[int, bool]:
        """
        Отправляет отчет всем указанным пользователям.
        
        Args:
            chat_ids: Список ID чатов для отправки
            report_path: Путь к JSON файлу отчета
            
        Returns:
            Словарь {chat_id: success} с результатами отправки
        """
        try:
            # Загружаем отчет один раз
            with open(report_path, "r", encoding="utf-8") as f:
                report = json.load(f)
            
            # Форматируем сообщение один раз
            message = self._format_report_message(report)
            
            # Отправляем всем пользователям
            return self.send_message_to_all(chat_ids, message)
            
        except FileNotFoundError:
            logger.error(f"Файл отчета не найден: {report_path}")
            return {}
        except json.JSONDecodeError as e:
            logger.error(f"Ошибка при парсинге JSON отчета: {e}")
            return {}
        except Exception as e:
            logger.error(f"Ошибка при отправке отчета: {e}")
            return {}
    
    def _format_report_message(self, report: Dict[str, Any]) -> str:
        """
        Форматирует отчет в текстовое сообщение для Telegram.
        
        Args:
            report: Словарь с данными отчета
            
        Returns:
            Отформатированное сообщение
        """
        lines = []
        
        # Заголовок
        analysis_date = datetime.fromisoformat(report.get("analysis_date", ""))
        date_str = analysis_date.strftime("%d.%m.%Y %H:%M")
        lines.append(f"📊 *КАРТА ДНЯ* - {date_str}")
        lines.append("")
        
        # Общая статистика
        total_news = report.get("total_news", 0)
        narratives_count = report.get("narratives_count", 0)
        lines.append(f"📰 Всего новостей: {total_news}")
        lines.append(f"🎯 Выявлено тем: {narratives_count}")
        lines.append("")
        
        # Топ-темы
        narratives = report.get("narratives", [])
        if narratives:
            lines.append("*ТОП-ТЕМЫ ДНЯ:*")
            lines.append("")
            
            for idx, narrative in enumerate(narratives[:5], 1):  # Топ-5
                size = narrative.get("size", 0)
                keywords = narrative.get("keywords", [])[:5]  # Первые 5 ключевых слов
                news_examples = narrative.get("news_examples", [])[:2]  # Первые 2 примера новостей

                lines.append(f"*{idx}. Тема #{narrative.get('cluster_id', idx-1)}* ({size} новостей)")

                if keywords:
                    keywords_str = ", ".join(keywords)
                    lines.append(f"🔑 Ключевые слова: {keywords_str}")

                if news_examples:
                    lines.append("📰 Примеры:")
                    # Показываем только 2 примера для экономии места в Telegram
                    for news_item in news_examples[:2]:
                        title = news_item.get("title", "")
                        source_name = news_item.get("source_name", "Неизвестный источник")
                        # Обрезаем длинные заголовки для Telegram
                        title_short = title[:50] + "..." if len(title) > 50 else title
                        lines.append(f"  • {title_short} ({source_name})")

                lines.append("")
        else:
            lines.append("⚠️ Темы не найдены")
        
        # Подвал
        lines.append("─" * 7)
        lines.append("_Автоматический анализ новостей_")
        
        return "\n".join(lines)
    
    def send_summary(self, chat_id: int, summary_text: str) -> bool:
        """
        Отправляет текстовое резюме в Telegram конкретному пользователю.

        Args:
            chat_id: ID чата для отправки
            summary_text: Текст резюме

        Returns:
            True если успешно, False в противном случае
        """
        # Telegram имеет лимит на длину сообщения (4096 символов)
        # Учитываем, что заголовки частей тоже занимают место
        max_length_per_part = 1800  # Еще более консервативный лимит для надежности

        logger.info(f"Отправка сообщения длиной {len(summary_text)} символов (лимит на часть: {max_length_per_part})")

        if len(summary_text) <= max_length_per_part:
            logger.info("Отправка одним сообщением")
            return self.send_message(chat_id, summary_text)
        else:
            # Разбиваем на части по строкам для надежности
            logger.info("Разбиение на части...")
            parts = []
            current_part = ""

            for line in summary_text.split("\n"):
                # Проверяем, не превысит ли добавление строки лимит
                # Добавляем 1 для символа новой строки
                if len(current_part) + len(line) + 1 > max_length_per_part:
                    if current_part:  # Не добавляем пустые части
                        parts.append(current_part)
                    current_part = line + "\n"
                else:
                    current_part += line + "\n"

            if current_part:  # Добавляем последнюю часть
                parts.append(current_part)

            logger.info(f"Создано {len(parts)} частей для отправки")

            # Отправляем все части
            success = True
            for i, part in enumerate(parts, 1):
                if len(parts) > 1:
                    # Убираем эмодзи и Markdown из заголовков частей для надежности
                    part_header = f"Часть {i}/{len(parts)}\n\n"
                    # Проверяем, чтобы общая длина части с заголовком не превышала лимит
                    if len(part_header) + len(part) > 3500:  # Консервативный лимит Telegram
                        # Если часть все еще слишком длинная, обрезаем ее
                        part = part[:3500 - len(part_header) - 50] + "\n\n[Сообщение было обрезано]"
                        logger.warning(f"Часть {i} была обрезана из-за превышения лимита Telegram.")
                    part = part_header + part
                    logger.info(f"Отправка части {i}/{len(parts)} (длина: {len(part)})")
                else:
                    logger.info(f"Отправка единственной части (длина: {len(part)})")

                if not self.send_message(chat_id, part):
                    success = False
                    logger.error(f"Не удалось отправить часть {i}")

            return success

    def send_themes_separately(
        self,
        chat_id: int,
        narratives: List[Dict[str, Any]],
        total_news: int,
        analysis_date: datetime,
        clustering_metrics: Optional[Dict[str, Any]] = None
    ) -> bool:
        """
        Отправляет каждую тему отдельным сообщением.

        Args:
            chat_id: ID чата для отправки
            narratives: Список нарративов
            total_news: Общее количество новостей
            analysis_date: Дата анализа
            clustering_metrics: Метрики кластеризации

        Returns:
            True если успешно, False в противном случае
        """
        try:
            logger.info(f"Отправка {len(narratives)} тем отдельными сообщениями")

            # Сначала отправляем заголовок с общей информацией
            header_text = self._format_analysis_header(total_news, len(narratives), analysis_date, clustering_metrics)
            if not self.send_message(chat_id, header_text):
                logger.error("Не удалось отправить заголовок анализа")
                return False

            # Затем отправляем каждую тему отдельно
            for idx, narrative in enumerate(narratives, 1):
                theme_text = self._format_single_theme(narrative, idx)
                if not self.send_message(chat_id, theme_text):
                    logger.error(f"Не удалось отправить тему #{idx}")
                    return False

                # Небольшая задержка между сообщениями
                import time
                time.sleep(0.1)

            logger.info(f"Успешно отправлено {len(narratives)} тем")
            return True

        except Exception as e:
            logger.error(f"Ошибка при отправке тем отдельно: {e}")
            return False

    def _format_analysis_header(
        self,
        total_news: int,
        themes_count: int,
        analysis_date: datetime,
        clustering_metrics: Optional[Dict[str, Any]] = None
    ) -> str:
        """Форматирует заголовок анализа."""
        lines = []
        lines.append("=" * 60)
        lines.append(f"КАРТА ДНЯ - {analysis_date.strftime('%d.%m.%Y')}")
        lines.append("=" * 60)
        lines.append("")
        lines.append(f"Всего новостей: {total_news}")
        lines.append(f"Выявлено тем: {themes_count}")

        if clustering_metrics:
            lines.append("")
            lines.append("📊 Метрики кластеризации:")
            lines.append(f"   • Кластеров: {clustering_metrics.get('total_clusters', 0)}")
            lines.append(f"   • Шумовых точек: {clustering_metrics.get('noise_points', 0)} ({clustering_metrics.get('noise_percentage', 0):.1f}%)")
            if clustering_metrics.get('total_clusters', 0) > 0:
                lines.append(f"   • Средний размер кластера: {clustering_metrics.get('avg_cluster_size', 0):.1f}")
                lines.append(f"   • Максимальный кластер: {clustering_metrics.get('max_cluster_size', 0)} новостей")
                lines.append(f"   • Минимальный кластер: {clustering_metrics.get('min_cluster_size', 0)} новостей")

        lines.append("")
        lines.append("Каждая тема будет отправлена отдельным сообщением.")
        lines.append("=" * 60)

        return "\n".join(lines)

    def _format_single_theme(self, narrative: Dict[str, Any], theme_number: int) -> str:
        """Форматирует одну тему для отправки."""
        lines = []

        size = narrative.get('size', 0)
        keywords = narrative.get('keywords', [])[:5]
        news_examples = narrative.get('news_examples', [])[:3]  # Показываем до 3 примеров

        lines.append(f"ТЕМА #{theme_number} (новостей: {size})")
        lines.append("-" * 60)
        lines.append(f"Ключевые слова: {', '.join(keywords)}")
        lines.append("")
        lines.append("Примеры новостей:")

        for news_item in news_examples:
            title = news_item.get('title', '')
            source_name = news_item.get('source_name', 'Неизвестный источник')
            link = news_item.get('link', '')

            lines.append(title)
            lines.append(source_name)
            lines.append(link)
            lines.append("")

        return "\n".join(lines)

    def _split_by_topics(self, text: str, max_length: int) -> List[str]:
        """
        Разбивает текст на части по темам для лучшей читаемости.

        Args:
            text: Полный текст отчета
            max_length: Максимальная длина одной части

        Returns:
            Список частей текста
        """
        lines = text.split('\n')
        parts = []
        current_part = ""

        for line in lines:
            # Если это начало новой темы
            if line.startswith('ТЕМА #') or line.startswith('============================================================'):
                # Если текущая часть не пустая и достаточно большая, сохраняем её
                if current_part and len(current_part) > max_length * 0.7:
                    parts.append(current_part.rstrip())
                    current_part = ""

            # Добавляем строку к текущей части
            if len(current_part) + len(line) + 1 > max_length:
                if current_part:
                    parts.append(current_part.rstrip())
                current_part = line + "\n"
            else:
                current_part += line + "\n"

        # Добавляем последнюю часть
        if current_part:
            parts.append(current_part.rstrip())

        return parts
