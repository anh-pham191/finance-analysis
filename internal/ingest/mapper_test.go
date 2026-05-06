package ingest

import (
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestMapAccountDefaultsCurrencyToNZD(t *testing.T) {
	t.Parallel()

	raw := ports.RawAccount{
		ID:   "acc-1",
		Name: "Everyday",
		Bank: "ANZ",
		Type: "CHECKING",
	}
	account := mapAccount(raw)

	if account.ID != raw.ID {
		t.Fatalf("ID = %q, want %q", account.ID, raw.ID)
	}
	if account.Name != raw.Name {
		t.Fatalf("Name = %q, want %q", account.Name, raw.Name)
	}
	if account.Bank != raw.Bank {
		t.Fatalf("Bank = %q, want %q", account.Bank, raw.Bank)
	}
	if account.Type != raw.Type {
		t.Fatalf("Type = %q, want %q", account.Type, raw.Type)
	}
	if account.Currency != "NZD" {
		t.Fatalf("Currency = %q, want NZD", account.Currency)
	}
}

func TestMapTxnMapsAmountAndDirection(t *testing.T) {
	t.Parallel()

	postedAt := time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC)
	rawJSON := []byte(`{"id":"txn-1"}`)
	raw := ports.RawTxn{
		ID:            "txn-1",
		AccountID:     "acc-1",
		PostedAt:      postedAt,
		Amount:        "12.34",
		Direction:     "DEBIT",
		Description:   "Coffee",
		Merchant:      "Cafe",
		AkahuCategory: "eating_out",
		RawJSON:       rawJSON,
	}
	txn, err := mapTxn(raw)
	if err != nil {
		t.Fatalf("mapTxn returned error: %v", err)
	}

	if txn.ID != raw.ID {
		t.Fatalf("ID = %q, want %q", txn.ID, raw.ID)
	}
	if txn.AccountID != raw.AccountID {
		t.Fatalf("AccountID = %q, want %q", txn.AccountID, raw.AccountID)
	}
	if !txn.PostedAt.Equal(raw.PostedAt) {
		t.Fatalf("PostedAt = %s, want %s", txn.PostedAt, raw.PostedAt)
	}
	if txn.Description != raw.Description {
		t.Fatalf("Description = %q, want %q", txn.Description, raw.Description)
	}
	if txn.Merchant != raw.Merchant {
		t.Fatalf("Merchant = %q, want %q", txn.Merchant, raw.Merchant)
	}
	if txn.AkahuCategory != raw.AkahuCategory {
		t.Fatalf("AkahuCategory = %q, want %q", txn.AkahuCategory, raw.AkahuCategory)
	}
	if string(txn.RawJSON) != string(raw.RawJSON) {
		t.Fatalf("RawJSON = %s, want %s", txn.RawJSON, raw.RawJSON)
	}
	if txn.Amount.String() != "12.34" {
		t.Fatalf("Amount = %s, want 12.34", txn.Amount.String())
	}
	if txn.Direction != domain.DirectionDebit {
		t.Fatalf("Direction = %q, want %q", txn.Direction, domain.DirectionDebit)
	}
}

func TestMapTxnRejectsUnknownDirectionWithoutPII(t *testing.T) {
	t.Parallel()

	const (
		txnID       = "txn-secret-direction"
		description = "private desc"
		merchant    = "private merchant"
		amount      = "12.34"
		rawJSON     = `{"secret":"raw"}`
	)

	_, err := mapTxn(ports.RawTxn{
		ID:          txnID,
		AccountID:   "acc-1",
		PostedAt:    time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC),
		Amount:      amount,
		Direction:   "TRANSFER",
		Description: description,
		Merchant:    merchant,
		RawJSON:     []byte(rawJSON),
	})

	assertMapperErrorRedactsPII(t, err, txnID, description, merchant, amount, rawJSON)
}

func TestMapTxnRejectsInvalidAmountWithoutPII(t *testing.T) {
	t.Parallel()

	const (
		txnID       = "txn-secret-amount"
		description = "private desc"
		merchant    = "private merchant"
		amount      = "not-money"
		rawJSON     = `{"secret":"raw"}`
	)

	_, err := mapTxn(ports.RawTxn{
		ID:          txnID,
		AccountID:   "acc-1",
		PostedAt:    time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC),
		Amount:      amount,
		Direction:   "DEBIT",
		Description: description,
		Merchant:    merchant,
		RawJSON:     []byte(rawJSON),
	})

	assertMapperErrorRedactsPII(t, err, txnID, description, merchant, amount, rawJSON)
}

func assertMapperErrorRedactsPII(t *testing.T, err error, txnID string, sensitive ...string) {
	t.Helper()

	if err == nil {
		t.Fatal("mapTxn returned nil error, want error")
	}

	message := err.Error()
	if !strings.Contains(message, txnID) {
		t.Fatalf("error = %q, want txn ID %q", message, txnID)
	}

	for _, value := range sensitive {
		if strings.Contains(message, value) {
			t.Fatalf("error = %q, leaked sensitive value %q", message, value)
		}
	}
}
