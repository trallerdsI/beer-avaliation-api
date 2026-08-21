package events

import (
	"context"
	"database/sql"
	"time"
)

// PostgresStore implements Store using PostgreSQL as the source of truth.
type PostgresStore struct {
	db interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}
}

// NewPostgresStore creates a new PostgreSQL-backed event store.
func NewPostgresStore(db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) *PostgresStore {
	return &PostgresStore{db: db}
}

// ListSince returns events since a timestamp from PostgreSQL.
func (p *PostgresStore) ListSince(ctx context.Context, beerID string, since time.Time) ([]Event, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT type, data, timestamp
		FROM beer_events
		WHERE beer_id = $1 AND timestamp > $2
		ORDER BY timestamp ASC
	`, beerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var ev Event
		var ts time.Time
		if err := rows.Scan(&ev.Type, &ev.Data, &ts); err != nil {
			return nil, err
		}
		ev.Timestamp = ts
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// LatestEvent returns the most recent event from PostgreSQL.
func (p *PostgresStore) LatestEvent(ctx context.Context, beerID string) (float64, string, error) {
	var ts time.Time
	var data []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT timestamp, data
		FROM beer_events
		WHERE beer_id = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`, beerID).Scan(&ts, &data)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", nil
		}
		return 0, "", err
	}
	return float64(ts.UnixNano()), string(data), nil
}

// Publish is a no-op for PostgresStore; use the repository directly.
func (p *PostgresStore) Publish(ctx context.Context, beerID string, ev Event) error {
	return nil
}
