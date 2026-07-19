-- Adiciona a coluna role em beerUsers (faltava na migration original, mas o
-- código de registo/login já persiste e lê role). Default 'user' para
-- registos preexistentes; admins são criados via seed/upsert explícito.
ALTER TABLE beerUsers ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'user';
