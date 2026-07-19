-- Remove a tabela comments separada (legado). A API atual armazena comentários
-- como JSONB embutido em beers (modelo documento), não como tabela relacional.
-- Migração destrutiva segura: não há dados definitivos nem dependentes.
DROP TABLE IF EXISTS comments CASCADE;
