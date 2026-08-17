CREATE TABLE IF NOT EXISTS beer_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    beer_id UUID NOT NULL REFERENCES beers(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES beerUsers(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_by UUID REFERENCES beerUsers(id),
    resolved_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_beer_reports_unique_open
    ON beer_reports (beer_id, user_id)
    WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_beer_reports_beer_id_status_created_at
    ON beer_reports(beer_id, status, created_at);

CREATE INDEX IF NOT EXISTS idx_beer_reports_user_id
    ON beer_reports(user_id);

CREATE INDEX IF NOT EXISTS idx_beer_reports_status
    ON beer_reports(status);
