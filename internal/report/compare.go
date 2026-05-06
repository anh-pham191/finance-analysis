package report

import (
	"context"
	"errors"
	"sort"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/shopspring/decimal"
)

type CompareOptions struct {
	Top int
}

func Compare(
	ctx context.Context,
	userID domain.UserID,
	deps SummaryDeps,
	a domain.Range,
	b domain.Range,
	opts CompareOptions,
) (CompareResult, error) {
	aSummary, err := Summary(ctx, userID, deps, a)
	if err != nil {
		return CompareResult{}, errors.New("compare: summarise A failed")
	}
	bSummary, err := Summary(ctx, userID, deps, b)
	if err != nil {
		return CompareResult{}, errors.New("compare: summarise B failed")
	}

	rows, err := compareCategories(aSummary.Categories, bSummary.Categories)
	if err != nil {
		return CompareResult{}, err
	}
	if opts.Top > 0 && opts.Top < len(rows) {
		rows = rows[:opts.Top]
	}

	return CompareResult{
		A:          a,
		B:          b,
		Categories: rows,
	}, nil
}

type compareCategoryAmounts struct {
	categoryID int64
	category   string
	kind       string
	a          decimal.Decimal
	b          decimal.Decimal
}

type compareCategoryRow struct {
	category CompareCategory
	absDelta decimal.Decimal
}

func compareCategories(aTotals, bTotals []CategoryTotal) ([]CompareCategory, error) {
	amounts := make(map[int64]compareCategoryAmounts, len(aTotals)+len(bTotals))
	for _, total := range aTotals {
		amount, err := parseMoneyAmount(total.Total)
		if err != nil {
			return nil, errors.New("compare: parse A amount failed")
		}
		amounts[total.CategoryID] = compareCategoryAmounts{
			categoryID: total.CategoryID,
			category:   total.Category,
			kind:       total.Kind,
			a:          amount,
		}
	}
	for _, total := range bTotals {
		amount, err := parseMoneyAmount(total.Total)
		if err != nil {
			return nil, errors.New("compare: parse B amount failed")
		}
		current := amounts[total.CategoryID]
		current.categoryID = total.CategoryID
		current.category = total.Category
		current.kind = total.Kind
		current.b = amount
		amounts[total.CategoryID] = current
	}

	rows := make([]compareCategoryRow, 0, len(amounts))
	for _, amount := range amounts {
		delta := amount.b.Sub(amount.a)
		var deltaPercent *float64
		if !amount.a.IsZero() {
			value, _ := delta.Div(amount.a).Mul(decimal.NewFromInt(100)).Float64()
			deltaPercent = &value
		}
		rows = append(rows, compareCategoryRow{
			category: CompareCategory{
				CategoryID:   amount.categoryID,
				Category:     amount.category,
				Kind:         amount.kind,
				A:            moneyAmount(amount.a),
				B:            moneyAmount(amount.b),
				Delta:        moneyAmount(delta),
				DeltaPercent: deltaPercent,
			},
			absDelta: delta.Abs(),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].absDelta.Equal(rows[j].absDelta) {
			return rows[i].absDelta.GreaterThan(rows[j].absDelta)
		}
		if rows[i].category.Category == rows[j].category.Category {
			return rows[i].category.CategoryID < rows[j].category.CategoryID
		}
		return rows[i].category.Category < rows[j].category.Category
	})

	result := make([]CompareCategory, len(rows))
	for i, row := range rows {
		result[i] = row.category
	}
	return result, nil
}

func parseMoneyAmount(value MoneyAmount) (decimal.Decimal, error) {
	return decimal.NewFromString(string(value))
}
