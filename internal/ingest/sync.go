package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

type Deps struct {
	Accounts       ports.AccountRepo
	Txns           ports.TxnRepo
	SyncStates     ports.SyncStateRepo
	Tokens         ports.TokenStore
	NewAkahuClient func(appToken, userToken string) ports.AkahuClient
	Clock          ports.Clock
}

type Options struct {
	From *time.Time
}

type Result struct {
	Accounts     int
	Transactions int
}

func Sync(ctx context.Context, userID domain.UserID, deps Deps, opts Options) (Result, error) {
	if err := validateDeps(deps); err != nil {
		return Result{}, err
	}

	appToken, userToken, err := deps.Tokens.AkahuTokens(ctx, userID)
	if err != nil {
		return Result{}, fmt.Errorf("load Akahu tokens: %w", err)
	}

	client := deps.NewAkahuClient(appToken, userToken)
	if client == nil {
		return Result{}, errors.New("new Akahu client returned nil")
	}
	accounts, err := client.ListAccounts(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("list Akahu accounts: %w", err)
	}

	result := Result{Accounts: len(accounts)}
	for _, rawAccount := range accounts {
		account := mapAccount(rawAccount)
		if err := deps.Accounts.Upsert(ctx, userID, account); err != nil {
			return Result{}, fmt.Errorf("upsert account %s: %w", account.ID, err)
		}

		state, err := deps.SyncStates.Get(ctx, userID, account.ID)
		if err != nil && !isSyncStateNotFound(err) {
			return Result{}, fmt.Errorf("load sync state for account %s: %w", account.ID, err)
		}

		txns, err := client.FetchTransactions(ctx, account.ID, syncSince(deps.Clock.Now(), state, opts))
		if err != nil {
			return Result{}, fmt.Errorf("fetch transactions for account %s: %w", account.ID, err)
		}
		result.Transactions += len(txns)

		for _, rawTxn := range txns {
			txn, err := mapTxn(rawTxn)
			if err != nil {
				return Result{}, err
			}
			if err := deps.Txns.Upsert(ctx, userID, txn); err != nil {
				return Result{}, fmt.Errorf("upsert transaction %s: %w", txn.ID, err)
			}
		}

		lastSyncedAt := deps.Clock.Now()
		if err := deps.SyncStates.Upsert(ctx, userID, domain.SyncState{
			AccountID:    account.ID,
			LastSyncedAt: &lastSyncedAt,
		}); err != nil {
			return Result{}, fmt.Errorf("upsert sync state for account %s: %w", account.ID, err)
		}
	}

	return result, nil
}

func validateDeps(deps Deps) error {
	switch {
	case deps.Accounts == nil:
		return errors.New("missing account repo")
	case deps.Txns == nil:
		return errors.New("missing transaction repo")
	case deps.SyncStates == nil:
		return errors.New("missing sync state repo")
	case deps.Tokens == nil:
		return errors.New("missing token store")
	case deps.NewAkahuClient == nil:
		return errors.New("missing Akahu client factory")
	case deps.Clock == nil:
		return errors.New("missing clock")
	default:
		return nil
	}
}

func syncSince(now time.Time, state domain.SyncState, opts Options) time.Time {
	if opts.From != nil {
		return *opts.From
	}
	if state.LastSyncedAt != nil {
		return state.LastSyncedAt.Add(-24 * time.Hour)
	}
	return now.AddDate(0, 0, -30)
}

func isSyncStateNotFound(err error) bool {
	return errors.Is(err, ports.ErrNotFound)
}
