package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// PostgresStore implements Store using PostgreSQL as the source of truth.
type PostgresStore struct {
	db interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}
}

// NewPostgresStore creates a new PostgreSQL-backed event store.
func NewPostgresStore(db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) *PostgresStore {
	return &PostgresStore{db: db}
}

// ListSince returns events since a timestamp from PostgreSQL.
func (p *PostgresStore) ListSince(ctx context.Context, beerID string, since time.Time) ([]Event, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, beer_id, type, data, timestamp
		FROM beer_events
		WHERE beer_id = $1 AND timestamp > $2
		ORDER BY timestamp ASC, id ASC
	`, beerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var ev Event
		var ts time.Time
		if err := rows.Scan(&ev.ID, &ev.BeerID, &ev.Type, &ev.Data, &ts); err != nil {
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
	err := scanRow(ctx, p.db, `
		SELECT timestamp, data
		FROM beer_events
		WHERE beer_id = $1
		ORDER BY timestamp DESC
		LIMIT 1`, []any{beerID}, &ts, &data)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", nil
		}
		return 0, "", err
	}
	return float64(ts.UnixNano()), string(data), nil
}

func (p *PostgresStore) Publish(ctx context.Context, beerID string, ev Event) error {
	data, err := json.Marshal(ev.Data)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `
		INSERT INTO beer_events (id, beer_id, type, data, timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`, ev.ID, beerID, ev.Type, data, ev.Timestamp)
	return err
}

func scanRow(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, query string, args []any, dest ...any) error {
	if scanner, ok := db.(interface {
		ScanRowContext(context.Context, string, []any, ...any) error
	}); ok {
		return scanner.ScanRowContext(ctx, query, args, dest...)
	}
	return db.QueryRowContext(ctx, query, args...).Scan(dest...)
}
