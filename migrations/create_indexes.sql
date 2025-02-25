-- Create indexes for faster searching on beers
CREATE INDEX IF NOT EXISTS idx_beer_name ON beers(name);
CREATE INDEX IF NOT EXISTS idx_beer_style ON beers(style);