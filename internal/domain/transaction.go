package domain

import (
	"encoding/json"
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
