-- Full-Text Search (FTS) para a tabela beers
-- Dicionário 'portuguese' + pesos A/B/C/D + índice GIN.
-- Idempotente e reversível.

-- 1. Coluna computada com pesos diferenciados
ALTER TABLE beers ADD COLUMN IF NOT EXISTS search_vector tsvector
  GENERATED ALWAYS AS (
    setweight(to_tsvector('portuguese', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('portuguese', COALESCE(style, '')), 'B') ||
    setweight(to_tsvector('portuguese', COALESCE(aroma, '')), 'C') ||
    setweight(to_tsvector('portuguese', COALESCE(color, '')), 'C') ||
    setweight(to_tsvector('portuguese', COALESCE(body, '')), 'C') ||
    setweight(to_tsvector('portuguese', COALESCE(description, '')), 'D')
  ) STORED;

-- 2. Índice invertido GIN
DROP INDEX IF EXISTS idx_beers_fts;
CREATE INDEX idx_beers_fts ON beers USING GIN (search_vector);

-- 3. Extensão para fallback fuzzy (trigram)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
