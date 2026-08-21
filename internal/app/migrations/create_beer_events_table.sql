-- Migration: create_beer_events_table
-- Descrição: Tabela de eventos de cerveja para fallback quando o Redis expirar

CREATE TABLE IF NOT EXISTS beer_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    beer_id UUID NOT NULL REFERENCES beers(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    data JSONB NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_beer_events_beer_id_timestamp
    ON beer_events (beer_id, timestamp DESC);
