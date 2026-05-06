package categorise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestLoadCategoriesLoadsValidYAML(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "categories.yaml", `
- name: Income
  kind: income
- name: Food
  kind: expense
- name: Food/Groceries
  kind: expense
  parent: Food
`)

	categories, err := LoadCategories(path)
	if err != nil {
		t.Fatalf("LoadCategories returned error: %v", err)
	}

	want := []Category{
		{Name: "Income", Kind: KindIncome},
		{Name: "Food", Kind: KindExpense},
		{Name: "Food/Groceries", Kind: KindExpense, Parent: "Food"},
	}
	if len(categories) != len(want) {
		t.Fatalf("LoadCategories returned %d categories, want %d", len(categories), len(want))
	}
	for i := range want {
		if categories[i] != want[i] {
			t.Fatalf("category %d = %+v, want %+v", i, categories[i], want[i])
		}
	}
}

func TestLoadCategoriesErrorsForMissingKind(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "categories.yaml", `
- name: Food
`)

	_, err := LoadCategories(path)
	assertErrorContains(t, err, "kind")
}

func TestLoadCategoriesErrorsForInvalidKind(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "categories.yaml", `
- name: Food
  kind: spending
`)

	_, err := LoadCategories(path)
	assertErrorContains(t, err, "spending")
}

func TestLoadCategoriesErrorsForOrphanParent(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "categories.yaml", `
- name: Food/Groceries
  kind: expense
  parent: Food
`)

	_, err := LoadCategories(path)
	assertErrorContains(t, err, "Food")
}

func TestLoadCategoriesErrorsForDuplicateNames(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "categories.yaml", `
- name: Food
  kind: expense
- name: Food
  kind: expense
`)

	_, err := LoadCategories(path)
	assertErrorContains(t, err, "Food")
}

func TestLoadCategoriesErrorsForUnknownFields(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "categories.yaml", `
- name: Food
  kind: expense
  colour: red
`)

	_, err := LoadCategories(path)
	assertErrorContains(t, err, "colour")
}

func TestLoadCategoriesErrorsForMissingName(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "categories.yaml", `
- kind: expense
`)

	_, err := LoadCategories(path)
	assertErrorContains(t, err, "name")
}

func TestLoadRulesLoadsValidYAML(t *testing.T) {
	t.Parallel()

	categories := []Category{
		{Name: "Food/Groceries", Kind: KindExpense},
		{Name: "Transfers", Kind: KindTransfer},
	}
	path := writeYAML(t, "rules.yaml", `
- name: Groceries
  priority: 20
  when:
    direction: DEBIT
    amount_min: "10.25"
    amount_max: "250.00"
    description_matches: "(?i)supermarket"
    merchant_in: ["Countdown", "New World"]
    merchant_matches: "(?i)pak'?n?save"
    account_in: ["everyday"]
    akahu_category: GROCERIES
  category: Food/Groceries
- name: Transfers
  priority: 5
  when:
    akahu_category: TRANSFER
  category: Transfers
`)

	rules, err := LoadRules(path, categories)
	if err != nil {
		t.Fatalf("LoadRules returned error: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("LoadRules returned %d rules, want 2", len(rules))
	}
	rule := rules[0]
	if rule.Name != "Groceries" || rule.Priority != 20 || rule.Category != "Food/Groceries" {
		t.Fatalf("rule metadata = %+v, want Groceries priority 20 category Food/Groceries", rule)
	}
	if !rule.IsEnabled() {
		t.Fatal("expected omitted enabled to default to true")
	}
	if rule.Predicate.Direction != "DEBIT" {
		t.Fatalf("Direction = %q, want DEBIT", rule.Predicate.Direction)
	}
	if rule.Predicate.AmountMin == nil || rule.Predicate.AmountMin.String() != "10.25" {
		t.Fatalf("AmountMin = %v, want 10.25", rule.Predicate.AmountMin)
	}
	if rule.Predicate.AmountMax == nil || rule.Predicate.AmountMax.String() != "250.00" {
		t.Fatalf("AmountMax = %v, want 250.00", rule.Predicate.AmountMax)
	}
	if rule.Predicate.DescriptionMatches != "(?i)supermarket" {
		t.Fatalf("DescriptionMatches = %q", rule.Predicate.DescriptionMatches)
	}
	if strings.Join(rule.Predicate.MerchantIn, ",") != "Countdown,New World" {
		t.Fatalf("MerchantIn = %v", rule.Predicate.MerchantIn)
	}
	if rule.Predicate.MerchantMatches != "(?i)pak'?n?save" {
		t.Fatalf("MerchantMatches = %q", rule.Predicate.MerchantMatches)
	}
	if strings.Join(rule.Predicate.AccountIn, ",") != "everyday" {
		t.Fatalf("AccountIn = %v", rule.Predicate.AccountIn)
	}
	if rule.Predicate.AkahuCategory != "GROCERIES" {
		t.Fatalf("AkahuCategory = %q, want GROCERIES", rule.Predicate.AkahuCategory)
	}
}

func TestLoadRulesErrorsForMissingReferencedCategory(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Groceries
  category: Food/Groceries
`)

	_, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	assertErrorContains(t, err, "Food/Groceries")
}

func TestLoadRulesErrorsForDuplicateNames(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Groceries
  category: Food
- name: Groceries
  category: Food
`)

	_, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	assertErrorContains(t, err, "Groceries")
}

