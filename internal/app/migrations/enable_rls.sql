-- Segurança (Supabase Advisor): tabelas públicas sem RLS expõem dados e a
-- coluna password. Como a API é a fonte da verdade e usa a service-role key
-- (que bypass RLS), o RLS aqui protege acesso direto à BD (SQL Editor, clientes
-- que contornem a API) e fecha os 4 issues do Advisor.
--
-- O backend Go autentica via JWT e injeta user_id no contexto; as policies
-- abaixo usam auth.uid()::text para escopar escritas. ATENÇÃO: created_by/id são
-- UUID e auth.uid()::text é TEXT, logo comparamos sempre os dois lados como TEXT
-- (uuid::text = auth.uid()::text) para evitar "operator does not exist:
-- uuid = text".

-- Supabase provides auth.uid(); plain PostgreSQL providers such as Neon do not.
-- Keep this migration a no-op outside Supabase so the API can use its database
-- role without an artificial auth schema or broken startup migration.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'auth') THEN
    EXECUTE 'ALTER TABLE beers ENABLE ROW LEVEL SECURITY';
    EXECUTE 'DROP POLICY IF EXISTS beers_select ON beers';
    EXECUTE 'DROP POLICY IF EXISTS beers_write ON beers';
    EXECUTE 'CREATE POLICY beers_select ON beers FOR SELECT USING (true)';
    EXECUTE 'DROP POLICY IF EXISTS beers_insert ON beers';
    EXECUTE $policy$
      CREATE POLICY beers_insert ON beers
      FOR INSERT
      WITH CHECK (
        created_by::text = auth.uid()::text
        OR EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id::text = auth.uid()::text AND beerUsers.role = 'admin')
      )
    $policy$;
    EXECUTE 'DROP POLICY IF EXISTS beers_update ON beers';
    EXECUTE $policy$
      CREATE POLICY beers_update ON beers
      FOR UPDATE
      USING (
        created_by::text = auth.uid()::text
        OR EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id::text = auth.uid()::text AND beerUsers.role = 'admin')
      )
      WITH CHECK (
        created_by::text = auth.uid()::text
        OR EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id::text = auth.uid()::text AND beerUsers.role = 'admin')
      )
    $policy$;
    EXECUTE 'DROP POLICY IF EXISTS beers_delete ON beers';
    EXECUTE $policy$
      CREATE POLICY beers_delete ON beers
      FOR DELETE
      USING (EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id::text = auth.uid()::text AND beerUsers.role = 'admin'))
    $policy$;

    EXECUTE 'ALTER TABLE beerUsers ENABLE ROW LEVEL SECURITY';
    EXECUTE 'DROP POLICY IF EXISTS beerUsers_select ON beerUsers';
    EXECUTE $policy$
      CREATE POLICY beerUsers_select ON beerUsers
      FOR SELECT
      USING (
        id::text = auth.uid()::text
        OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id::text = auth.uid()::text AND b.role = 'admin')
      )
    $policy$;
    EXECUTE 'DROP POLICY IF EXISTS beerUsers_write ON beerUsers';
    EXECUTE $policy$
      CREATE POLICY beerUsers_write ON beerUsers
      FOR ALL
      USING (
        id::text = auth.uid()::text
        OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id::text = auth.uid()::text AND b.role = 'admin')
      )
      WITH CHECK (
        id::text = auth.uid()::text
        OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id::text = auth.uid()::text AND b.role = 'admin')
      )
    $policy$;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'comments') THEN
      EXECUTE 'ALTER TABLE comments ENABLE ROW LEVEL SECURITY';
    END IF;
  END IF;
END $$;
