package categorise

import "github.com/anh-pham191/finance-analysis/internal/domain"

type CategoryKind string

const (
	KindIncome   CategoryKind = "income"
	KindExpense  CategoryKind = "expense"
	KindTransfer CategoryKind = "transfer"
)

type Category struct {
	Name   string       `yaml:"name"`
	Kind   CategoryKind `yaml:"kind"`
	Parent string       `yaml:"parent,omitempty"`
}

type Predicate struct {
	Direction          string        `yaml:"direction,omitempty"`
	AmountMin          *domain.Money `yaml:"amount_min,omitempty"`
	AmountMax          *domain.Money `yaml:"amount_max,omitempty"`
	DescriptionMatches string        `yaml:"description_matches,omitempty"`
	MerchantIn         []string      `yaml:"merchant_in,omitempty"`
	MerchantMatches    string        `yaml:"merchant_matches,omitempty"`
	AccountIn          []string      `yaml:"account_in,omitempty"`
	AkahuCategory      string        `yaml:"akahu_category,omitempty"`
}

type Rule struct {
	Name      string    `yaml:"name"`
	Priority  int       `yaml:"priority"`
	Predicate Predicate `yaml:"when"`
	Category  string    `yaml:"category"`
	Enabled   *bool     `yaml:"enabled,omitempty"`
}

func (r Rule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

type Assignment struct {
	Category string
	Source   domain.AssignmentSource
	RuleName string
}
