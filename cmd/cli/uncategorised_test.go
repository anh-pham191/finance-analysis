package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestUncategorisedCommandListsOnlyUncategorisedTransactionIDs(t *testing.T) {
	writeUncategorisedCategories(t, "config/categories.yaml")

	var out bytes.Buffer
	called := false
	uncategorisedRunner = func(ctx context.Context, opts uncategorisedOptions) ([]domain.CategoryAssignment, error) {
		called = true
		if opts.CategoriesPath != "config/categories.yaml" {
			t.Fatalf("categories path = %q, want default", opts.CategoriesPath)
		}
		if !categoryDeclared(opts.Categories, "Uncategorised") {
			t.Fatal("loaded categories did not include Uncategorised")
		}
		return []domain.CategoryAssignment{
			{TxnID: "txn-uncategorised-1"},
			{TxnID: "txn-uncategorised-2"},
		}, nil
	}
	t.Cleanup(func() {
		uncategorisedRunner = runUncategorised
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"uncategorised"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute uncategorised: %v", err)
	}
	if !called {
		t.Fatal("uncategorised runner was not called")
	}

	got := out.String()
	if got != "txn-uncategorised-1\ntxn-uncategorised-2\n" {
		t.Fatalf("output = %q, want transaction IDs only", got)
	}
	if strings.Contains(got, "txn-categorised") {
		t.Fatalf("output = %q, includes categorised transaction", got)
	}
}

func TestUncategorisedCommandOutputAvoidsSensitiveData(t *testing.T) {
	writeUncategorisedCategories(t, "config/categories.yaml")
	t.Setenv("AKAHU_APP_TOKEN", "sensitive-app-token")
	t.Setenv("AKAHU_USER_TOKEN", "sensitive-user-token")

	var out bytes.Buffer
	uncategorisedRunner = func(ctx context.Context, opts uncategorisedOptions) ([]domain.CategoryAssignment, error) {
		return []domain.CategoryAssignment{{TxnID: "txn-safe-id"}}, nil
	}
	t.Cleanup(func() {
		uncategorisedRunner = runUncategorised
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"uncategorised"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute uncategorised: %v", err)
	}

	got := out.String()
	if got != "txn-safe-id\n" {
		t.Fatalf("output = %q, want transaction ID only", got)
	}
	for _, sensitive := range []string{
		"sensitive-app-token",
		"sensitive-user-token",
		"raw_json",
		`{"raw":true}`,
		"Flat white",
		"Coffee Merchant",
		"12.34",
	} {
		if strings.Contains(got, sensitive) {
			t.Fatalf("output = %q, contains sensitive fragment %q", got, sensitive)
		}
	}
}

func TestUncategorisedCommandEmptyListPrintsMessage(t *testing.T) {
	writeUncategorisedCategories(t, "config/categories.yaml")

	var out bytes.Buffer
	uncategorisedRunner = func(ctx context.Context, opts uncategorisedOptions) ([]domain.CategoryAssignment, error) {
		return nil, nil
	}
	t.Cleanup(func() {
		uncategorisedRunner = runUncategorised
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"uncategorised"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute uncategorised: %v", err)
	}
	if got := out.String(); got != "No uncategorised transactions.\n" {
		t.Fatalf("output = %q, want clear empty-list message", got)
	}
}

func writeUncategorisedCategories(t *testing.T, categoriesPath string) {
	t.Helper()
	chdirTemp(t)
	writeFile(t, categoriesPath, `
- name: Food/Groceries
  kind: expense
- name: Uncategorised
  kind: expense
`)
}
