package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Session is the per-device join/session state (1:1 with a tag). Session keys
// are returned decrypted for internal simulator use.
type Session struct {
	TagID       int64
	DevAddr     string
	NwkSKey     string
	AppSKey     string
	FCntUp      uint32
	FCntDown    uint32
	DevNonce    uint16
	RxDelay     int
	RX1DROffset int
	RX2DR       int
	RX2Freq     int
	CFList      string
	Joined      bool
}

// JoinState carries the result of a successful OTAA join to persist. Keys are
// plaintext on input and encrypted at rest by SaveJoinResult.
type JoinState struct {
	DevAddr     string
	NwkSKey     string
	AppSKey     string
	RxDelay     int
	RX1DROffset int
	RX2DR       int
	RX2Freq     int
	CFList      string
}

// SessionRepo reads and writes session rows.
type SessionRepo struct {
	s *Store
}

func sessionAAD(tagID int64, column string) []byte {
	return []byte(fmt.Sprintf("session|%d|%s", tagID, column))
}

// Get returns the session for a tag, or ErrNotFound.
func (r *SessionRepo) Get(ctx context.Context, tagID int64) (*Session, error) {
	var (
		s                        Session
		devAddr                  sql.NullString
		nwkSKey, appSKey, cflist sql.NullString
		rxDelay, rx1Off          sql.NullInt64
		rx2DR, rx2Freq           sql.NullInt64
		joined                   int
	)
	err := r.s.db.QueryRowContext(ctx,
		`SELECT tag_id, dev_addr, nwk_skey, app_skey, fcnt_up, fcnt_down, dev_nonce,
		        rx_delay, rx1_dr_offset, rx2_dr, rx2_freq, cflist, joined
		   FROM sessions WHERE tag_id = ?`, tagID,
	).Scan(&s.TagID, &devAddr, &nwkSKey, &appSKey, &s.FCntUp, &s.FCntDown, &s.DevNonce,
		&rxDelay, &rx1Off, &rx2DR, &rx2Freq, &cflist, &joined)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}

	s.DevAddr = devAddr.String
	s.CFList = cflist.String
	s.RxDelay = int(rxDelay.Int64)
	s.RX1DROffset = int(rx1Off.Int64)
	s.RX2DR = int(rx2DR.Int64)
	s.RX2Freq = int(rx2Freq.Int64)
	s.Joined = joined != 0

	if nwkSKey.Valid {
		if s.NwkSKey, err = r.s.cipher.Decrypt(nwkSKey.String, sessionAAD(tagID, "nwk_skey")); err != nil {
			return nil, fmt.Errorf("decrypting nwk_skey: %w", err)
		}
	}
	if appSKey.Valid {
		if s.AppSKey, err = r.s.cipher.Decrypt(appSKey.String, sessionAAD(tagID, "app_skey")); err != nil {
			return nil, fmt.Errorf("decrypting app_skey: %w", err)
		}
	}
	return &s, nil
}

// NextDevNonce atomically increments and persists the device nonce, returning
// the new value. DevNonce is monotonic and never reused (1.0.3 requirement).
func (r *SessionRepo) NextDevNonce(ctx context.Context, tagID int64) (uint16, error) {
	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()

	var next uint16
	err := r.s.db.QueryRowContext(ctx,
		`UPDATE sessions SET dev_nonce = dev_nonce + 1, updated_at = ?
		   WHERE tag_id = ? RETURNING dev_nonce`, r.s.clock(), tagID,
	).Scan(&next)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("incrementing dev_nonce: %w", err)
	}
	return next, nil
}

// SaveJoinResult persists session state from a successful join: encrypted keys,
// RX parameters, joined flag, and reset frame counters.
func (r *SessionRepo) SaveJoinResult(ctx context.Context, tagID int64, js JoinState) error {
	encNwk, err := r.s.cipher.Encrypt(js.NwkSKey, sessionAAD(tagID, "nwk_skey"))
	if err != nil {
		return fmt.Errorf("encrypting nwk_skey: %w", err)
	}
	encApp, err := r.s.cipher.Encrypt(js.AppSKey, sessionAAD(tagID, "app_skey"))
	if err != nil {
		return fmt.Errorf("encrypting app_skey: %w", err)
	}
	now := r.s.clock()

	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()
	res, err := r.s.db.ExecContext(ctx,
		`UPDATE sessions
		    SET dev_addr = ?, nwk_skey = ?, app_skey = ?, rx_delay = ?, rx1_dr_offset = ?,
		        rx2_dr = ?, rx2_freq = ?, cflist = ?, joined = 1, joined_at = ?,
		        fcnt_up = 0, fcnt_down = 0, updated_at = ?
		  WHERE tag_id = ?`,
		js.DevAddr, encNwk, encApp, js.RxDelay, js.RX1DROffset, js.RX2DR, js.RX2Freq,
		nullable(js.CFList), now, now, tagID)
	if err != nil {
		return fmt.Errorf("saving join result: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TakeFCntUp returns the current uplink frame counter to use and atomically
// increments the persisted value for the next uplink.
func (r *SessionRepo) TakeFCntUp(ctx context.Context, tagID int64) (uint32, error) {
	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()

	var used uint32
	err := r.s.db.QueryRowContext(ctx,
		`UPDATE sessions SET fcnt_up = fcnt_up + 1, updated_at = ?
		   WHERE tag_id = ? RETURNING fcnt_up - 1`, r.s.clock(), tagID,
	).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("taking fcnt_up: %w", err)
	}
	return used, nil
}

// SetFCntDown persists the highest seen downlink frame counter (for dedup).
func (r *SessionRepo) SetFCntDown(ctx context.Context, tagID int64, fcnt uint32) error {
	r.s.wmu.Lock()
	defer r.s.wmu.Unlock()
	res, err := r.s.db.ExecContext(ctx,
		`UPDATE sessions SET fcnt_down = ?, updated_at = ? WHERE tag_id = ?`,
		fcnt, r.s.clock(), tagID)
	if err != nil {
		return fmt.Errorf("setting fcnt_down: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
