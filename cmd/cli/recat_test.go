package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRecatCommandLoadsCategoriesAndCallsRunner(t *testing.T) {
	writeRecatCategories(t, "fixtures/categories.yaml")

	var out bytes.Buffer
	called := false
	recatRunner = func(ctx context.Context, opts recatOptions) error {
		called = true
		if opts.TxnID != "txn-1" {
			t.Fatalf("txn ID = %q, want txn-1", opts.TxnID)
		}
		if opts.CategoryName != "Food/Groceries" {
			t.Fatalf("category name = %q, want Food/Groceries", opts.CategoryName)
		}
		if opts.CategoriesPath != "fixtures/categories.yaml" {
			t.Fatalf("categories path = %q, want custom path", opts.CategoriesPath)
		}
		if len(opts.Categories) != 2 {
			t.Fatalf("loaded categories = %d, want 2", len(opts.Categories))
		}
		return nil
	}
	t.Cleanup(func() {
		recatRunner = runRecat
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"recat", "txn-1", "Food/Groceries", "--categories", "fixtures/categories.yaml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute recat: %v", err)
	}
	if !called {
		t.Fatal("recat runner was not called")
	}
}

func TestRecatCommandRejectsUnknownCategoryWithCandidates(t *testing.T) {
	writeRecatCategories(t, "fixtures/categories.yaml")

	var out bytes.Buffer
	recatRunner = func(ctx context.Context, opts recatOptions) error {
		t.Fatal("recat runner should not be called for unknown category")
		return nil
	}
	t.Cleanup(func() {
		recatRunner = runRecat
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"recat", "txn-1", "Missing", "--categories", "fixtures/categories.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("recat succeeded with unknown category")
	}
	if !strings.Contains(err.Error(), `unknown category "Missing"`) {
		t.Fatalf("error = %q, want unknown category", err.Error())
	}
	if !strings.Contains(err.Error(), "Food/Groceries") || !strings.Contains(err.Error(), "Uncategorised") {
		t.Fatalf("error = %q, want candidate category names", err.Error())
	}
}

func TestRecatCommandRequiresTxnAndCategory(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"recat"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("recat succeeded with missing args")
	}
}

func writeRecatCategories(t *testing.T, categoriesPath string) {
	t.Helper()
	chdirTemp(t)
	writeFile(t, categoriesPath, `
- name: Food/Groceries
  kind: expense
- name: Uncategorised
  kind: expense
`)
}
