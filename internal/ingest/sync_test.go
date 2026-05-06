package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestSyncFirstRunFetchesAccountsAndTransactions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(42)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	account := ports.RawAccount{
		ID:       "acc-1",
		Name:     "Everyday",
		Bank:     "ANZ",
		Type:     "CHECKING",
		Currency: "NZD",
	}
	rawTxn := validRawTxn("txn-1", account.ID)
	client := &fakeAkahuClient{
		accounts: []ports.RawAccount{account},
		txns:     map[string][]ports.RawTxn{account.ID: {rawTxn}},
	}
	tokens := &fakeTokenStore{app: "app-token", user: "user-token"}
	accounts := &fakeAccountRepo{}
	txns := &fakeTxnRepo{}
	states := newFakeSyncStateRepo()

	var gotAppToken, gotUserToken string
	deps := Deps{
		Accounts:   accounts,
		Txns:       txns,
		SyncStates: states,
		Tokens:     tokens,
		NewAkahuClient: func(appToken, userToken string) ports.AkahuClient {
			gotAppToken = appToken
			gotUserToken = userToken
			return client
		},
		Clock: fakeClock{now: now},
	}

	if err := Sync(ctx, userID, deps, Options{}); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	if tokens.calledWith != userID {
		t.Fatalf("token store called with userID %v, want %v", tokens.calledWith, userID)
	}
	if gotAppToken != tokens.app {
		t.Fatalf("client factory app token = %q, want %q", gotAppToken, tokens.app)
	}
	if gotUserToken != tokens.user {
		t.Fatalf("client factory user token = %q, want %q", gotUserToken, tokens.user)
	}
	if client.listAccountsCalls != 1 {
		t.Fatalf("ListAccounts called %d times, want 1", client.listAccountsCalls)
	}
	if len(accounts.upserts) != 1 {
		t.Fatalf("account upserts = %d, want 1", len(accounts.upserts))
	}
	if accounts.upserts[0].userID != userID {
		t.Fatalf("account upsert userID = %v, want %v", accounts.upserts[0].userID, userID)
	}
	if accounts.upserts[0].account.ID != account.ID {
		t.Fatalf("account upsert ID = %q, want %q", accounts.upserts[0].account.ID, account.ID)
	}
	wantSince := now.AddDate(0, 0, -30)
	assertFetchSince(t, client, account.ID, wantSince)
	if len(txns.upserts) != 1 {
		t.Fatalf("txn upserts = %d, want 1", len(txns.upserts))
	}
	if txns.upserts[0].userID != userID {
		t.Fatalf("txn upsert userID = %v, want %v", txns.upserts[0].userID, userID)
	}
	if txns.upserts[0].txn.ID != rawTxn.ID {
		t.Fatalf("txn upsert ID = %q, want %q", txns.upserts[0].txn.ID, rawTxn.ID)
	}
	state, ok := states.states[account.ID]
	if !ok {
		t.Fatalf("sync state for account %q was not upserted", account.ID)
	}
	if state.LastSyncedAt == nil || !state.LastSyncedAt.Equal(now) {
		t.Fatalf("LastSyncedAt = %v, want %v", state.LastSyncedAt, now)
	}
	if state.AccountID != account.ID {
		t.Fatalf("sync state AccountID = %q, want %q", state.AccountID, account.ID)
	}
	if len(states.gets) != 1 {
		t.Fatalf("sync state gets = %d, want 1", len(states.gets))
	}
	if states.gets[0].userID != userID {
		t.Fatalf("sync state get userID = %v, want %v", states.gets[0].userID, userID)
	}
	if len(states.upserts) != 1 {
		t.Fatalf("sync state upserts = %d, want 1", len(states.upserts))
	}
	if states.upserts[0].userID != userID {
		t.Fatalf("sync state upsert userID = %v, want %v", states.upserts[0].userID, userID)
	}
}

func TestSyncSecondRunUsesOverlapBeforeLastSyncedAt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(42)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	lastSyncedAt := now.Add(-48 * time.Hour)
	accountID := "acc-1"
	client := &fakeAkahuClient{
		accounts: []ports.RawAccount{{ID: accountID, Name: "Everyday"}},
		txns:     map[string][]ports.RawTxn{accountID: {validRawTxn("txn-1", accountID)}},
	}
	states := newFakeSyncStateRepo()
	states.states[accountID] = domain.SyncState{
		AccountID:    accountID,
		LastSyncedAt: &lastSyncedAt,
	}

	deps := validSyncDeps(now, client, states, &fakeTxnRepo{})

	if err := Sync(ctx, userID, deps, Options{}); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	assertFetchSince(t, client, accountID, lastSyncedAt.Add(-24*time.Hour))
}

