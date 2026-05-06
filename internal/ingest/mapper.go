package ingest

import (
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func mapAccount(raw ports.RawAccount) domain.Account {
	currency := raw.Currency
	if currency == "" {
		currency = "NZD"
	}

	return domain.Account{
		ID:       raw.ID,
		Name:     raw.Name,
		Bank:     raw.Bank,
		Type:     raw.Type,
		Currency: currency,
	}
}

func mapTxn(raw ports.RawTxn) (domain.Transaction, error) {
	amount, err := domain.NewMoneyFromString(raw.Amount)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("map txn %s: invalid amount", raw.ID)
	}

	direction, err := domain.ParseDirection(raw.Direction)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("map txn %s: unknown direction", raw.ID)
	}

	return domain.Transaction{
		ID:            raw.ID,
		AccountID:     raw.AccountID,
		PostedAt:      raw.PostedAt,
		Amount:        amount,
		Direction:     direction,
		Description:   raw.Description,
		Merchant:      raw.Merchant,
		AkahuCategory: raw.AkahuCategory,
		RawJSON:       raw.RawJSON,
	}, nil
}
