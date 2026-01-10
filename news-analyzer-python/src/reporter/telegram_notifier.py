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
            payload = {
                "chat_id": chat_id,
                "text": text,
                "parse_mode": self.parse_mode,
                "disable_notification": disable_notification
            }
            
            response = requests.post(url, json=payload, timeout=10)
            response.raise_for_status()
            
            logger.debug(f"Сообщение успешно отправлено в Telegram (chat_id: {chat_id})")
            return True
            
        except requests.exceptions.HTTPError as e:
            # Обрабатываем специфичные ошибки Telegram API
            if e.response.status_code == 403:
                logger.warning(f"Бот заблокирован пользователем (chat_id: {chat_id})")
            elif e.response.status_code == 400:
                logger.warning(f"Некорректный запрос для chat_id {chat_id}: {e}")
            else:
                logger.error(f"HTTP ошибка при отправке сообщения (chat_id: {chat_id}): {e}")
            return False
        except requests.exceptions.RequestException as e:
            logger.error(f"Ошибка при отправке сообщения в Telegram (chat_id: {chat_id}): {e}")
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
                    for news_item in news_examples:
                        title = news_item.get("title", "")
                        source_name = news_item.get("source_name", "Неизвестный источник")
                        # Обрезаем длинные заголовки для Telegram
                        title_short = title[:60] + "..." if len(title) > 60 else title
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
        max_length = 3500  # Уменьшаем для надежности
        
        if len(summary_text) <= max_length:
            return self.send_message(chat_id, summary_text)
        else:
            # Разбиваем на части
            parts = []
            current_part = ""
            
            for line in summary_text.split("\n"):
                if len(current_part) + len(line) + 1 > max_length:
                    parts.append(current_part)
                    current_part = line + "\n"
                else:
                    current_part += line + "\n"
            
            if current_part:
                parts.append(current_part)
            
            # Отправляем все части
            success = True
            for i, part in enumerate(parts, 1):
                if len(parts) > 1:
                    part = f"Часть {i}/{len(parts)}\n\n{part}"
                if not self.send_message(chat_id, part):
                    success = False
            
            return success
