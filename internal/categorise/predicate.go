package categorise

import (
	"regexp"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func (p Predicate) Match(txn domain.Transaction) bool {
	if p.Direction != "" && domain.Direction(p.Direction) != txn.Direction {
		return false
	}
	if p.AmountMin != nil && txn.Amount.Decimal().Cmp(p.AmountMin.Decimal()) < 0 {
		return false
	}
	if p.AmountMax != nil && txn.Amount.Decimal().Cmp(p.AmountMax.Decimal()) > 0 {
		return false
	}
	if p.DescriptionMatches != "" && !regexMatches(p.DescriptionMatches, txn.Description) {
		return false
	}
	if len(p.MerchantIn) > 0 && !containsString(p.MerchantIn, txn.Merchant) {
		return false
	}
	if p.MerchantMatches != "" && !regexMatches(p.MerchantMatches, txn.Merchant) {
		return false
	}
	if len(p.AccountIn) > 0 && !containsString(p.AccountIn, txn.AccountID) {
		return false
	}
	if p.AkahuCategory != "" && p.AkahuCategory != txn.AkahuCategory {
		return false
	}
	return true
}

func regexMatches(expression, value string) bool {
	matcher, err := regexp.Compile(expression)
	if err != nil {
		return false
	}
	return matcher.MatchString(value)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