func TestSyncFromOverrideUsesExplicitDate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(42)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	lastSyncedAt := now.Add(-48 * time.Hour)
	from := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	accountID := "acc-1"
	client := &fakeAkahuClient{
		accounts: []ports.RawAccount{{ID: accountID, Name: "Everyday"}},
		txns:     map[string][]ports.RawTxn{accountID: {validRawTxn("txn-1", accountID)}},
	}
	states := newFakeSyncStateRepo()
	states.states[accountID] = domain.SyncState{
		AccountID:    accountID,
		LastSyncedAt: &lastSyncedAt,
	}

	deps := validSyncDeps(now, client, states, &fakeTxnRepo{})

	if err := Sync(ctx, userID, deps, Options{From: &from}); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	assertFetchSince(t, client, accountID, from)
}

func TestSyncMappingErrorDoesNotUpsertBadTxn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(42)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	accountID := "acc-1"
	badTxn := validRawTxn("txn-bad", accountID)
	badTxn.Direction = "TRANSFER"
	client := &fakeAkahuClient{
		accounts: []ports.RawAccount{{ID: accountID, Name: "Everyday"}},
		txns:     map[string][]ports.RawTxn{accountID: {badTxn}},
	}
	txns := &fakeTxnRepo{}
	deps := validSyncDeps(now, client, newFakeSyncStateRepo(), txns)

	err := Sync(ctx, userID, deps, Options{})
	if err == nil {
		t.Fatal("Sync returned nil error, want mapping error")
	}
	if len(txns.upserts) != 0 {
		t.Fatalf("txn upserts = %d, want 0", len(txns.upserts))
	}
}

func TestSyncRejectsNilDependencyWithoutSensitiveValues(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	client := &fakeAkahuClient{
		accounts: []ports.RawAccount{{ID: "acc-1", Name: "Everyday"}},
		txns:     map[string][]ports.RawTxn{"acc-1": {validRawTxn("txn-1", "acc-1")}},
	}
	deps := validSyncDeps(now, client, newFakeSyncStateRepo(), &fakeTxnRepo{})
	deps.Accounts = nil

	err := Sync(context.Background(), domain.UserID(42), deps, Options{})
	assertSyncErrorWithoutSensitiveValues(t, err, "missing account repo")
}

func TestSyncRejectsNilClientWithoutSensitiveValues(t *testing.T) {
	t.Parallel()

	const (
		appToken  = "app-token-secret"
		userToken = "user-token-secret"
	)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	deps := validSyncDeps(now, nil, newFakeSyncStateRepo(), &fakeTxnRepo{})
	deps.Tokens = &fakeTokenStore{app: appToken, user: userToken}

	err := Sync(context.Background(), domain.UserID(42), deps, Options{})
	assertSyncErrorWithoutSensitiveValues(t, err, "new Akahu client returned nil")
}

type accountUpsert struct {
	userID  domain.UserID
	account domain.Account
}

type fakeAccountRepo struct {
	upserts []accountUpsert
}

func (r *fakeAccountRepo) Upsert(_ context.Context, userID domain.UserID, account domain.Account) error {
	r.upserts = append(r.upserts, accountUpsert{userID: userID, account: account})
	return nil
}

func (r *fakeAccountRepo) Get(context.Context, domain.UserID, string) (domain.Account, error) {
	return domain.Account{}, errors.New("unexpected account get")
}

type txnUpsert struct {
	userID domain.UserID
	txn    domain.Transaction
}

type fakeTxnRepo struct {
	upserts []txnUpsert
}

func (r *fakeTxnRepo) Upsert(_ context.Context, userID domain.UserID, txn domain.Transaction) error {
	r.upserts = append(r.upserts, txnUpsert{userID: userID, txn: txn})
	return nil
}

func (r *fakeTxnRepo) Get(context.Context, domain.UserID, string) (domain.Transaction, error) {
	return domain.Transaction{}, errors.New("unexpected txn get")
}

