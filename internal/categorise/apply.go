package categorise

import (
	"sort"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func Apply(txn domain.Transaction, rules []Rule) (Assignment, bool) {
	evaluationOrder := append([]Rule(nil), rules...)
	sort.Slice(evaluationOrder, func(i, j int) bool {
		if evaluationOrder[i].Priority != evaluationOrder[j].Priority {
			return evaluationOrder[i].Priority < evaluationOrder[j].Priority
		}
		return evaluationOrder[i].Name < evaluationOrder[j].Name
	})

	for _, rule := range evaluationOrder {
		if !rule.IsEnabled() || !rule.Predicate.Match(txn) {
			continue
		}
		return Assignment{
			Category: rule.Category,
			Source:   domain.AssignmentSourceRule,
			RuleName: rule.Name,
		}, true
	}

	return Assignment{}, false
}
