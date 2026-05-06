package domain

import "time"

type SyncState struct {
	AccountID    string
	LastSyncedAt *time.Time
	LastCursor   *string
}
