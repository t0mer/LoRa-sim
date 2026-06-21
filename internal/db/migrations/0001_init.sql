-- +goose Up
-- gateway: single logical gateway (EUI generated on first run).
CREATE TABLE gateway (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  eui TEXT NOT NULL UNIQUE,                         -- 16 hex chars
  region TEXT NOT NULL DEFAULT 'EU868',
  sub_band INTEGER NOT NULL DEFAULT 2,
  connection_mode TEXT NOT NULL DEFAULT 'cups',     -- cups | lns
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE tags (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  dev_eui TEXT NOT NULL UNIQUE,
  join_eui TEXT NOT NULL,
  app_key TEXT NOT NULL,                            -- stored protected; never returned raw
  class TEXT NOT NULL DEFAULT 'A',                  -- A | B | C
  region TEXT NOT NULL,
  sub_band INTEGER NOT NULL DEFAULT 2,
  default_dr INTEGER NOT NULL,
  fport INTEGER NOT NULL DEFAULT 10,
  payload_type TEXT NOT NULL DEFAULT 'counter',
  payload_config TEXT,                              -- JSON
  schedule TEXT,                                    -- JSON
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

-- per-device join/session state (1:1 with tags).
CREATE TABLE sessions (
  tag_id INTEGER PRIMARY KEY REFERENCES tags(id) ON DELETE CASCADE,
  dev_addr TEXT,
  nwk_skey TEXT,                                    -- protected
  app_skey TEXT,                                    -- protected
  fcnt_up INTEGER NOT NULL DEFAULT 0,
  fcnt_down INTEGER NOT NULL DEFAULT 0,
  dev_nonce INTEGER NOT NULL DEFAULT 0,             -- monotonic; never reused
  rx_delay INTEGER,
  rx1_dr_offset INTEGER,
  rx2_dr INTEGER,
  rx2_freq INTEGER,
  cflist TEXT,                                      -- JSON
  joined INTEGER NOT NULL DEFAULT 0,
  joined_at DATETIME,
  updated_at DATETIME NOT NULL
);

-- traffic / event log for the live UI + history.
CREATE TABLE events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts DATETIME NOT NULL,
  tag_id INTEGER REFERENCES tags(id) ON DELETE SET NULL,
  direction TEXT NOT NULL,                          -- up | down
  kind TEXT NOT NULL,                               -- join | data | ack | macdown
  freq INTEGER,
  dr INTEGER,
  fcnt INTEGER,
  fport INTEGER,
  rssi REAL,
  snr REAL,
  payload_hex TEXT,
  decoded TEXT,                                     -- JSON
  result TEXT
);
CREATE INDEX idx_events_ts ON events(ts);
CREATE INDEX idx_events_tag ON events(tag_id);

-- +goose Down
DROP INDEX idx_events_tag;
DROP INDEX idx_events_ts;
DROP TABLE events;
DROP TABLE sessions;
DROP TABLE tags;
DROP TABLE gateway;
