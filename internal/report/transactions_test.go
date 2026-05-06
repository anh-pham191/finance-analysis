package report

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestTransactionsMapsFilterToRepo(t *testing.T) {
	t.Parallel()

	userID := domain.UserID(42)
	categoryID := int64(99)
	direction := domain.DirectionDebit
	min := domain.MustMoneyFromString("10.00")
	max := domain.MustMoneyFromString("200.00")
	period := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	repo := &fakeTxnQueryRepo{}

	_, err := Transactions(context.Background(), userID, TransactionsDeps{Txns: repo}, TxnFilter{
		Period:     period,
		CategoryID: &categoryID,
		Merchant:   "Alpha Market",
		AccountID:  "acc_123",
		Direction:  &direction,
		Min:        &min,
		Max:        &max,
		Sort:       "amount",
		Offset:     -10,
	})
	if err != nil {
		t.Fatalf("Transactions returned error: %v", err)
	}

	if repo.userID != userID {
		t.Fatalf("userID = %v, want %v", repo.userID, userID)
	}
	got := repo.filter
	if got.Range != period {
		t.Fatalf("Range = %#v, want %#v", got.Range, period)
	}
	if got.CategoryID == nil || *got.CategoryID != categoryID {
		t.Fatalf("CategoryID = %#v, want %d", got.CategoryID, categoryID)
	}
	if got.Merchant != "Alpha Market" {
		t.Fatalf("Merchant = %q, want Alpha Market", got.Merchant)
	}
	if got.AccountID != "acc_123" {
		t.Fatalf("AccountID = %q, want acc_123", got.AccountID)
	}
	if got.Direction == nil || *got.Direction != direction {
		t.Fatalf("Direction = %#v, want %s", got.Direction, direction)
	}
	if got.Min == nil || got.Min.String() != "10.00" {
		t.Fatalf("Min = %#v, want 10.00", got.Min)
	}
	if got.Max == nil || got.Max.String() != "200.00" {
		t.Fatalf("Max = %#v, want 200.00", got.Max)
	}
	if got.Sort != "amount" {
		t.Fatalf("Sort = %q, want amount", got.Sort)
	}
	if got.Limit != 100 {
		t.Fatalf("Limit = %d, want 100", got.Limit)
	}
	if got.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", got.Offset)
	}
}

func TestTransactionsPassesLimitOffsetAndSortThrough(t *testing.T) {
	t.Parallel()

	repo := &fakeTxnQueryRepo{}
	_, err := Transactions(context.Background(), domain.UserID(42), TransactionsDeps{Txns: repo}, TxnFilter{
		Sort:   "merchant",
		Limit:  25,
		Offset: 50,
	})
	if err != nil {
		t.Fatalf("Transactions returned error: %v", err)
	}

	if repo.filter.Sort != "merchant" {
		t.Fatalf("Sort = %q, want merchant", repo.filter.Sort)
	}
	if repo.filter.Limit != 25 {
		t.Fatalf("Limit = %d, want 25", repo.filter.Limit)
	}
	if repo.filter.Offset != 50 {
		t.Fatalf("Offset = %d, want 50", repo.filter.Offset)
	}
}

func TestTransactionsMapsRows(t *testing.T) {
	t.Parallel()

	postedAt := time.Date(2026, 4, 15, 12, 30, 0, 0, time.UTC)
	repo := &fakeTxnQueryRepo{rows: []ports.TxnReportRow{{
		Transaction: domain.Transaction{
			ID:          "txn_123",
			PostedAt:    postedAt,
			AccountID:   "acc_123",
			Direction:   domain.DirectionCredit,
			Amount:      domain.MustMoneyFromString("12.34"),
			Merchant:    "Alpha Market",
			Description: "Synthetic description",
		},
		Category: "Groceries",
	}}}

	got, err := Transactions(context.Background(), domain.UserID(42), TransactionsDeps{Txns: repo}, TxnFilter{})
	if err != nil {
		t.Fatalf("Transactions returned error: %v", err)
	}

	want := []TxnRow{{
		TxnID:       "txn_123",
		PostedAt:    postedAt,
		AccountID:   "acc_123",
		Category:    "Groceries",
		Direction:   "CREDIT",
		Amount:      "12.34",
		Merchant:    "Alpha Market",
		Description: "Synthetic description",
	}}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %#v, want %#v", got, want)
	}
}

func TestTransactionsDTOJSONTagsUseSnakeCase(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(TxnRow{
		TxnID:     "txn_123",
		PostedAt:  time.Date(2026, 4, 15, 12, 30, 0, 0, time.UTC),
		AccountID: "acc_123",
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	body := string(payload)

	for _, want := range []string{`"txn_id":"txn_123"`, `"posted_at":"2026-04-15T12:30:00Z"`, `"account_id":"acc_123"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("JSON %s does not contain %s", body, want)
		}
	}
	for _, notWant := range []string{"TxnID", "PostedAt", "AccountID"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("JSON %s contains non-snake-case key %s", body, notWant)
		}
	}
}

func TestTransactionsValidatesDependencies(t *testing.T) {
	t.Parallel()

	_, err := Transactions(context.Background(), domain.UserID(42), TransactionsDeps{}, TxnFilter{})
	if err == nil {
		t.Fatal("Transactions returned nil error, want dependency validation error")
	}
	if !strings.Contains(err.Error(), "transactions: missing transaction query repo") {
		t.Fatalf("error = %q, want clear missing dependency error", err)
	}
}

type fakeTxnQueryRepo struct {
	rows   []ports.TxnReportRow
	err    error
	userID domain.UserID
	filter ports.TxnFilter
}

func (r *fakeTxnQueryRepo) ListFiltered(_ context.Context, userID domain.UserID, filter ports.TxnFilter) ([]ports.TxnReportRow, error) {
	r.userID = userID
	r.filter = filter
	if r.err != nil {
		return nil, r.err
	}
	return slices.Clone(r.rows), nil
}

var _ ports.TxnQueryRepo = (*fakeTxnQueryRepo)(nil)