type fakeSyncStateRepo struct {
	states  map[string]domain.SyncState
	gets    []syncStateGet
	upserts []syncStateUpsert
}

type syncStateGet struct {
	userID    domain.UserID
	accountID string
}

type syncStateUpsert struct {
	userID domain.UserID
	state  domain.SyncState
}

func newFakeSyncStateRepo() *fakeSyncStateRepo {
	return &fakeSyncStateRepo{states: make(map[string]domain.SyncState)}
}

func (r *fakeSyncStateRepo) Upsert(_ context.Context, userID domain.UserID, state domain.SyncState) error {
	r.upserts = append(r.upserts, syncStateUpsert{userID: userID, state: state})
	r.states[state.AccountID] = state
	return nil
}

func (r *fakeSyncStateRepo) Get(_ context.Context, userID domain.UserID, accountID string) (domain.SyncState, error) {
	r.gets = append(r.gets, syncStateGet{userID: userID, accountID: accountID})
	state, ok := r.states[accountID]
	if !ok {
		return domain.SyncState{}, ports.ErrNotFound
	}
	return state, nil
}

type fakeTokenStore struct {
	app        string
	user       string
	calledWith domain.UserID
}

func (s *fakeTokenStore) AkahuTokens(_ context.Context, userID domain.UserID) (string, string, error) {
	s.calledWith = userID
	return s.app, s.user, nil
}

type fetchCall struct {
	accountID string
	since     time.Time
}

type fakeAkahuClient struct {
	accounts          []ports.RawAccount
	txns              map[string][]ports.RawTxn
	listAccountsCalls int
	fetchCalls        []fetchCall
}

func (c *fakeAkahuClient) ListAccounts(context.Context) ([]ports.RawAccount, error) {
	c.listAccountsCalls++
	return c.accounts, nil
}

func (c *fakeAkahuClient) FetchTransactions(_ context.Context, accountID string, since time.Time) ([]ports.RawTxn, error) {
	c.fetchCalls = append(c.fetchCalls, fetchCall{accountID: accountID, since: since})
	return c.txns[accountID], nil
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

func validSyncDeps(now time.Time, client ports.AkahuClient, states ports.SyncStateRepo, txns ports.TxnRepo) Deps {
	return Deps{
		Accounts:   &fakeAccountRepo{},
		Txns:       txns,
		SyncStates: states,
		Tokens:     &fakeTokenStore{app: "app-token", user: "user-token"},
		NewAkahuClient: func(string, string) ports.AkahuClient {
			return client
		},
		Clock: fakeClock{now: now},
	}
}

func validRawTxn(id, accountID string) ports.RawTxn {
	return ports.RawTxn{
		ID:            id,
		AccountID:     accountID,
		PostedAt:      time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
		Amount:        "12.34",
		Direction:     "DEBIT",
		Description:   "Coffee",
		Merchant:      "Cafe",
		AkahuCategory: "eating_out",
		RawJSON:       []byte(`{"id":"` + id + `"}`),
	}
}

func assertFetchSince(t *testing.T, client *fakeAkahuClient, accountID string, want time.Time) {
	t.Helper()

	if len(client.fetchCalls) != 1 {
		t.Fatalf("FetchTransactions called %d times, want 1", len(client.fetchCalls))
	}
	if client.fetchCalls[0].accountID != accountID {
		t.Fatalf("FetchTransactions accountID = %q, want %q", client.fetchCalls[0].accountID, accountID)
	}
	if !client.fetchCalls[0].since.Equal(want) {
		t.Fatalf("FetchTransactions since = %v, want %v", client.fetchCalls[0].since, want)
	}
}

func assertSyncErrorWithoutSensitiveValues(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatal("Sync returned nil error, want error")
	}
	message := err.Error()
	if !strings.Contains(message, want) {
		t.Fatalf("error = %q, want to contain %q", message, want)
	}
	for _, sensitive := range []string{
		"app-token-secret",
		"user-token-secret",
		"Coffee",
		"Cafe",
		"12.34",
		`{"id":"txn-1"}`,
	} {
		if strings.Contains(message, sensitive) {
			t.Fatalf("error = %q, leaked sensitive value %q", message, sensitive)
		}
	}
}
