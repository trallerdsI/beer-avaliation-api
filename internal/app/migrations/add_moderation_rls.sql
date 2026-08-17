-- Row Level Security (RLS) para tabelas de moderação
-- Garante que apenas admins e service_role acessem dados sensíveis,
-- e que usuários comuns apenas criem suas próprias denúncias/solicitações.

ALTER TABLE beer_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE beer_deletion_requests ENABLE ROW LEVEL SECURITY;

-- beer_reports: usuário autenticado pode inserir sua própria denúncia
CREATE POLICY insert_own_beer_report ON beer_reports
    FOR INSERT
    TO authenticated
    WITH CHECK (user_id = auth.uid());

-- beer_reports: admins podem ver todas as denúncias
CREATE POLICY admin_select_beer_reports ON beer_reports
    FOR SELECT
    TO authenticated
    USING (auth.jwt() ->> 'role' = 'admin');

-- beer_reports: admins podem atualizar denúncias (resolver/descartar)
CREATE POLICY admin_update_beer_reports ON beer_reports
    FOR UPDATE
    TO authenticated
    USING (auth.jwt() ->> 'role' = 'admin');

-- beer_deletion_requests: usuário autenticado pode inserir sua própria solicitação
CREATE POLICY insert_own_deletion_request ON beer_deletion_requests
    FOR INSERT
    TO authenticated
    WITH CHECK (user_id = auth.uid());

-- beer_deletion_requests: admins possuem acesso irrestrito para leitura e atualização
CREATE POLICY admin_manage_deletion_requests ON beer_deletion_requests
    FOR ALL
    TO authenticated
    USING (auth.jwt() ->> 'role' = 'admin');
