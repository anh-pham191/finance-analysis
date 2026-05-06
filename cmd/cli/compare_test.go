package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/render"
)

func TestCompareCommandResolvesMonthOverMonth(t *testing.T) {
	freezeReportingNow(t)

	gotOptions := executeCompareCommand(t, "compare", "--mom", "--format", "md")

	if gotOptions.Format != render.FormatMarkdown {
		t.Fatalf("format = %q, want md", gotOptions.Format)
	}
	assertRange(t, gotOptions.A, rangeInAuckland(t, 2026, 3, 1, 2026, 4, 1))
	assertRange(t, gotOptions.B, rangeInAuckland(t, 2026, 4, 1, 2026, 5, 1))
}

func TestCompareCommandResolvesWeekOverWeek(t *testing.T) {
	freezeReportingNow(t)

	gotOptions := executeCompareCommand(t, "compare", "--wow")

	assertRange(t, gotOptions.A, rangeInAuckland(t, 2026, 4, 20, 2026, 4, 27))
	assertRange(t, gotOptions.B, rangeInAuckland(t, 2026, 4, 27, 2026, 5, 4))
}

func TestCompareCommandResolvesYearOverYear(t *testing.T) {
	freezeReportingNow(t)

	gotOptions := executeCompareCommand(t, "compare", "--yoy")

	assertRange(t, gotOptions.A, rangeInAuckland(t, 2025, 1, 1, 2026, 1, 1))
	assertRange(t, gotOptions.B, rangeInAuckland(t, 2026, 1, 1, 2027, 1, 1))
}

func TestCompareCommandRejectsNegativeTop(t *testing.T) {
	freezeReportingNow(t)

	var out bytes.Buffer
	compareRunner = func(ctx context.Context, opts compareOptions) error {
		t.Fatal("compare runner should not be called")
		return nil
	}
	t.Cleanup(func() {
		compareRunner = runCompare
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"compare", "--mom", "--top", "-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("compare accepted negative --top")
	}
	if !strings.Contains(err.Error(), "--top must be >= 0") {
		t.Fatalf("error = %q, want --top validation message", err.Error())
	}
}

func executeCompareCommand(t *testing.T, args ...string) compareOptions {
	t.Helper()

	var out bytes.Buffer
	var gotOptions compareOptions
	called := false
	compareRunner = func(ctx context.Context, opts compareOptions) error {
		called = true
		gotOptions = opts
		return nil
	}
	t.Cleanup(func() {
		compareRunner = runCompare
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute compare: %v", err)
	}
	if !called {
		t.Fatal("compare runner was not called")
	}
	return gotOptions
}

func rangeInAuckland(t *testing.T, fromYear int, fromMonth time.Month, fromDay int, toYear int, toMonth time.Month, toDay int) domain.Range {
	t.Helper()
	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return domain.Range{
		From: time.Date(fromYear, fromMonth, fromDay, 0, 0, 0, 0, loc),
		To:   time.Date(toYear, toMonth, toDay, 0, 0, 0, 0, loc),
	}
}
