-- Миграция для исправления экранированных символов в заголовках новостей
-- Удаляет обратные слеши перед дефисами и другими символами, которые не должны быть экранированы

-- Исправляем дефисы: заменяем \- на -
UPDATE news 
SET title = REPLACE(title, '\-', '-')
WHERE title LIKE '%\-%';

-- Исправляем точки: заменяем \. на . (если они были экранированы)
UPDATE news 
SET title = REPLACE(title, '\.', '.')
WHERE title LIKE '%\.%';

-- Обновляем tsvector для полнотекстового поиска после изменения заголовков
-- Это произойдет автоматически благодаря GENERATED ALWAYS, но можно принудительно обновить
UPDATE news 
SET updated_at = NOW()
WHERE title LIKE '%\%' OR title LIKE '%\.%';

-- Проверка: сколько новостей было исправлено
-- SELECT COUNT(*) FROM news WHERE title LIKE '%\%' OR title LIKE '%\.%';