func TestLoadRulesErrorsForUnknownTopLevelFields(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Groceries
  priority: 20
  category: Food
  stop_after_match: true
`)

	_, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	assertErrorContains(t, err, "stop_after_match")
}

func TestLoadRulesErrorsForUnknownPredicateFields(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Groceries
  when:
    merchant_contains: Market
  category: Food
`)

	_, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	assertErrorContains(t, err, "merchant_contains")
}

func TestLoadRulesErrorsForMissingName(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- category: Food
`)

	_, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	assertErrorContains(t, err, "name")
}

func TestLoadRulesErrorsForBadDescriptionRegex(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Bad description regex
  when:
    description_matches: "["
  category: Food
`)

	_, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	assertErrorContains(t, err, "description_matches")
}

func TestLoadRulesErrorsForBadMerchantRegex(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Bad merchant regex
  when:
    merchant_matches: "["
  category: Food
`)

	_, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	assertErrorContains(t, err, "merchant_matches")
}

func TestLoadRulesLoadsExplicitDisabledRule(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Disabled
  enabled: false
  category: Food
`)

	rules, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	if err != nil {
		t.Fatalf("LoadRules returned error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("LoadRules returned %d rules, want 1", len(rules))
	}
	if rules[0].IsEnabled() {
		t.Fatal("expected explicit enabled: false to disable rule")
	}
}

func TestLoadRulesLoadsAmountScalarsIntoMoney(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "rules.yaml", `
- name: Amount range
  when:
    amount_min: "1.20"
    amount_max: "3.40"
  category: Food
`)

	rules, err := LoadRules(path, []Category{{Name: "Food", Kind: KindExpense}})
	if err != nil {
		t.Fatalf("LoadRules returned error: %v", err)
	}

	wantMin := domain.MustMoneyFromString("1.20")
	wantMax := domain.MustMoneyFromString("3.40")
	if rules[0].Predicate.AmountMin == nil || rules[0].Predicate.AmountMin.String() != wantMin.String() {
		t.Fatalf("AmountMin = %v, want %s", rules[0].Predicate.AmountMin, wantMin.String())
	}
	if rules[0].Predicate.AmountMax == nil || rules[0].Predicate.AmountMax.String() != wantMax.String() {
		t.Fatalf("AmountMax = %v, want %s", rules[0].Predicate.AmountMax, wantMax.String())
	}
}

func writeYAML(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o600); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	return path
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, err.Error())
	}
}
