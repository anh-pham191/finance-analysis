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

func TestSummaryCommandParsesPeriodAndFormat(t *testing.T) {
	freezeReportingNow(t)

	var out bytes.Buffer
	var gotOptions summaryOptions
	called := false
	summaryRunner = func(ctx context.Context, opts summaryOptions) error {
		called = true
		gotOptions = opts
		return nil
	}
	t.Cleanup(func() {
		summaryRunner = runSummary
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"summary", "--period", "2026-04", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute summary: %v", err)
	}
	if !called {
		t.Fatal("summary runner was not called")
	}
	if gotOptions.Format != render.FormatJSON {
		t.Fatalf("format = %q, want json", gotOptions.Format)
	}

	want := april2026Range(t)
	assertRange(t, gotOptions.Period, want)
}

func TestSummaryCommandRejectsInvalidFormat(t *testing.T) {
	freezeReportingNow(t)

	var out bytes.Buffer
	summaryRunner = func(ctx context.Context, opts summaryOptions) error {
		t.Fatal("summary runner should not be called")
		return nil
	}
	t.Cleanup(func() {
		summaryRunner = runSummary
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"summary", "--period", "2026-04", "--format", "yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("summary accepted invalid format")
	}
	if !strings.Contains(err.Error(), `invalid format "yaml"`) {
		t.Fatalf("error = %q, want invalid format", err.Error())
	}
}

func april2026Range(t *testing.T) domain.Range {
	t.Helper()
	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, loc),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
	}
}

func assertRange(t *testing.T, got, want domain.Range) {
	t.Helper()
	if !got.From.Equal(want.From) || !got.To.Equal(want.To) {
		t.Fatalf("range = [%v, %v), want [%v, %v)", got.From, got.To, want.From, want.To)
	}
	if got.From.Location().String() != want.From.Location().String() {
		t.Fatalf("range location = %q, want %q", got.From.Location(), want.From.Location())
	}
}

func freezeReportingNow(t *testing.T) {
	t.Helper()
	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	reportingNow = func() time.Time {
		return time.Date(2026, 5, 6, 12, 0, 0, 0, loc)
	}
	t.Cleanup(func() {
		reportingNow = time.Now
	})
}
