package store

import "time"

// timeLayout is the textual form used for DATETIME columns (UTC, RFC3339).
const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func nowUTC() string {
	return time.Now().UTC().Format(timeLayout)
}
