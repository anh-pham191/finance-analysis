package akahu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestClientListAccountsSendsAuthHeadersAndMapsAccountFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts" {
			t.Fatalf("path = %q, want /accounts", r.URL.Path)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer user_token_test"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("X-Akahu-ID"), "app_token_test"; got != want {
			t.Fatalf("X-Akahu-ID = %q, want %q", got, want)
		}
		_, _ = fmt.Fprint(w, `{"items":[{"_id":"acc_1","name":"Everyday","connection":{"name":"ANZ"},"type":"CHECKING","balance":{"currency":"NZD"}}],"cursor":{"next":""}}`)
	}))
	defer server.Close()

	client := newTestClient(server)

	accounts, err := client.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len(accounts) = %d, want 1", len(accounts))
	}
	want := ports.RawAccount{
		ID:       "acc_1",
		Name:     "Everyday",
		Bank:     "ANZ",
		Type:     "CHECKING",
		Currency: "NZD",
	}
	if accounts[0] != want {
		t.Fatalf("account = %#v, want %#v", accounts[0], want)
	}
}

func TestClientFetchTransactionsIncludesAccountIDAndSinceAndMapsStringAmount(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/accounts/acc_1/transactions"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("start"), since.Format(time.RFC3339); got != want {
			t.Fatalf("start = %q, want %q", got, want)
		}
		_, _ = fmt.Fprint(w, `{"items":[{"_id":"txn_1","_account":"acc_1","date":"2026-05-01T00:00:00Z","amount":"-12.30","type":"DEBIT","description":"desc","merchant":{"name":"merchant"},"category":{"name":"FOOD"}}],"cursor":{"next":""}}`)
	}))
	defer server.Close()

	client := newTestClient(server)

	txns, err := client.FetchTransactions(context.Background(), "acc_1", since)
	if err != nil {
		t.Fatalf("FetchTransactions returned error: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("len(txns) = %d, want 1", len(txns))
	}
	wantPostedAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	got := txns[0]
	if got.ID != "txn_1" ||
		got.AccountID != "acc_1" ||
		!got.PostedAt.Equal(wantPostedAt) ||
		got.Amount != "-12.30" ||
		got.Direction != "DEBIT" ||
		got.Description != "desc" ||
		got.Merchant != "merchant" ||
		got.AkahuCategory != "FOOD" {
		t.Fatalf("txn = %#v", got)
	}
	if !strings.Contains(string(got.RawJSON), `"amount":"-12.30"`) {
		t.Fatalf("RawJSON = %s, want original transaction JSON", got.RawJSON)
	}
}

func TestClientFetchTransactionsMapsNumericAmount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"items":[{"_id":"txn_1","_account":"acc_1","date":"2026-05-01T00:00:00Z","amount":-12.3,"type":"DEBIT"}],"cursor":{"next":""}}`)
	}))
	defer server.Close()

	client := newTestClient(server)

	txns, err := client.FetchTransactions(context.Background(), "acc_1", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchTransactions returned error: %v", err)
	}
	if got, want := txns[0].Amount, "-12.3"; got != want {
		t.Fatalf("Amount = %q, want %q", got, want)
	}
}

func TestClientPaginationLoopsUntilCursorNextEmpty(t *testing.T) {
	t.Parallel()

	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.RequestURI() {
		case "/accounts/acc_1/transactions?start=2026-05-01T00%3A00%3A00Z":
			_, _ = fmt.Fprint(w, `{"items":[{"_id":"txn_1","_account":"acc_1","date":"2026-05-01T00:00:00Z","amount":"1.00","type":"CREDIT"}],"cursor":{"next":"next_page"}}`)
		case "/accounts/acc_1/transactions?cursor=next_page&start=2026-05-01T00%3A00%3A00Z":
			_, _ = fmt.Fprint(w, `{"items":[{"_id":"txn_2","_account":"acc_1","date":"2026-05-02T00:00:00Z","amount":"2.00","type":"CREDIT"}],"cursor":{"next":""}}`)
		default:
			t.Fatalf("unexpected request URI %q", r.URL.RequestURI())
		}
	}))
	defer server.Close()

	client := newTestClient(server)

	txns, err := client.FetchTransactions(context.Background(), "acc_1", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchTransactions returned error: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("len(txns) = %d, want 2", len(txns))
	}
	if got, want := txns[0].ID, "txn_1"; got != want {
		t.Fatalf("first txn id = %q, want %q", got, want)
	}
	if got, want := txns[1].ID, "txn_2"; got != want {
		t.Fatalf("second txn id = %q, want %q", got, want)
	}
	if len(paths) != 2 {
		t.Fatalf("request count = %d, want 2", len(paths))
	}
}

func TestClientNon2xxErrorRedactsTokenShapedResponseContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failed: Authorization: Bearer abc123def456", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newTestClient(server)

	_, err := client.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("ListAccounts returned nil error, want error")
	}
	if strings.Contains(err.Error(), "abc123def456") {
		t.Fatalf("error leaked token: %v", err)
	}
	if !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("error = %q, want status", err.Error())
	}
}

func TestClientMalformedTransactionResponseIncludesTxnIDAndOmitsRawBody(t *testing.T) {
	t.Parallel()

	const rawBodyContent = "raw_body_marker"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"items":[{"_id":"txn_bad","_account":"acc_1","date":"not-a-date","amount":"-12.30","description":%q}],"cursor":{"next":""}}`, rawBodyContent)
	}))
	defer server.Close()

	client := newTestClient(server)

	_, err := client.FetchTransactions(context.Background(), "acc_1", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("FetchTransactions returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "txn_bad") {
		t.Fatalf("error = %q, want txn id", err.Error())
	}
	if strings.Contains(err.Error(), rawBodyContent) {
		t.Fatalf("error leaked raw body content: %v", err)
	}
}

func TestClientMalformedTransactionAmountIncludesTxnIDAndOmitsRawBody(t *testing.T) {
	t.Parallel()

	const rawBodySecret = "raw_body_secret_marker"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"items":[{"_id":"txn_bad","_account":"acc_1","date":"2026-05-01T00:00:00Z","amount":{"secret":%q},"type":"DEBIT"}],"cursor":{"next":""}}`, rawBodySecret)
	}))
	defer server.Close()

	client := newTestClient(server)

	_, err := client.FetchTransactions(context.Background(), "acc_1", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("FetchTransactions returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "txn_bad") {
		t.Fatalf("error = %q, want txn id", err.Error())
	}
	if strings.Contains(err.Error(), rawBodySecret) {
		t.Fatalf("error leaked raw body content: %v", err)
	}
}

func TestNewClientUsesM2RetryDefaults(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{BaseURL: "https://api.akahu.test"})

	if client.retry.maxRetries != 3 {
		t.Fatalf("maxRetries = %d, want 3", client.retry.maxRetries)
	}
	if client.retry.baseDelay != time.Second {
		t.Fatalf("baseDelay = %v, want 1s", client.retry.baseDelay)
	}
	for i := 0; i < 20; i++ {
		jitter := client.retry.jitter(time.Second)
		if jitter < -250*time.Millisecond || jitter > 250*time.Millisecond {
			t.Fatalf("jitter = %v, want within ±250ms", jitter)
		}
	}
}

func newTestClient(server *httptest.Server) ports.AkahuClient {
	client := NewClient(Config{
		AppToken:   "app_token_test",
		UserToken:  "user_token_test",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	return client
}
