-- RFC 9111 (HTTP Caching): adiciona updated_at para suportar ETag de
-- validação por versão em recursos estáveis (detalhe de cerveja/perfil).
-- O ETag é derivado de updated_at no servidor; o cliente reenvia
-- If-None-Match e recebe 304 Not Modified quando não houve alteração,
-- poupando franquia de dados e bateria no app Flutter (redes 3G/4G/5G).

ALTER TABLE beers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE beerUsers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Trigger que mantém updated_at a cada UPDATE (sem depender do app).
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Cria triggers apenas se não existirem (evita deadlock em produção).
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_beers_updated_at') THEN
        CREATE TRIGGER trg_beers_updated_at
          BEFORE UPDATE ON beers
          FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_beerUsers_updated_at') THEN
        CREATE TRIGGER trg_beerUsers_updated_at
          BEFORE UPDATE ON beerUsers
          FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END;
$$;

-- ROLLBACK (Gap C):
-- DROP TRIGGER IF EXISTS trg_beers_updated_at ON beers;
-- DROP TRIGGER IF EXISTS trg_beerUsers_updated_at ON beerUsers;
-- DROP FUNCTION IF EXISTS set_updated_at();
-- ALTER TABLE beers DROP COLUMN IF EXISTS updated_at;
-- ALTER TABLE beerUsers DROP COLUMN IF EXISTS updated_at;
