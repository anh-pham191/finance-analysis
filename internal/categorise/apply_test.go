package categorise

import (
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestApply(t *testing.T) {
	t.Parallel()

	txn := applyTestTransaction()

	tests := map[string]struct {
		rules        []Rule
		wantOK       bool
		wantCategory string
		wantRuleName string
	}{
		"first matching enabled rule wins": {
			rules: []Rule{
				{
					Name:     "Eating out",
					Priority: 10,
					Predicate: Predicate{
						MerchantIn: []string{"Cafe"},
					},
					Category: "Food/Eating-out",
				},
				{
					Name:     "Groceries",
					Priority: 20,
					Predicate: Predicate{
						MerchantIn: []string{"Countdown"},
					},
					Category: "Food/Groceries",
				},
				{
					Name:     "Supermarket",
					Priority: 30,
					Predicate: Predicate{
						DescriptionMatches: "(?i)supermarket",
					},
					Category: "Food",
				},
			},
			wantOK:       true,
			wantCategory: "Food/Groceries",
			wantRuleName: "Groceries",
		},
		"disabled rules skipped": {
			rules: []Rule{
				{
					Name:     "Disabled groceries",
					Priority: 10,
					Predicate: Predicate{
						MerchantIn: []string{"Countdown"},
					},
					Category: "Food/Disabled",
					Enabled:  boolPtr(false),
				},
				{
					Name:     "Enabled groceries",
					Priority: 20,
					Predicate: Predicate{
						MerchantIn: []string{"Countdown"},
					},
					Category: "Food/Groceries",
				},
			},
			wantOK:       true,
			wantCategory: "Food/Groceries",
			wantRuleName: "Enabled groceries",
		},
		"lower priority number wins": {
			rules: []Rule{
				{
					Name:     "Later high number",
					Priority: 50,
					Predicate: Predicate{
						MerchantIn: []string{"Countdown"},
					},
					Category: "Food",
				},
				{
					Name:     "Earlier low number",
					Priority: 5,
					Predicate: Predicate{
						MerchantIn: []string{"Countdown"},
					},
					Category: "Food/Groceries",
				},
			},
			wantOK:       true,
			wantCategory: "Food/Groceries",
			wantRuleName: "Earlier low number",
		},
		"equal priority tiebreaks by name ascending": {
			rules: []Rule{
				{
					Name:     "Z supermarket",
					Priority: 10,
					Predicate: Predicate{
						MerchantIn: []string{"Countdown"},
					},
					Category: "Food",
				},
				{
					Name:     "A groceries",
					Priority: 10,
					Predicate: Predicate{
						MerchantIn: []string{"Countdown"},
					},
					Category: "Food/Groceries",
				},
			},
			wantOK:       true,
			wantCategory: "Food/Groceries",
			wantRuleName: "A groceries",
		},
		"no explicit match returns false": {
			rules: []Rule{
				{
					Name:     "Salary",
					Priority: 10,
					Predicate: Predicate{
						Direction: "CREDIT",
					},
					Category: "Income/Salary",
				},
			},
			wantOK: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assignment, ok := Apply(txn, test.rules)
			if ok != test.wantOK {
				t.Fatalf("Apply() ok = %v, want %v", ok, test.wantOK)
			}
			if !test.wantOK {
				if assignment.Category != "" || assignment.RuleName != "" {
					t.Fatalf("Apply() assignment = %+v, want no category or rule", assignment)
				}
				return
			}

			if assignment.Source != domain.AssignmentSourceRule {
				t.Fatalf("Apply() source = %q, want %q", assignment.Source, domain.AssignmentSourceRule)
			}
			if assignment.Category != test.wantCategory {
				t.Fatalf("Apply() category = %q, want %q", assignment.Category, test.wantCategory)
			}
			if assignment.RuleName != test.wantRuleName {
				t.Fatalf("Apply() rule name = %q, want %q", assignment.RuleName, test.wantRuleName)
			}
		})
	}
}

func TestApplyDoesNotMutateRules(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{Name: "B groceries", Priority: 20, Predicate: Predicate{MerchantIn: []string{"Countdown"}}, Category: "Food"},
		{Name: "A groceries", Priority: 10, Predicate: Predicate{MerchantIn: []string{"Countdown"}}, Category: "Food/Groceries"},
	}

	_, _ = Apply(applyTestTransaction(), rules)

	if rules[0].Name != "B groceries" || rules[1].Name != "A groceries" {
		t.Fatalf("Apply() mutated caller rules order: %+v", rules)
	}
}

func applyTestTransaction() domain.Transaction {
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

func boolPtr(value bool) *bool {
	return &value
}
