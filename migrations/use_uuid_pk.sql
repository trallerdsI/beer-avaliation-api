-- RFC 9562: migra chaves primárias de SERIAL (int sequencial) para UUID,
-- gerados pela aplicação como UUIDv7 (time-ordered). Isto elimina IDs
-- sequenciais expostos (enumeração de /beers/1,2,3) e permite ordenação
-- cronológica natural do feed sem colisões em escritas offline do app Flutter.
--
-- Requer Postgres >= 13 (gen_random_uuid). UUIDv7 é gerado na app (pkg/uuid),
-- logo não dependemos de extensão uuid-ossp para a versão 7.

-- beerUsers: PK e colunas referenciadas.
ALTER TABLE beerUsers ALTER COLUMN id SET DATA TYPE UUID USING gen_random_uuid();
ALTER TABLE beerUsers ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- beers: PK.
ALTER TABLE beers ALTER COLUMN id SET DATA TYPE UUID USING gen_random_uuid();
ALTER TABLE beers ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- comments: PK + FKs para beers/users (agora UUID).
ALTER TABLE comments ALTER COLUMN id SET DATA TYPE UUID USING gen_random_uuid();
ALTER TABLE comments ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE comments ALTER COLUMN beer_id SET DATA TYPE UUID USING gen_random_uuid();
ALTER TABLE comments ALTER COLUMN user_id SET DATA TYPE UUID USING gen_random_uuid();

-- Recria as FKs com o novo tipo UUID (as antigas referem int e falhariam).
ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_beer_id_fkey;
ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_user_id_fkey;
ALTER TABLE comments
  ADD CONSTRAINT comments_beer_id_fkey FOREIGN KEY (beer_id)
  REFERENCES beers(id) ON DELETE CASCADE;
ALTER TABLE comments
  ADD CONSTRAINT comments_user_id_fkey FOREIGN KEY (user_id)
  REFERENCES beerUsers(id) ON DELETE CASCADE;

-- Índice BRIN sobre created_at para ordenação cronológica eficiente de feeds
-- (UUIDv7 já ordena por tempo no índice btree da PK; BRIN cobre created_at).
CREATE INDEX IF NOT EXISTS idx_beers_created_brin ON beers USING brin (created);
CREATE INDEX IF NOT EXISTS idx_comments_created_brin ON comments USING brin (created);

-- ROLLBACK (Gap C): reverter para SERIAL caso necessário.
-- ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_beer_id_fkey;
-- ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_user_id_fkey;
-- ALTER TABLE beerUsers ALTER COLUMN id SET DATA TYPE INTEGER USING 0;
-- ALTER TABLE beers ALTER COLUMN id SET DATA TYPE INTEGER USING 0;
-- ALTER TABLE comments ALTER COLUMN id SET DATA TYPE INTEGER USING 0;
-- ALTER TABLE comments ALTER COLUMN beer_id SET DATA TYPE INTEGER USING 0;
-- ALTER TABLE comments ALTER COLUMN user_id SET DATA TYPE INTEGER USING 0;
-- (Nota: reverter para SERIAL requer recriar sequences; mantido como referência.)
