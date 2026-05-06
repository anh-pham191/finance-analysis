package categorise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

const uncategorisedCategoryName = "Uncategorised"

type Deps struct {
	Categories  ports.CategoryRepo
	Rules       ports.RuleRepo
	Assignments ports.AssignmentRepo
	Txns        ports.TxnRepo
	Clock       ports.Clock
}

type Config struct {
	Categories []Category
	Rules      []Rule
}

func Categorise(ctx context.Context, userID domain.UserID, deps Deps, cfg Config) error {
	if err := validateDeps(deps); err != nil {
		return err
	}

	categoriesByName, err := reconcileCategories(ctx, userID, deps.Categories, cfg.Categories)
	if err != nil {
		return err
	}
	uncategorised, ok := categoriesByName[uncategorisedCategoryName]
	if !ok {
		return fmt.Errorf("categorise config: missing %q category", uncategorisedCategoryName)
	}

	rulesByName, err := reconcileRules(ctx, userID, deps.Rules, categoriesByName, cfg.Rules)
	if err != nil {
		return err
	}

	txns, err := deps.Txns.List(ctx, userID)
	if err != nil {
		return fmt.Errorf("list transactions for categorisation: %w", err)
	}

	for _, txn := range txns {
		existing, err := deps.Assignments.Get(ctx, userID, txn.ID)
		if err != nil && !errors.Is(err, ports.ErrNotFound) {
			return fmt.Errorf("get category assignment: %w", err)
		}
		if err == nil && existing.Source == domain.AssignmentSourceManual {
			continue
		}

		next := domain.CategoryAssignment{
			TxnID:      txn.ID,
			CategoryID: uncategorised.ID,
			Source:     domain.AssignmentSourceRule,
		}
		if assignment, ok := Apply(txn, cfg.Rules); ok {
			category, ok := categoriesByName[assignment.Category]
			if !ok {
				return fmt.Errorf("categorise rule %q references unknown category %q", assignment.RuleName, assignment.Category)
			}
			rule, ok := rulesByName[assignment.RuleName]
			if !ok {
				return fmt.Errorf("categorise rule %q was not reconciled", assignment.RuleName)
			}
			next.CategoryID = category.ID
			next.RuleID = &rule.ID
		}
		if err == nil && assignmentsEquivalent(existing, next) {
			continue
		}

		if _, err := deps.Assignments.UpsertIfChanged(ctx, userID, next); err != nil {
			return fmt.Errorf("upsert category assignment: %w", err)
		}
	}

	return nil
}

func assignmentsEquivalent(existing, desired domain.CategoryAssignment) bool {
	return existing.CategoryID == desired.CategoryID &&
		existing.Source == desired.Source &&
		sameOptionalRuleID(existing.RuleID, desired.RuleID)
}

func sameOptionalRuleID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func validateDeps(deps Deps) error {
	if deps.Categories == nil {
		return errors.New("categorise dependencies: categories repo is nil")
	}
	if deps.Rules == nil {
		return errors.New("categorise dependencies: rules repo is nil")
	}
	if deps.Assignments == nil {
		return errors.New("categorise dependencies: assignments repo is nil")
	}
	if deps.Txns == nil {
		return errors.New("categorise dependencies: transactions repo is nil")
	}
	if deps.Clock == nil {
		return errors.New("categorise dependencies: clock is nil")
	}
	return nil
}

