-- Adiciona a coluna created_by em beers (AuthZ: só o criador ou um admin
-- podem editar/apagar a cerveja). Faltava na migration original, mas o
-- repositório já persiste e lê created_by.
ALTER TABLE beers ADD COLUMN IF NOT EXISTS created_by VARCHAR(255) NOT NULL DEFAULT '';
