package categorise

import (
	"reflect"
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestTypesCategoryKinds(t *testing.T) {
	t.Parallel()

	tests := map[string]CategoryKind{
		"income":   KindIncome,
		"expense":  KindExpense,
		"transfer": KindTransfer,
	}

	for want, kind := range tests {
		if string(kind) != want {
			t.Fatalf("expected %q, got %q", want, kind)
		}
	}
}

func TestTypesAssignmentSources(t *testing.T) {
	t.Parallel()

	tests := map[domain.AssignmentSource]Assignment{
		domain.AssignmentSourceRule:   {Source: domain.AssignmentSourceRule, RuleName: "Groceries"},
		domain.AssignmentSourceManual: {Source: domain.AssignmentSourceManual},
		domain.AssignmentSourceAkahu:  {Source: domain.AssignmentSourceAkahu},
	}

	for want, assignment := range tests {
		if assignment.Source != want {
			t.Fatalf("expected %q, got %q", want, assignment.Source)
		}
	}
}

func TestTypesRuleEnabledDefaultsToEnabled(t *testing.T) {
	t.Parallel()

	var rule Rule
	if !rule.IsEnabled() {
		t.Fatal("expected zero-value rule to be enabled")
	}

	disabled := false
	rule.Enabled = &disabled
	if rule.IsEnabled() {
		t.Fatal("expected explicit false to disable rule")
	}

	enabled := true
	rule.Enabled = &enabled
	if !rule.IsEnabled() {
		t.Fatal("expected explicit true to enable rule")
	}
}

func TestTypesConfigYAMLTags(t *testing.T) {
	t.Parallel()

	assertYAMLTag(t, reflect.TypeOf(Rule{}), "Predicate", "when")
	assertYAMLTag(t, reflect.TypeOf(Predicate{}), "DescriptionMatches", "description_matches,omitempty")
	assertYAMLTag(t, reflect.TypeOf(Predicate{}), "MerchantIn", "merchant_in,omitempty")
	assertYAMLTag(t, reflect.TypeOf(Predicate{}), "AkahuCategory", "akahu_category,omitempty")
	assertYAMLTag(t, reflect.TypeOf(Rule{}), "Enabled", "enabled,omitempty")
}

func assertYAMLTag(t *testing.T, typ reflect.Type, fieldName, want string) {
	t.Helper()

	field, ok := typ.FieldByName(fieldName)
	if !ok {
		t.Fatalf("expected %s to have field %s", typ.Name(), fieldName)
	}
	if got := field.Tag.Get("yaml"); got != want {
		t.Fatalf("expected %s.%s yaml tag %q, got %q", typ.Name(), fieldName, want, got)
	}
}
