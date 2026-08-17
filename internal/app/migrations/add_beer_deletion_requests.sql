CREATE TABLE IF NOT EXISTS beer_deletion_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    beer_id UUID NOT NULL REFERENCES beers(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES beerUsers(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    details TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_by UUID REFERENCES beerUsers(id),
    reviewed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_beer_deletion_requests_unique_pending
    ON beer_deletion_requests (beer_id, user_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_beer_deletion_requests_beer_id_status_created_at
    ON beer_deletion_requests(beer_id, status, created_at);

CREATE INDEX IF NOT EXISTS idx_beer_deletion_requests_user_id
    ON beer_deletion_requests(user_id);

CREATE INDEX IF NOT EXISTS idx_beer_deletion_requests_status
    ON beer_deletion_requests(status);
