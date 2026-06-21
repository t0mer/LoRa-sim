package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Event is a single traffic/event log row for the live UI and history.
type Event struct {
	ID         int64    `json:"id"`
	TS         string   `json:"ts"`
	TagID      *int64   `json:"tag_id,omitempty"`
	Direction  string   `json:"direction"` // up | down
	Kind       string   `json:"kind"`      // join | data | ack | macdown
	Freq       *int64   `json:"freq,omitempty"`
	DR         *int64   `json:"dr,omitempty"`
	FCnt       *int64   `json:"fcnt,omitempty"`
	FPort      *int64   `json:"fport,omitempty"`
	RSSI       *float64 `json:"rssi,omitempty"`
	SNR        *float64 `json:"snr,omitempty"`
	PayloadHex string   `json:"payload_hex,omitempty"`
	Decoded    string   `json:"decoded,omitempty"` // JSON
	Result     string   `json:"result,omitempty"`
}

// EventFilter narrows an event listing. Zero values mean "no filter".
type EventFilter struct {
	TagID     *int64
	Direction string
	BeforeID  int64 // keyset pagination: return events with id < BeforeID
	Limit     int   // default 100, capped at 1000
}

// EventRepo appends and queries traffic events.
type EventRepo struct {
	s *Store
}

// Append inserts an event and returns its id. The timestamp is set to now.
func (r *EventRepo) Append(ctx context.Context, e Event) (int64, error) {
	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()
	res, err := r.s.db.ExecContext(ctx,
		`INSERT INTO events
		   (ts, tag_id, direction, kind, freq, dr, fcnt, fport, rssi, snr, payload_hex, decoded, result)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.s.clock(), e.TagID, e.Direction, e.Kind, e.Freq, e.DR, e.FCnt, e.FPort,
		e.RSSI, e.SNR, nullable(e.PayloadHex), nullable(e.Decoded), nullable(e.Result))
	if err != nil {
		return 0, fmt.Errorf("appending event: %w", err)
	}
	return res.LastInsertId()
}

// List returns events newest-first, applying the filter.
func (r *EventRepo) List(ctx context.Context, f EventFilter) ([]Event, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var where []string
	var args []any
	if f.TagID != nil {
		where = append(where, "tag_id = ?")
		args = append(args, *f.TagID)
	}
	if f.Direction != "" {
		where = append(where, "direction = ?")
		args = append(args, f.Direction)
	}
	if f.BeforeID > 0 {
		where = append(where, "id < ?")
		args = append(args, f.BeforeID)
	}
	query := `SELECT id, ts, tag_id, direction, kind, freq, dr, fcnt, fport, rssi, snr,
	                 payload_hex, decoded, result FROM events`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Prune keeps only the newest keep events, deleting older ones.
func (r *EventRepo) Prune(ctx context.Context, keep int) (int64, error) {
	if keep < 0 {
		keep = 0
	}
	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()
	res, err := r.s.db.ExecContext(ctx,
		`DELETE FROM events WHERE id NOT IN (SELECT id FROM events ORDER BY id DESC LIMIT ?)`,
		keep)
	if err != nil {
		return 0, fmt.Errorf("pruning events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func scanEvent(sc scanner) (Event, error) {
	var (
		e                         Event
		tagID, freq, dr, fcnt, fp sql.NullInt64
		rssi, snr                 sql.NullFloat64
		payload, decoded, result  sql.NullString
	)
	if err := sc.Scan(&e.ID, &e.TS, &tagID, &e.Direction, &e.Kind, &freq, &dr, &fcnt, &fp,
		&rssi, &snr, &payload, &decoded, &result); err != nil {
		return e, fmt.Errorf("scanning event: %w", err)
	}
	e.TagID = nullInt64Ptr(tagID)
	e.Freq = nullInt64Ptr(freq)
	e.DR = nullInt64Ptr(dr)
	e.FCnt = nullInt64Ptr(fcnt)
	e.FPort = nullInt64Ptr(fp)
	if rssi.Valid {
		e.RSSI = &rssi.Float64
	}
	if snr.Valid {
		e.SNR = &snr.Float64
	}
	e.PayloadHex = payload.String
	e.Decoded = decoded.String
	e.Result = result.String
	return e, nil
}

func nullInt64Ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
