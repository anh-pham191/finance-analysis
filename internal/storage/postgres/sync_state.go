package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type SyncStateRepo struct {
	db *sql.DB
}

func NewSyncStateRepo(db *sql.DB) *SyncStateRepo {
	return &SyncStateRepo{db: db}
}

func (r *SyncStateRepo) Upsert(ctx context.Context, userID domain.UserID, state domain.SyncState) error {
	return withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sync_state (user_id, account_id, last_synced_at, last_cursor)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, account_id) DO UPDATE SET
				last_synced_at = EXCLUDED.last_synced_at,
				last_cursor = EXCLUDED.last_cursor
		`, userID.Int64(), state.AccountID, state.LastSyncedAt, state.LastCursor)
		if err != nil {
			return fmt.Errorf("upsert sync state: %w", err)
		}
		return nil
	})
}

func (r *SyncStateRepo) Get(ctx context.Context, userID domain.UserID, accountID string) (domain.SyncState, error) {
	var state domain.SyncState
	var lastSynced sql.NullTime
	var cursor sql.NullString
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT account_id, last_synced_at, last_cursor
			FROM sync_state
			WHERE user_id = $1 AND account_id = $2
		`, userID.Int64(), accountID).Scan(&state.AccountID, &lastSynced, &cursor)
	})
	if err != nil {
		return domain.SyncState{}, err
	}
	if lastSynced.Valid {
		state.LastSyncedAt = &lastSynced.Time
	}
	if cursor.Valid {
		state.LastCursor = &cursor.String
	}
	return state, nil
}
