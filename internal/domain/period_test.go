package domain

import (
	"strings"
	"testing"
	"time"
)

func TestPeriodMonthResolvesInLocation(t *testing.T) {
	t.Parallel()

	loc := aucklandLocation(t)
	period, err := ParsePeriod("2026-04")
	if err != nil {
		t.Fatalf("ParsePeriod returned error: %v", err)
	}

	got, err := period.Resolve(loc, time.Date(2026, 5, 6, 12, 0, 0, 0, loc))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	assertRange(t, got,
		time.Date(2026, 4, 1, 0, 0, 0, 0, loc),
		time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
	)
}

func TestPeriodYearResolvesCalendarYear(t *testing.T) {
	t.Parallel()

	loc := aucklandLocation(t)
	period, err := ParsePeriod("2026")
	if err != nil {
		t.Fatalf("ParsePeriod returned error: %v", err)
	}

	got, err := period.Resolve(loc, time.Date(2026, 5, 6, 12, 0, 0, 0, loc))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	assertRange(t, got,
		time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
		time.Date(2027, 1, 1, 0, 0, 0, 0, loc),
	)
}

func TestPeriodISOWeek53ResolvesWhenValid(t *testing.T) {
	t.Parallel()

	loc := aucklandLocation(t)
	period, err := ParsePeriod("2026-W53")
	if err != nil {
		t.Fatalf("ParsePeriod returned error: %v", err)
	}

	got, err := period.Resolve(loc, time.Date(2026, 5, 6, 12, 0, 0, 0, loc))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	assertRange(t, got,
		time.Date(2026, 12, 28, 0, 0, 0, 0, loc),
		time.Date(2027, 1, 4, 0, 0, 0, 0, loc),
	)
}

func TestPeriodISOWeek53ErrorsWhenInvalid(t *testing.T) {
	t.Parallel()

	loc := aucklandLocation(t)
	period, err := ParsePeriod("2021-W53")
	if err == nil {
		_, err = period.Resolve(loc, time.Date(2026, 5, 6, 12, 0, 0, 0, loc))
	}
	if err == nil {
		t.Fatal("ParsePeriod/Resolve returned nil error for invalid ISO week")
	}
}

func TestPeriodRelativeFormsResolveFromFixedNow(t *testing.T) {
	t.Parallel()

	loc := aucklandLocation(t)
	now := time.Date(2026, 5, 6, 12, 30, 0, 0, loc)

	tests := []struct {
		name  string
		value string
		from  time.Time
		to    time.Time
	}{
		{
			name:  "this week",
			value: "this-week",
			from:  time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			to:    time.Date(2026, 5, 11, 0, 0, 0, 0, loc),
		},
		{
			name:  "last week",
			value: "last-week",
			from:  time.Date(2026, 4, 27, 0, 0, 0, 0, loc),
			to:    time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
		},
		{
			name:  "this month",
			value: "this-month",
			from:  time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
			to:    time.Date(2026, 6, 1, 0, 0, 0, 0, loc),
		},
		{
			name:  "last month",
			value: "last-month",
			from:  time.Date(2026, 4, 1, 0, 0, 0, 0, loc),
			to:    time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
		},
		{
			name:  "this year",
			value: "this-year",
			from:  time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
			to:    time.Date(2027, 1, 1, 0, 0, 0, 0, loc),
		},
		{
			name:  "last year",
			value: "last-year",
			from:  time.Date(2025, 1, 1, 0, 0, 0, 0, loc),
			to:    time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			period, err := ParsePeriod(tt.value)
			if err != nil {
				t.Fatalf("ParsePeriod returned error: %v", err)
			}

			got, err := period.Resolve(loc, now)
			if err != nil {
				t.Fatalf("Resolve returned error: %v", err)
			}

			assertRange(t, got, tt.from, tt.to)
		})
	}
}

func TestPeriodExplicitRangeTreatsToDateAsInclusive(t *testing.T) {
	t.Parallel()

	loc := aucklandLocation(t)
	period := ExplicitPeriod(
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
	)

	got, err := period.Resolve(loc, time.Date(2026, 5, 6, 12, 0, 0, 0, loc))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	assertRange(t, got,
		time.Date(2026, 4, 1, 0, 0, 0, 0, loc),
		time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
	)
}

func TestPeriodMonthBoundariesAreWallClockAnchoredAcrossDST(t *testing.T) {
	t.Parallel()

	loc := aucklandLocation(t)

	tests := []struct {
		name  string
		value string
		from  time.Time
		to    time.Time
	}{
		{
			name:  "DST ends in April",
			value: "2026-04",
			from:  time.Date(2026, 4, 1, 0, 0, 0, 0, loc),
			to:    time.Date(2026, 5, 1, 0, 0, 0, 0, loc),
		},
		{
			name:  "DST starts in September",
			value: "2026-09",
			from:  time.Date(2026, 9, 1, 0, 0, 0, 0, loc),
			to:    time.Date(2026, 10, 1, 0, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			period, err := ParsePeriod(tt.value)
			if err != nil {
				t.Fatalf("ParsePeriod returned error: %v", err)
			}

			got, err := period.Resolve(loc, time.Date(2026, 5, 6, 12, 0, 0, 0, loc))
			if err != nil {
				t.Fatalf("Resolve returned error: %v", err)
			}

			assertRange(t, got, tt.from, tt.to)
		})
	}
}

func TestPeriodInvalidStringErrorsClearly(t *testing.T) {
	t.Parallel()

	_, err := ParsePeriod("April 2026")
	if err == nil {
		t.Fatal("ParsePeriod returned nil error")
	}
	if !strings.Contains(err.Error(), "invalid period") {
		t.Fatalf("ParsePeriod error = %q, want invalid period context", err)
	}
}

func TestPeriodResolveNilLocationErrorsClearly(t *testing.T) {
	t.Parallel()

	period, err := ParsePeriod("2026-04")
	if err != nil {
		t.Fatalf("ParsePeriod returned error: %v", err)
	}

	_, err = period.Resolve(nil, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("Resolve returned nil error")
	}
	if !strings.Contains(err.Error(), "location") {
		t.Fatalf("Resolve error = %q, want location context", err)
	}
}

func aucklandLocation(t *testing.T) *time.Location {
	t.Helper()

	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}
	return loc
}

func assertRange(t *testing.T, got Range, wantFrom, wantTo time.Time) {
	t.Helper()

	if !got.From.Equal(wantFrom) {
		t.Fatalf("Range.From = %v, want %v", got.From, wantFrom)
	}
	if got.From.Location() != wantFrom.Location() {
		t.Fatalf("Range.From location = %v, want %v", got.From.Location(), wantFrom.Location())
	}
	if !got.To.Equal(wantTo) {
		t.Fatalf("Range.To = %v, want %v", got.To, wantTo)
	}
	if got.To.Location() != wantTo.Location() {
		t.Fatalf("Range.To location = %v, want %v", got.To.Location(), wantTo.Location())
	}
}
