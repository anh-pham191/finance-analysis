package report

import (
	"context"
	"errors"
	"sort"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
	"github.com/shopspring/decimal"
)

type SummaryDeps struct {
	Txns        ports.TxnRepo
	Categories  ports.CategoryRepo
	Assignments ports.AssignmentRepo
}

func Summary(ctx context.Context, userID domain.UserID, deps SummaryDeps, period domain.Range) (SummaryResult, error) {
	if deps.Txns == nil {
		return SummaryResult{}, errors.New("summary: missing transaction repo")
	}
	if deps.Categories == nil {
		return SummaryResult{}, errors.New("summary: missing category repo")
	}
	if deps.Assignments == nil {
		return SummaryResult{}, errors.New("summary: missing assignment repo")
	}

	txns, err := deps.Txns.List(ctx, userID)
	if err != nil {
		return SummaryResult{}, errors.New("summary: list transactions failed")
	}
	categories, err := deps.Categories.List(ctx, userID)
	if err != nil {
		return SummaryResult{}, errors.New("summary: list categories failed")
	}

	categoryByID := make(map[int64]domain.Category, len(categories))
	var uncategorised *domain.Category
	for _, category := range categories {
		categoryByID[category.ID] = category
		if category.Name == "Uncategorised" {
			copied := category
			uncategorised = &copied
		}
	}

	totals := make(map[int64]decimal.Decimal)
	assignedCounts := make(map[int64]int)
	income := decimal.Zero
	expense := decimal.Zero

	for _, txn := range txns {
		if txn.PostedAt.Before(period.From) || !txn.PostedAt.Before(period.To) {
			continue
		}

		assignment, err := deps.Assignments.Get(ctx, userID, txn.ID)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) && uncategorised != nil {
				assignment = domain.CategoryAssignment{TxnID: txn.ID, CategoryID: uncategorised.ID}
			} else if errors.Is(err, ports.ErrNotFound) {
				continue
			} else {
				return SummaryResult{}, errors.New("summary: get assignment failed")
			}
		}

		category, ok := categoryByID[assignment.CategoryID]
		if !ok {
			continue
		}

		amount := txn.Amount.Decimal()
		totalAmount := summaryCategoryAmount(category, txn.Direction, amount)
		totals[category.ID] = totals[category.ID].Add(totalAmount)
		assignedCounts[category.ID]++

		switch {
		case category.Kind == domain.CategoryKindIncome && txn.Direction == domain.DirectionCredit:
			income = income.Add(amount)
		case category.Kind == domain.CategoryKindExpense && txn.Direction == domain.DirectionDebit:
			expense = expense.Add(amount.Abs())
		}
	}

	result := SummaryResult{
		Period:           period,
		Income:           moneyAmount(income),
		Expense:          moneyAmount(expense),
		Net:              moneyAmount(income.Sub(expense)),
		Categories:       categoryTotals(categories, totals, assignedCounts),
		HasUncategorised: uncategorised != nil && assignedCounts[uncategorised.ID] > 0,
	}
	return result, nil
}

func categoryTotals(categories []domain.Category, totals map[int64]decimal.Decimal, assignedCounts map[int64]int) []CategoryTotal {
	result := make([]CategoryTotal, 0, len(totals))
	for _, category := range categories {
		total := totals[category.ID]
		if total.IsZero() && assignedCounts[category.ID] == 0 {
			continue
		}
		result = append(result, CategoryTotal{
			CategoryID: category.ID,
			Category:   category.Name,
			Kind:       string(category.Kind),
			Total:      moneyAmount(total),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Category == result[j].Category {
			return result[i].CategoryID < result[j].CategoryID
		}
		return result[i].Category < result[j].Category
	})
	return result
}

func moneyAmount(value decimal.Decimal) MoneyAmount {
	return MoneyAmount(value.Round(2).StringFixed(2))
}

func summaryCategoryAmount(category domain.Category, direction domain.Direction, amount decimal.Decimal) decimal.Decimal {
	if direction == domain.DirectionDebit {
		switch category.Kind {
		case domain.CategoryKindExpense, domain.CategoryKindTransfer:
			return amount.Abs()
		}
	}
	return amount
}
