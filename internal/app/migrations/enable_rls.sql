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

-- beers: leitura pública (o catálogo é público); escrita só ao dono ou admin.
ALTER TABLE beers ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS beers_select ON beers;
CREATE POLICY beers_select ON beers
  FOR SELECT
  USING (true);

DROP POLICY IF EXISTS beers_write ON beers;
CREATE POLICY beers_write ON beers
  FOR ALL
  USING (
    created_by::text = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id::text = auth.uid()::text AND beerUsers.role = 'admin')
  )
  WITH CHECK (
    created_by::text = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id::text = auth.uid()::text AND beerUsers.role = 'admin')
  );

-- beerUsers: um utilizador só se vê/edge a si próprio; admin vê todos.
ALTER TABLE beerUsers ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS beerUsers_select ON beerUsers;
CREATE POLICY beerUsers_select ON beerUsers
  FOR SELECT
  USING (
    id::text = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id::text = auth.uid()::text AND b.role = 'admin')
  );

DROP POLICY IF EXISTS beerUsers_write ON beerUsers;
CREATE POLICY beerUsers_write ON beerUsers
  FOR ALL
  USING (
    id::text = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id::text = auth.uid()::text AND b.role = 'admin')
  )
  WITH CHECK (
    id::text = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id::text = auth.uid()::text AND b.role = 'admin')
  );

-- comments (se ainda existir como tabela separada): proteção defensiva caso
-- a tabela legada persista. Usa CAST para TEXT para evitar uuid = text.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'comments') THEN
    EXECUTE 'ALTER TABLE comments ENABLE ROW LEVEL SECURITY';
  END IF;
END $$;
