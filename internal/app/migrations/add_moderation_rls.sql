-- RLS for moderation tables when running on Supabase.
-- Neon/plain PostgreSQL has no auth.uid(), so this is intentionally a no-op
-- there; the API authorization layer remains responsible for access control.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'auth') THEN
        EXECUTE 'ALTER TABLE beer_reports ENABLE ROW LEVEL SECURITY';
        EXECUTE 'ALTER TABLE beer_deletion_requests ENABLE ROW LEVEL SECURITY';

        EXECUTE 'DROP POLICY IF EXISTS insert_own_beer_report ON beer_reports';
        EXECUTE 'DROP POLICY IF EXISTS admin_select_beer_reports ON beer_reports';
        EXECUTE 'DROP POLICY IF EXISTS admin_update_beer_reports ON beer_reports';
        EXECUTE 'DROP POLICY IF EXISTS insert_own_deletion_request ON beer_deletion_requests';
        EXECUTE 'DROP POLICY IF EXISTS admin_manage_deletion_requests ON beer_deletion_requests';

        EXECUTE $policy$
            CREATE POLICY insert_own_beer_report ON beer_reports
            FOR INSERT
            WITH CHECK (user_id::text = auth.uid()::text)
        $policy$;
        EXECUTE $policy$
            CREATE POLICY admin_select_beer_reports ON beer_reports
            FOR SELECT
            USING (EXISTS (SELECT 1 FROM beerUsers u WHERE u.id::text = auth.uid()::text AND u.role = 'admin'))
        $policy$;
        EXECUTE $policy$
            CREATE POLICY admin_update_beer_reports ON beer_reports
            FOR UPDATE
            USING (EXISTS (SELECT 1 FROM beerUsers u WHERE u.id::text = auth.uid()::text AND u.role = 'admin'))
        $policy$;
        EXECUTE $policy$
            CREATE POLICY insert_own_deletion_request ON beer_deletion_requests
            FOR INSERT
            WITH CHECK (user_id::text = auth.uid()::text)
        $policy$;
        EXECUTE $policy$
            CREATE POLICY admin_manage_deletion_requests ON beer_deletion_requests
            FOR ALL
            USING (EXISTS (SELECT 1 FROM beerUsers u WHERE u.id::text = auth.uid()::text AND u.role = 'admin'))
        $policy$;
    END IF;
END $$;
