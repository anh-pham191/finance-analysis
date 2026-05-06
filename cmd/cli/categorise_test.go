package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCategoriseCommandPassesDefaultPaths(t *testing.T) {
	writeCategoriseConfig(t, "config/categories.yaml", "config/rules.yaml")

	var out bytes.Buffer
	called := false
	categoriseRunner = func(ctx context.Context, opts categoriseOptions) error {
		called = true
		if opts.CategoriesPath != "config/categories.yaml" {
			t.Fatalf("categories path = %q, want default", opts.CategoriesPath)
		}
		if opts.RulesPath != "config/rules.yaml" {
			t.Fatalf("rules path = %q, want default", opts.RulesPath)
		}
		return nil
	}
	t.Cleanup(func() {
		categoriseRunner = runCategorise
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"categorise"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute categorise: %v", err)
	}
	if !called {
		t.Fatal("categorise runner was not called")
	}
}

func TestCategoriseCommandPassesCustomPaths(t *testing.T) {
	writeCategoriseConfig(t, "fixtures/categories.yaml", "fixtures/rules.yaml")

	var out bytes.Buffer
	categoriseRunner = func(ctx context.Context, opts categoriseOptions) error {
		if opts.CategoriesPath != "fixtures/categories.yaml" {
			t.Fatalf("categories path = %q, want custom", opts.CategoriesPath)
		}
		if opts.RulesPath != "fixtures/rules.yaml" {
			t.Fatalf("rules path = %q, want custom", opts.RulesPath)
		}
		return nil
	}
	t.Cleanup(func() {
		categoriseRunner = runCategorise
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{
		"categorise",
		"--categories", "fixtures/categories.yaml",
		"--rules", "fixtures/rules.yaml",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute categorise: %v", err)
	}
}

func TestCategoriseCommandSurfacesLoaderError(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll("config", 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile("config/categories.yaml", []byte(`
- name: Uncategorised
  kind: unknown
`), 0o644); err != nil {
		t.Fatalf("write categories: %v", err)
	}

	var out bytes.Buffer
	categoriseRunner = func(ctx context.Context, opts categoriseOptions) error {
		t.Fatal("categorise runner should not be called when loading fails")
		return nil
	}
	t.Cleanup(func() {
		categoriseRunner = runCategorise
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"categorise"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("categorise succeeded with invalid categories config")
	}
	if !strings.Contains(err.Error(), `load categories "config/categories.yaml"`) {
		t.Fatalf("error = %q, want categories path", err.Error())
	}
	if !strings.Contains(err.Error(), "invalid kind") {
		t.Fatalf("error = %q, want loader validation message", err.Error())
	}
}

func TestCategoriseCommandCallsRunnerWithParsedConfig(t *testing.T) {
	writeCategoriseConfig(t, "config/categories.yaml", "config/rules.yaml")

	var out bytes.Buffer
	categoriseRunner = func(ctx context.Context, opts categoriseOptions) error {
		if len(opts.Config.Categories) != 2 {
			t.Fatalf("loaded categories = %d, want 2", len(opts.Config.Categories))
		}
		if opts.Config.Categories[0].Name != "Uncategorised" {
			t.Fatalf("first category = %q, want Uncategorised", opts.Config.Categories[0].Name)
		}
		if len(opts.Config.Rules) != 1 {
			t.Fatalf("loaded rules = %d, want 1", len(opts.Config.Rules))
		}
		rule := opts.Config.Rules[0]
		if rule.Name != "Groceries" {
			t.Fatalf("rule name = %q, want Groceries", rule.Name)
		}
		if rule.Category != "Food/Groceries" {
			t.Fatalf("rule category = %q, want Food/Groceries", rule.Category)
		}
		if len(rule.Predicate.MerchantIn) != 1 || rule.Predicate.MerchantIn[0] != "Countdown" {
			t.Fatalf("rule merchant predicate = %v, want Countdown", rule.Predicate.MerchantIn)
		}
		return errors.New("runner called")
	}
	t.Cleanup(func() {
		categoriseRunner = runCategorise
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"categorise"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("categorise succeeded, want runner error")
	}
	if !strings.Contains(err.Error(), "runner called") {
		t.Fatalf("error = %q, want runner error", err.Error())
	}
}

func writeCategoriseConfig(t *testing.T, categoriesPath, rulesPath string) {
	t.Helper()
	chdirTemp(t)

	writeFile(t, categoriesPath, `
- name: Uncategorised
  kind: expense
- name: Food/Groceries
  kind: expense
`)
	writeFile(t, rulesPath, `
- name: Groceries
  priority: 10
  when:
    merchant_in: ["Countdown"]
  category: Food/Groceries
`)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func chdirTemp(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}
