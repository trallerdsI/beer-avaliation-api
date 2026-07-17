-- Adiciona created_at em beers para ordenação cronológica do feed social.
-- Default now() garante valor mesmo para cervejas criadas fora do app.
ALTER TABLE beers ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
