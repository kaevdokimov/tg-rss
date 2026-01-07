-- PostgreSQL initialization script
-- This script ensures the database and user are properly set up

-- Create the news_bot database if it doesn't exist
-- (PostgreSQL Docker image already creates it via POSTGRES_DB env var)

-- Connect to the news_bot database and set up basic schema
\c news_bot;

-- Ensure the user has proper privileges
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO postgres;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO postgres;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO postgres;
