-- Создание таблицы для хранения результатов анализа новостей
-- Эта таблица хранит исторические данные анализа для последующего использования

CREATE TABLE IF NOT EXISTS news_analysis (
    id SERIAL PRIMARY KEY,
    analysis_date TIMESTAMP NOT NULL,
    total_news INTEGER NOT NULL,
    narratives_count INTEGER NOT NULL,
    narratives JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    -- Индекс для быстрого поиска по дате анализа
    CONSTRAINT unique_analysis_date UNIQUE (analysis_date)
);

-- Индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_news_analysis_date ON news_analysis (analysis_date DESC);
CREATE INDEX IF NOT EXISTS idx_news_analysis_created_at ON news_analysis (created_at DESC);

-- Комментарии к таблице и полям
COMMENT ON TABLE news_analysis IS 'Исторические данные анализа новостей';
COMMENT ON COLUMN news_analysis.analysis_date IS 'Дата и время анализа';
COMMENT ON COLUMN news_analysis.total_news IS 'Общее количество проанализированных новостей';
COMMENT ON COLUMN news_analysis.narratives_count IS 'Количество найденных тем (нарративов)';
COMMENT ON COLUMN news_analysis.narratives IS 'JSON массив с нарративами (темами)';
COMMENT ON COLUMN news_analysis.created_at IS 'Время создания записи в БД';
