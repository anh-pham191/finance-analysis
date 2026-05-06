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
