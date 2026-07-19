-- Liga contas externas (OAuth2/OIDC: Google, Apple) à mesma tabela beerUsers.
-- provider identifica a origem ("local" para email/senha, "google", "apple");
-- external_sub é o "sub" do IdP (subject estável do provedor). O par
-- (provider, external_sub) é único quando presente, permitindo upsert idempotente
-- de login social e ligação à mesma fila de utilizadores das rotas protegidas.
-- password fica NULL para contas sociais (nunca temos a palavra-passe do IdP).
ALTER TABLE beerUsers ADD COLUMN IF NOT EXISTS provider VARCHAR(20) NOT NULL DEFAULT 'local';
ALTER TABLE beerUsers ALTER COLUMN password DROP NOT NULL;
ALTER TABLE beerUsers ADD COLUMN IF NOT EXISTS external_sub VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_sub
  ON beerUsers(provider, external_sub) WHERE external_sub IS NOT NULL;
