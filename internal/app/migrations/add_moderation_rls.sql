-- Row Level Security (RLS) para tabelas de moderação
-- Garante que apenas admins acessem dados sensíveis,
-- e que usuários comuns apenas criem suas próprias denúncias/solicitações.
-- Compatível com Supabase e PostgreSQL puro.

ALTER TABLE beer_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE beer_deletion_requests ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS insert_own_beer_report ON beer_reports;
DROP POLICY IF EXISTS admin_select_beer_reports ON beer_reports;
DROP POLICY IF EXISTS admin_update_beer_reports ON beer_reports;
DROP POLICY IF EXISTS insert_own_deletion_request ON beer_deletion_requests;
DROP POLICY IF EXISTS admin_manage_deletion_requests ON beer_deletion_requests;

-- beer_reports: qualquer usuário autenticado pode inserir denúncia
CREATE POLICY insert_own_beer_report ON beer_reports
    FOR INSERT
    WITH CHECK (user_id::text = auth.uid()::text);

-- beer_reports: admins podem ver todas as denúncias
CREATE POLICY admin_select_beer_reports ON beer_reports
    FOR SELECT
    USING (EXISTS (SELECT 1 FROM beerUsers u WHERE u.id::text = auth.uid()::text AND u.role = 'admin'));

-- beer_reports: admins podem atualizar denúncias (resolver/descartar)
CREATE POLICY admin_update_beer_reports ON beer_reports
    FOR UPDATE
    USING (EXISTS (SELECT 1 FROM beerUsers u WHERE u.id::text = auth.uid()::text AND u.role = 'admin'));

-- beer_deletion_requests: qualquer usuário autenticado pode inserir solicitação
CREATE POLICY insert_own_deletion_request ON beer_deletion_requests
    FOR INSERT
    WITH CHECK (user_id::text = auth.uid()::text);

-- beer_deletion_requests: admins possuem acesso irrestrito para leitura e atualização
CREATE POLICY admin_manage_deletion_requests ON beer_deletion_requests
    FOR ALL
    USING (EXISTS (SELECT 1 FROM beerUsers u WHERE u.id::text = auth.uid()::text AND u.role = 'admin'));
