package report

import (
	"context"
	"errors"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

type TransactionsDeps struct {
	Txns ports.TxnQueryRepo
}

type TxnFilter struct {
	Period     domain.Range
	CategoryID *int64
	Merchant   string
	AccountID  string
	Direction  *domain.Direction
	Min        *domain.Money
	Max        *domain.Money
	Sort       string
	Limit      int
	Offset     int
}

func Transactions(ctx context.Context, userID domain.UserID, deps TransactionsDeps, filter TxnFilter) ([]TxnRow, error) {
	if deps.Txns == nil {
		return nil, errors.New("transactions: missing transaction query repo")
	}

	reportRows, err := deps.Txns.ListFiltered(ctx, userID, ports.TxnFilter{
		Range:      filter.Period,
		CategoryID: filter.CategoryID,
		Merchant:   filter.Merchant,
		AccountID:  filter.AccountID,
		Direction:  filter.Direction,
		Min:        filter.Min,
		Max:        filter.Max,
		Sort:       filter.Sort,
		Limit:      transactionsLimit(filter.Limit),
		Offset:     transactionsOffset(filter.Offset),
	})
	if err != nil {
		return nil, errors.New("transactions: list filtered transactions failed")
	}

	rows := make([]TxnRow, 0, len(reportRows))
	for _, reportRow := range reportRows {
		txn := reportRow.Transaction
		rows = append(rows, TxnRow{
			TxnID:       txn.ID,
			PostedAt:    txn.PostedAt,
			AccountID:   txn.AccountID,
			Category:    reportRow.Category,
			Direction:   string(txn.Direction),
			Amount:      MoneyAmount(txn.Amount.String()),
			Merchant:    txn.Merchant,
			Description: txn.Description,
		})
	}
	return rows, nil
}

func transactionsLimit(limit int) int {
	if limit == 0 {
		return 100
	}
	return limit
}

func transactionsOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
