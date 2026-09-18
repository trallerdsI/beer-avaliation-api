-- Migration: add_beer_trgm_index
-- Descrição: Índice GIN com gin_trgm_ops para fallback fuzzy search
--
-- gin_trgm_ops não pode ser aplicada a expressões arbitrárias em índices GIN
-- (PostgreSQL rejeita com "syntax error at or near ||"). Usar coluna gerada
-- STORED, mesmo padrão já adotado em add_beer_fts.sql → search_vector.

ALTER TABLE beers ADD COLUMN IF NOT EXISTS name_desc_trgm TEXT
    GENERATED ALWAYS AS (COALESCE(name, '') || ' ' || COALESCE(description, '')) STORED;

DROP INDEX IF EXISTS idx_beers_name_description_trgm;
CREATE INDEX idx_beers_name_description_trgm
    ON beers
    USING GIN (name_desc_trgm gin_trgm_ops);
