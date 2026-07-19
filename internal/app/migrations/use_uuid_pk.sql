-- RFC 9562: migra chaves primárias de SERIAL (int sequencial) para UUID,
-- gerados pela aplicação como UUIDv7 (time-ordered). Isto elimina IDs
-- sequenciais expostos (enumeração de /beers/1,2,3) e permite ordenação
-- cronológica natural do feed sem colisões em escritas offline do app Flutter.
--
-- Requer Postgres >= 13 (gen_random_uuid). UUIDv7 é gerado na app (pkg/uuid),
-- logo não dependemos de extensão uuid-ossp para a versão 7.
--
-- NOTA: um SERIAL cria um default nextval(<seq>). O Postgres não consegue
-- converter esse default automaticamente para UUID, daí o erro
-- "default for column id cannot be cast automatically to type uuid". Por isso
-- removemos o default ANTES do cast e restauramos um default UUID depois.

-- beerUsers: PK.
ALTER TABLE beerUsers ALTER COLUMN id DROP DEFAULT;
ALTER TABLE beerUsers ALTER COLUMN id SET DATA TYPE UUID USING gen_random_uuid();
ALTER TABLE beerUsers ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- beers: PK.
ALTER TABLE beers ALTER COLUMN id DROP DEFAULT;
ALTER TABLE beers ALTER COLUMN id SET DATA TYPE UUID USING gen_random_uuid();
ALTER TABLE beers ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- comments: só existe se a migration create_comments_table.sql correu.
-- Usamos um bloco DO para alterar apenas se a tabela existir, evitando
-- falha quando já foi removida.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'comments') THEN
    ALTER TABLE comments ALTER COLUMN id DROP DEFAULT;
    ALTER TABLE comments ALTER COLUMN id SET DATA TYPE UUID USING gen_random_uuid();
    ALTER TABLE comments ALTER COLUMN id SET DEFAULT gen_random_uuid();
    ALTER TABLE comments ALTER COLUMN beer_id SET DATA TYPE UUID USING gen_random_uuid();
    ALTER TABLE comments ALTER COLUMN user_id SET DATA TYPE UUID USING gen_random_uuid();

    ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_beer_id_fkey;
    ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_user_id_fkey;
    ALTER TABLE comments
      ADD CONSTRAINT comments_beer_id_fkey FOREIGN KEY (beer_id)
      REFERENCES beers(id) ON DELETE CASCADE;
    ALTER TABLE comments
      ADD CONSTRAINT comments_user_id_fkey FOREIGN KEY (user_id)
      REFERENCES beerUsers(id) ON DELETE CASCADE;
  END IF;
END $$;

-- Índice BRIN sobre created_at (e não 'created', que é coluna legada não lida
-- pelo repositório) para ordenação cronológica eficiente de feeds. A tabela
-- comments separada é removida por drop_legacy_comments_table.sql, logo NÃO se
-- cria índice nela (criaria erro e abortaria migrateDB).
CREATE INDEX IF NOT EXISTS idx_beers_created_brin ON beers USING brin (created_at);

-- ROLLBACK (Gap C): reverter para SERIAL caso necessário.
-- (Nota: reverter para SERIAL requer recriar sequences; mantido como referência.)
