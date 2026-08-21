-- Migration: add_beer_trgm_index
-- Descrição: Índice GIN com gin_trgm_ops para fallback fuzzy search

-- Índice para busca trigram em nome + descrição (usado no fallback do repositório)
DROP INDEX IF EXISTS idx_beers_name_description_trgm;
CREATE INDEX idx_beers_name_description_trgm
    ON beers
    USING GIN (
        COALESCE(name, '') || ' ' || COALESCE(description, '')
        gin_trgm_ops
    );
