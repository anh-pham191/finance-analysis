package domain

import (
	"log/slog"
	"strings"
	"testing"
)

func TestTransactionLogValueContainsOnlyID(t *testing.T) {
	t.Parallel()

	txn := Transaction{
		ID:          "txn-1",
		Description: "private description",
		Merchant:    "private merchant",
		Amount:      MustMoneyFromString("12.30"),
		RawJSON:     []byte(`{"secret":"raw"}`),
	}

	got := txn.LogValue().String()
	if !strings.Contains(got, "txn_id") || !strings.Contains(got, "txn-1") {
		t.Fatalf("LogValue = %q, want txn id only", got)
	}
	for _, leaked := range []string{"private description", "private merchant", "12.30", "secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("LogValue leaked %q: %q", leaked, got)
		}
	}
	if txn.LogValue().Kind() != slog.KindGroup {
		t.Fatalf("LogValue kind = %v, want group", txn.LogValue().Kind())
	}
}
