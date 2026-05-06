package domain

import (
	"encoding/json"
	"log/slog"
	"time"
)

type Transaction struct {
	ID            string
	AccountID     string
	PostedAt      time.Time
	Amount        Money
	Direction     Direction
	Description   string
	Merchant      string
	AkahuCategory string
	RawJSON       json.RawMessage
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (t Transaction) LogValue() slog.Value {
	return slog.GroupValue(slog.String("txn_id", t.ID))
}
