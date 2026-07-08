CREATE INDEX IF NOT EXISTS idx_beers_name ON beers (name);
CREATE INDEX IF NOT EXISTS idx_beers_style ON beers (style);
CREATE INDEX IF NOT EXISTS idx_comments_beer_id ON comments (beer_id);
CREATE INDEX IF NOT EXISTS idx_comments_user_id ON comments (user_id);
