package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/render"
)

func TestTxnsCommandParsesFilterAndFormat(t *testing.T) {
	freezeReportingNow(t)

	var out bytes.Buffer
	var gotOptions txnsOptions
	called := false
	txnsRunner = func(ctx context.Context, opts txnsOptions) error {
		called = true
		gotOptions = opts
		return nil
	}
	t.Cleanup(func() {
		txnsRunner = runTxns
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{
		"txns",
		"--category", "Food/Groceries",
		"--period", "last-month",
		"--limit", "50",
		"--sort", "amount",
		"--format", "csv",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute txns: %v", err)
	}
	if !called {
		t.Fatal("txns runner was not called")
	}
	if gotOptions.CategoryName != "Food/Groceries" {
		t.Fatalf("category = %q, want Food/Groceries", gotOptions.CategoryName)
	}
	if gotOptions.Limit != 50 {
		t.Fatalf("limit = %d, want 50", gotOptions.Limit)
	}
	if gotOptions.Sort != "amount" {
		t.Fatalf("sort = %q, want amount", gotOptions.Sort)
	}
	if gotOptions.Format != render.FormatCSV {
		t.Fatalf("format = %q, want csv", gotOptions.Format)
	}

	assertRange(t, gotOptions.Period, april2026Range(t))
}

func TestTxnsCommandRejectsNegativeLimit(t *testing.T) {
	freezeReportingNow(t)

	err := executeInvalidTxnsCommand(t, "txns", "--period", "this-month", "--limit", "-1")
	if !strings.Contains(err.Error(), "--limit must be >= 0") {
		t.Fatalf("error = %q, want --limit validation message", err.Error())
	}
}

func TestTxnsCommandRejectsNegativeOffset(t *testing.T) {
	freezeReportingNow(t)

	err := executeInvalidTxnsCommand(t, "txns", "--period", "this-month", "--offset", "-1")
	if !strings.Contains(err.Error(), "--offset must be >= 0") {
		t.Fatalf("error = %q, want --offset validation message", err.Error())
	}
}

func executeInvalidTxnsCommand(t *testing.T, args ...string) error {
	t.Helper()

	var out bytes.Buffer
	txnsRunner = func(ctx context.Context, opts txnsOptions) error {
		t.Fatal("txns runner should not be called")
		return nil
	}
	t.Cleanup(func() {
		txnsRunner = runTxns
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs(args)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("txns accepted invalid numeric flag")
	}
	return err
}
