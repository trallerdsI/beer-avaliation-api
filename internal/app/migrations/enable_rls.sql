-- Segurança (Supabase Advisor): tabelas públicas sem RLS expõem dados e a
-- coluna password. Como a API é a fonte da verdade e usa a service-role key
-- (que bypass RLS), o RLS aqui protege acesso direto à BD (SQL Editor, clientes
-- que contornem a API) e fecha os 4 issues do Advisor.
--
-- O backend Go autentica via JWT e injeta user_id no contexto; as policies
-- abaixo usam auth.uid() = '...' (o UUID do utilizador) para escopar escritas.

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
    created_by = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id = auth.uid() AND beerUsers.role = 'admin')
  )
  WITH CHECK (
    created_by = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers WHERE beerUsers.id = auth.uid() AND beerUsers.role = 'admin')
  );

-- beerUsers: um utilizador só se vê/edge a si próprio; admin vê todos.
ALTER TABLE beerUsers ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS beerUsers_select ON beerUsers;
CREATE POLICY beerUsers_select ON beerUsers
  FOR SELECT
  USING (
    id = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id = auth.uid() AND b.role = 'admin')
  );

DROP POLICY IF EXISTS beerUsers_write ON beerUsers;
CREATE POLICY beerUsers_write ON beerUsers
  FOR ALL
  USING (
    id = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id = auth.uid() AND b.role = 'admin')
  )
  WITH CHECK (
    id = auth.uid()::text
    OR EXISTS (SELECT 1 FROM beerUsers b WHERE b.id = auth.uid() AND b.role = 'admin')
  );

-- comments (se ainda existir como tabela separada): lida como documento
-- embutido em beers; proteção defensiva caso a tabela legada persista.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'comments') THEN
    EXECUTE 'ALTER TABLE comments ENABLE ROW LEVEL SECURITY';
  END IF;
END $$;
