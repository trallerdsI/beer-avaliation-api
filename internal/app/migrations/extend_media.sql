-- RFC 7578 (Multipart Form Data): o app Flutter passa a enviar imagens via
-- upload server-side. O Go ingere o binário, valida e grava no Supabase Storage,
-- persistindo a URL. Alarga image_url para TEXT (URLs de CDN assinadas do
-- Storage passam de 255) e adiciona media JSONB para múltiplas imagens.

ALTER TABLE beers ALTER COLUMN image_url TYPE TEXT;

-- Array documento de mídias anexadas à cerveja. Inicializado como '[]'::jsonb
-- (nunca NULL) para evitar quebras de serialização no app Flutter.
ALTER TABLE beers ADD COLUMN IF NOT EXISTS media JSONB NOT NULL DEFAULT '[]'::jsonb;

-- ROLLBACK (Gap C):
-- ALTER TABLE beers ALTER COLUMN image_url TYPE VARCHAR(255);
-- ALTER TABLE beers DROP COLUMN IF EXISTS media;
