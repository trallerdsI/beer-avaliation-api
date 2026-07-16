-- Adiciona coluna JSONB para armazenar os comentários embutidos na cerveja
-- (modelo documento, alinhado com a struct Beer.Comments do domínio).
ALTER TABLE beers ADD COLUMN IF NOT EXISTS comments JSONB NOT NULL DEFAULT '[]'::jsonb;
