-- anonymize_data.sql
-- Script de anonimização de dados para o ambiente de Staging.
-- Executado automaticamente na inicialização do container PostgreSQL.
--
-- Este script mascara dados sensíveis em tabelas populadas por dump de produção:
-- - beerusers: email, external_sub (OAuth), campos de auth
-- - beers: campos que possam conter referências a utilizadores reais
-- - comments: texto de comentários (pode conter PII)
--
-- Política de masking:
-- - Emails: substituídos por padrões anonimizados mantendo formato válido
-- - Nomes/Textos: substituídos por "Usuário Anônimo [id]"
-- - Senhas: redefinidas para hash de senha de teste
-- - Tokens OAuth: invalidados/nulleados
-- - URLs de imagem: mantidas (não são PII)

-- ============================================================================
-- 1. ANONIMIZAÇÃO DE USUÁRIOS (beerusers)
-- ============================================================================

UPDATE beerusers
SET
    email = CONCAT('user_', id::text, '@staging.example.com'),
    username = CONCAT('Usuario Anonimo ', id::text),
    password = '$2a$10$staging.hash.fixo.para.todos.os.usuarios.de.teste',
    external_sub = NULL,
    role = CASE WHEN role = 'admin' THEN 'user' ELSE role END,
    updated_at = NOW()
WHERE email NOT LIKE '%@staging.example.com';

-- ============================================================================
-- 2. ANONIMIZAÇÃO DE COMENTÁRIOS (comments embutidos em beers.comments)
-- ============================================================================
-- Como comments é JSONB embutido em beers, atualizamos via JSONB path

UPDATE beers
SET
    comments = (
        SELECT jsonb_agg(
            jsonb_set(
                jsonb_set(
                    jsonb_set(
                        jsonb_build_object(
                            'id', c->>'id',
                            'beer_id', c->>'beer_id',
                            'rating', c->>'rating',
                            'likes', c->>'likes',
                            'likedBy', COALESCE(c->'likedBy', '[]'::jsonb),
                            'positive', c->>'positive',
                            'createdAt', c->>'createdAt',
                            'createdBy', c->>'createdBy',
                            'text', 'Comentário anonimizado para staging',
                            'createdByName', CONCAT('Usuario Anonimo ', COALESCE(c->>'createdBy', '0'))
                        ),
                        '{createdByName}', 'to_jsonb(CONCAT(''Usuario Anonimo '', COALESCE(c->>''createdBy'', ''0'')))'::jsonb
                    ),
                    '{text}', '"Comentário anonimizado para staging"'::jsonb
                ),
                '{user}', 'to_jsonb(CONCAT(''Usuario Anonimo '', COALESCE(c->>''createdBy'', ''0'')))'::jsonb
            )
        )
        FROM jsonb_array_elements(COALESCE(comments, '[]'::jsonb)) AS c
    ),
    updated_at = NOW()
WHERE comments IS NOT NULL AND jsonb_array_length(comments) > 0;

-- ============================================================================
-- 3. LIMPEZA DE SUBSCRIPTIONS PUSH (push_subscriptions)
-- ============================================================================

DELETE FROM push_subscriptions;

-- ============================================================================
-- 4. REINICIALIZAÇÃO DE ESTATÍSTICAS (opcional)
-- ============================================================================
-- Zera contadores que possam revelar padrões de produção

UPDATE beers
SET
    -- Mantém os valores de rating existentes mas limpa metadados de produção
    updated_at = NOW()
WHERE 1=1;

-- ============================================================================
-- 5. LOG DE AUDITORIA
-- ============================================================================

DO $$
BEGIN
    RAISE NOTICE '[STAGING ANONYMIZATION] Dados anonimizados com sucesso em %', NOW();
    RAISE NOTICE '[STAGING ANONYMIZATION] Tabelas afetadas: beerusers, beers, push_subscriptions';
END $$;
