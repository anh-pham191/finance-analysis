package categorise

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"gopkg.in/yaml.v3"
)

type ruleYAML struct {
	Name      string        `yaml:"name"`
	Priority  int           `yaml:"priority"`
	Predicate predicateYAML `yaml:"when"`
	Category  string        `yaml:"category"`
	Enabled   *bool         `yaml:"enabled,omitempty"`
}

type predicateYAML struct {
	Direction          string   `yaml:"direction,omitempty"`
	AmountMin          *string  `yaml:"amount_min,omitempty"`
	AmountMax          *string  `yaml:"amount_max,omitempty"`
	DescriptionMatches string   `yaml:"description_matches,omitempty"`
	MerchantIn         []string `yaml:"merchant_in,omitempty"`
	MerchantMatches    string   `yaml:"merchant_matches,omitempty"`
	AccountIn          []string `yaml:"account_in,omitempty"`
	AkahuCategory      string   `yaml:"akahu_category,omitempty"`
}

func LoadCategories(path string) ([]Category, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}

	var categories []Category
	if err := unmarshalKnownFields(data, &categories); err != nil {
		return nil, fmt.Errorf("parse categories: %w", err)
	}
	if err := validateCategories(categories); err != nil {
		return nil, err
	}
	return categories, nil
}

func LoadRules(path string, categories []Category) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}

	var rawRules []ruleYAML
	if err := unmarshalKnownFields(data, &rawRules); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}

	categoryNames := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		categoryNames[category.Name] = struct{}{}
	}

	ruleNames := make(map[string]struct{}, len(rawRules))
	rules := make([]Rule, 0, len(rawRules))
	for _, rawRule := range rawRules {
		if rawRule.Name == "" {
			return nil, fmt.Errorf("rule missing name")
		}
		if _, exists := ruleNames[rawRule.Name]; exists {
			return nil, fmt.Errorf("duplicate rule %q", rawRule.Name)
		}
		ruleNames[rawRule.Name] = struct{}{}

		if _, exists := categoryNames[rawRule.Category]; !exists {
			return nil, fmt.Errorf("rule %q references unknown category %q", rawRule.Name, rawRule.Category)
		}

		predicate, err := convertPredicate(rawRule.Name, rawRule.Predicate)
		if err != nil {
			return nil, err
		}

		rules = append(rules, Rule{
			Name:      rawRule.Name,
			Priority:  rawRule.Priority,
			Predicate: predicate,
			Category:  rawRule.Category,
			Enabled:   rawRule.Enabled,
		})
	}

	return rules, nil
}

func unmarshalKnownFields(data []byte, value any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(value)
}

func validateCategories(categories []Category) error {
	names := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		if category.Name == "" {
			return fmt.Errorf("category missing name")
		}
		if _, exists := names[category.Name]; exists {
			return fmt.Errorf("duplicate category %q", category.Name)
		}
		names[category.Name] = struct{}{}

		switch category.Kind {
		case KindIncome, KindExpense, KindTransfer:
		case "":
			return fmt.Errorf("category %q missing kind", category.Name)
		default:
			return fmt.Errorf("category %q has invalid kind %q", category.Name, category.Kind)
		}
	}

	for _, category := range categories {
		if category.Parent == "" {
			continue
		}
		if _, exists := names[category.Parent]; !exists {
			return fmt.Errorf("category %q references unknown parent %q", category.Name, category.Parent)
		}
	}

	return nil
}

func convertPredicate(ruleName string, raw predicateYAML) (Predicate, error) {
	amountMin, err := parseOptionalMoney(ruleName, "amount_min", raw.AmountMin)
	if err != nil {
		return Predicate{}, err
	}
	amountMax, err := parseOptionalMoney(ruleName, "amount_max", raw.AmountMax)
	if err != nil {
		return Predicate{}, err
	}
	if err := validateRegex(ruleName, "description_matches", raw.DescriptionMatches); err != nil {
		return Predicate{}, err
	}
	if err := validateRegex(ruleName, "merchant_matches", raw.MerchantMatches); err != nil {
		return Predicate{}, err
	}

	return Predicate{
		Direction:          raw.Direction,
		AmountMin:          amountMin,
		AmountMax:          amountMax,
		DescriptionMatches: raw.DescriptionMatches,
		MerchantIn:         raw.MerchantIn,
		MerchantMatches:    raw.MerchantMatches,
		AccountIn:          raw.AccountIn,
		AkahuCategory:      raw.AkahuCategory,
	}, nil
}

func parseOptionalMoney(ruleName, field string, value *string) (*domain.Money, error) {
	if value == nil {
		return nil, nil
	}
	money, err := domain.NewMoneyFromString(*value)
	if err != nil {
		return nil, fmt.Errorf("rule %q invalid %s %q: %w", ruleName, field, *value, err)
	}
	return &money, nil
}

func validateRegex(ruleName, field, expression string) error {
	if expression == "" {
		return nil
	}
	if _, err := regexp.Compile(expression); err != nil {
		return fmt.Errorf("rule %q invalid %s: %w", ruleName, field, err)
	}
	return nil
}
