package categorise

import (
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestPredicateMatch(t *testing.T) {
	t.Parallel()

	base := predicateTestTransaction()

	tests := map[string]struct {
		predicate Predicate
		txn       domain.Transaction
		want      bool
	}{
		"direction matches same direction": {
			predicate: Predicate{Direction: "DEBIT"},
			txn:       base,
			want:      true,
		},
		"direction rejects different direction": {
			predicate: Predicate{Direction: "CREDIT"},
			txn:       base,
			want:      false,
		},
		"amount_min matches amount greater than min": {
			predicate: Predicate{AmountMin: moneyPtr("40.00")},
			txn:       base,
			want:      true,
		},
		"amount_min matches amount equal to min": {
			predicate: Predicate{AmountMin: moneyPtr("42.50")},
			txn:       base,
			want:      true,
		},
		"amount_min rejects lower amount": {
			predicate: Predicate{AmountMin: moneyPtr("42.51")},
			txn:       base,
			want:      false,
		},
		"amount_max matches amount less than max": {
			predicate: Predicate{AmountMax: moneyPtr("50.00")},
			txn:       base,
			want:      true,
		},
		"amount_max matches amount equal to max": {
			predicate: Predicate{AmountMax: moneyPtr("42.50")},
			txn:       base,
			want:      true,
		},
		"amount_max rejects higher amount": {
			predicate: Predicate{AmountMax: moneyPtr("42.49")},
			txn:       base,
			want:      false,
		},
		"description_matches matches description": {
			predicate: Predicate{DescriptionMatches: "(?i)weekly supermarket"},
			txn:       base,
			want:      true,
		},
		"description_matches rejects non-matching description": {
			predicate: Predicate{DescriptionMatches: "(?i)payroll"},
			txn:       base,
			want:      false,
		},
		"merchant_in matches exact merchant in list": {
			predicate: Predicate{MerchantIn: []string{"Countdown", "New World"}},
			txn:       base,
			want:      true,
		},
		"merchant_in rejects merchant outside list": {
			predicate: Predicate{MerchantIn: []string{"PaknSave", "New World"}},
			txn:       base,
			want:      false,
		},
		"merchant_matches matches merchant": {
			predicate: Predicate{MerchantMatches: "(?i)^count"},
			txn:       base,
			want:      true,
		},
		"merchant_matches rejects non-matching merchant": {
			predicate: Predicate{MerchantMatches: "(?i)^pak"},
			txn:       base,
			want:      false,
		},
		"account_in matches exact account id in list": {
			predicate: Predicate{AccountIn: []string{"acct-savings", "acct-everyday"}},
			txn:       base,
			want:      true,
		},
		"account_in rejects account id outside list": {
			predicate: Predicate{AccountIn: []string{"acct-savings", "acct-credit"}},
			txn:       base,
			want:      false,
		},
		"akahu_category matches exact category": {
			predicate: Predicate{AkahuCategory: "GROCERIES"},
			txn:       base,
			want:      true,
		},
		"akahu_category rejects different category": {
			predicate: Predicate{AkahuCategory: "TRANSFER"},
			txn:       base,
			want:      false,
		},
		"combination matches when all fields match": {
			predicate: Predicate{
				Direction:          "DEBIT",
				AmountMin:          moneyPtr("40.00"),
				AmountMax:          moneyPtr("50.00"),
				DescriptionMatches: "(?i)supermarket",
				MerchantIn:         []string{"Countdown", "New World"},
				MerchantMatches:    "(?i)down$",
				AccountIn:          []string{"acct-everyday"},
				AkahuCategory:      "GROCERIES",
			},
			txn:  base,
			want: true,
		},
		"combination rejects when one field differs": {
			predicate: Predicate{
				Direction:          "DEBIT",
				AmountMin:          moneyPtr("40.00"),
				AmountMax:          moneyPtr("50.00"),
				DescriptionMatches: "(?i)supermarket",
				MerchantIn:         []string{"Countdown", "New World"},
				MerchantMatches:    "(?i)down$",
				AccountIn:          []string{"acct-savings"},
				AkahuCategory:      "GROCERIES",
			},
			txn:  base,
			want: false,
		},
		"empty predicate matches any transaction": {
			predicate: Predicate{},
			txn:       domain.Transaction{ID: "txn-empty", Amount: domain.MustMoneyFromString("0.00")},
			want:      true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := test.predicate.Match(test.txn); got != test.want {
				t.Fatalf("Match() = %v, want %v", got, test.want)
			}
		})
	}
}

func predicateTestTransaction() domain.Transaction {
	return domain.Transaction{
		ID:            "txn-groceries",
		AccountID:     "acct-everyday",
		Amount:        domain.MustMoneyFromString("42.50"),
		Direction:     domain.DirectionDebit,
		Description:   "Weekly supermarket shop",
		Merchant:      "Countdown",
		AkahuCategory: "GROCERIES",
	}
}

func moneyPtr(value string) *domain.Money {
	money := domain.MustMoneyFromString(value)
	return &money
}
