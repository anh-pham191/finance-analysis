package report

import "github.com/anh-pham191/finance-analysis/internal/domain"

type MoneyAmount string

type SummaryResult struct {
	Period           domain.Range    `json:"period"`
	Income           MoneyAmount     `json:"income"`
	Expense          MoneyAmount     `json:"expense"`
	Net              MoneyAmount     `json:"net"`
	Categories       []CategoryTotal `json:"categories"`
	HasUncategorised bool            `json:"has_uncategorised"`
}

type CategoryTotal struct {
	CategoryID int64       `json:"category_id"`
	Category   string      `json:"category"`
	Kind       string      `json:"kind"`
	Total      MoneyAmount `json:"total"`
}

type CompareResult struct {
	A          domain.Range      `json:"a"`
	B          domain.Range      `json:"b"`
	Categories []CompareCategory `json:"categories"`
}

type CompareCategory struct {
	CategoryID   int64       `json:"category_id"`
	Category     string      `json:"category"`
	Kind         string      `json:"kind"`
	A            MoneyAmount `json:"a"`
	B            MoneyAmount `json:"b"`
	Delta        MoneyAmount `json:"delta"`
	DeltaPercent *float64    `json:"delta_percent"`
}
