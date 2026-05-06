package ports

import (
	"context"
	"encoding/json"
	"time"
)

type RawAccount struct {
	ID       string
	Name     string
	Bank     string
	Type     string
	Currency string
}

type RawTxn struct {
	ID            string
	AccountID     string
	PostedAt      time.Time
	Amount        string
	Direction     string
	Description   string
	Merchant      string
	AkahuCategory string
	RawJSON       json.RawMessage
}

type AkahuClient interface {
	ListAccounts(ctx context.Context) ([]RawAccount, error)
	FetchTransactions(ctx context.Context, accountID string, since time.Time) ([]RawTxn, error)
}