func reconcileCategories(ctx context.Context, userID domain.UserID, repo ports.CategoryRepo, cfg []Category) (map[string]domain.Category, error) {
	configByName := make(map[string]Category, len(cfg))
	for _, category := range cfg {
		configByName[category.Name] = category
	}

	reconciled := make(map[string]domain.Category, len(cfg))
	visiting := make(map[string]bool, len(cfg))
	var upsert func(name string) (domain.Category, error)
	upsert = func(name string) (domain.Category, error) {
		if category, ok := reconciled[name]; ok {
			return category, nil
		}
		configCategory, ok := configByName[name]
		if !ok {
			return domain.Category{}, fmt.Errorf("category %q is not declared in config", name)
		}
		if visiting[name] {
			return domain.Category{}, fmt.Errorf("category %q has a parent cycle", name)
		}
		visiting[name] = true
		defer delete(visiting, name)

		var parentID *int64
		if configCategory.Parent != "" {
			parent, err := upsert(configCategory.Parent)
			if err != nil {
				return domain.Category{}, fmt.Errorf("resolve parent for category %q: %w", name, err)
			}
			parentID = &parent.ID
		}

		category, err := repo.Upsert(ctx, userID, domain.Category{
			Name:     configCategory.Name,
			ParentID: parentID,
			Kind:     domain.CategoryKind(configCategory.Kind),
		})
		if err != nil {
			return domain.Category{}, fmt.Errorf("upsert category %q: %w", configCategory.Name, err)
		}
		reconciled[name] = category
		return category, nil
	}

	for _, category := range cfg {
		if _, err := upsert(category.Name); err != nil {
			return nil, err
		}
	}
	return reconciled, nil
}

func reconcileRules(
	ctx context.Context,
	userID domain.UserID,
	repo ports.RuleRepo,
	categoriesByName map[string]domain.Category,
	cfg []Rule,
) (map[string]domain.Rule, error) {
	rulesByName := make(map[string]domain.Rule, len(cfg))
	keepNames := make([]string, 0, len(cfg))

	for _, configRule := range cfg {
		category, ok := categoriesByName[configRule.Category]
		if !ok {
			return nil, fmt.Errorf("rule %q references unknown category %q", configRule.Name, configRule.Category)
		}
		predicate, err := marshalPredicate(configRule.Predicate)
		if err != nil {
			return nil, fmt.Errorf("marshal predicate for rule %q: %w", configRule.Name, err)
		}
		rule, err := repo.Upsert(ctx, userID, domain.Rule{
			Name:       configRule.Name,
			Priority:   configRule.Priority,
			Predicate:  predicate,
			CategoryID: category.ID,
			Enabled:    configRule.IsEnabled(),
		})
		if err != nil {
			return nil, fmt.Errorf("upsert rule %q: %w", configRule.Name, err)
		}
		rulesByName[configRule.Name] = rule
		keepNames = append(keepNames, configRule.Name)
	}

	if err := repo.DeleteMissing(ctx, userID, keepNames); err != nil {
		return nil, fmt.Errorf("delete missing rules: %w", err)
	}
	return rulesByName, nil
}

func marshalPredicate(predicate Predicate) (json.RawMessage, error) {
	type storedPredicate struct {
		Direction          string   `json:"direction,omitempty"`
		AmountMin          *string  `json:"amount_min,omitempty"`
		AmountMax          *string  `json:"amount_max,omitempty"`
		DescriptionMatches string   `json:"description_matches,omitempty"`
		MerchantIn         []string `json:"merchant_in,omitempty"`
		MerchantMatches    string   `json:"merchant_matches,omitempty"`
		AccountIn          []string `json:"account_in,omitempty"`
		AkahuCategory      string   `json:"akahu_category,omitempty"`
	}

	var amountMin *string
	if predicate.AmountMin != nil {
		value := predicate.AmountMin.String()
		amountMin = &value
	}
	var amountMax *string
	if predicate.AmountMax != nil {
		value := predicate.AmountMax.String()
		amountMax = &value
	}

	data, err := json.Marshal(storedPredicate{
		Direction:          predicate.Direction,
		AmountMin:          amountMin,
		AmountMax:          amountMax,
		DescriptionMatches: predicate.DescriptionMatches,
		MerchantIn:         predicate.MerchantIn,
		MerchantMatches:    predicate.MerchantMatches,
		AccountIn:          predicate.AccountIn,
		AkahuCategory:      predicate.AkahuCategory,
	})
	return json.RawMessage(data), err
}
