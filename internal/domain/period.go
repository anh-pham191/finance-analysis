package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	yearPeriodRegexp  = regexp.MustCompile(`^\d{4}$`)
	monthPeriodRegexp = regexp.MustCompile(`^(\d{4})-(0[1-9]|1[0-2])$`)
	weekPeriodRegexp  = regexp.MustCompile(`^(\d{4})-W(0[1-9]|[1-4][0-9]|5[0-3])$`)
)

type Range struct {
	From time.Time
	To   time.Time
}

type Period struct {
	value        string
	explicitFrom *time.Time
	explicitTo   *time.Time
}

func ParsePeriod(value string) (Period, error) {
	value = strings.TrimSpace(value)
	if isRelativePeriod(value) ||
		yearPeriodRegexp.MatchString(value) ||
		monthPeriodRegexp.MatchString(value) ||
		weekPeriodRegexp.MatchString(value) {
		return Period{value: value}, nil
	}
	return Period{}, fmt.Errorf("invalid period %q: want YYYY, YYYY-MM, YYYY-Www, or a relative period", value)
}

func ExplicitPeriod(from, to time.Time) Period {
	return Period{
		explicitFrom: &from,
		explicitTo:   &to,
	}
}

func (p Period) Resolve(loc *time.Location, now time.Time) (Range, error) {
	if loc == nil {
		return Range{}, fmt.Errorf("resolve period: location is nil")
	}
	if p.explicitFrom != nil && p.explicitTo != nil {
		fromYear, fromMonth, fromDay := p.explicitFrom.Date()
		toYear, toMonth, toDay := p.explicitTo.Date()
		from := time.Date(fromYear, fromMonth, fromDay, 0, 0, 0, 0, loc)
		to := time.Date(toYear, toMonth, toDay, 0, 0, 0, 0, loc).AddDate(0, 0, 1)
		if to.Before(from) || to.Equal(from) {
			return Range{}, fmt.Errorf("resolve explicit period: to date must be on or after from date")
		}
		return Range{From: from, To: to}, nil
	}

	if p.value == "" {
		return Range{}, fmt.Errorf("resolve period: invalid empty period")
	}

	now = now.In(loc)
	switch p.value {
	case "this-week":
		from := startOfISOWeek(now, loc)
		return Range{From: from, To: from.AddDate(0, 0, 7)}, nil
	case "last-week":
		to := startOfISOWeek(now, loc)
		return Range{From: to.AddDate(0, 0, -7), To: to}, nil
	case "this-month":
		from := startOfMonth(now.Year(), now.Month(), loc)
		return Range{From: from, To: from.AddDate(0, 1, 0)}, nil
	case "last-month":
		to := startOfMonth(now.Year(), now.Month(), loc)
		return Range{From: to.AddDate(0, -1, 0), To: to}, nil
	case "this-year":
		from := startOfYear(now.Year(), loc)
		return Range{From: from, To: from.AddDate(1, 0, 0)}, nil
	case "last-year":
		to := startOfYear(now.Year(), loc)
		return Range{From: to.AddDate(-1, 0, 0), To: to}, nil
	}

	if yearPeriodRegexp.MatchString(p.value) {
		year, err := strconv.Atoi(p.value)
		if err != nil {
			return Range{}, fmt.Errorf("resolve year period %q: %w", p.value, err)
		}
		from := startOfYear(year, loc)
		return Range{From: from, To: from.AddDate(1, 0, 0)}, nil
	}

	if matches := monthPeriodRegexp.FindStringSubmatch(p.value); matches != nil {
		year, err := strconv.Atoi(matches[1])
		if err != nil {
			return Range{}, fmt.Errorf("resolve month period %q: %w", p.value, err)
		}
		monthNumber, err := strconv.Atoi(matches[2])
		if err != nil {
			return Range{}, fmt.Errorf("resolve month period %q: %w", p.value, err)
		}
		from := startOfMonth(year, time.Month(monthNumber), loc)
		return Range{From: from, To: from.AddDate(0, 1, 0)}, nil
	}

	if matches := weekPeriodRegexp.FindStringSubmatch(p.value); matches != nil {
		year, err := strconv.Atoi(matches[1])
		if err != nil {
			return Range{}, fmt.Errorf("resolve ISO week period %q: %w", p.value, err)
		}
		week, err := strconv.Atoi(matches[2])
		if err != nil {
			return Range{}, fmt.Errorf("resolve ISO week period %q: %w", p.value, err)
		}
		from := startOfISOWeekYear(year, loc).AddDate(0, 0, (week-1)*7)
		gotYear, gotWeek := from.ISOWeek()
		if gotYear != year || gotWeek != week {
			return Range{}, fmt.Errorf("invalid ISO week period %q", p.value)
		}
		return Range{From: from, To: from.AddDate(0, 0, 7)}, nil
	}

	return Range{}, fmt.Errorf("resolve period: invalid period %q", p.value)
}

func isRelativePeriod(value string) bool {
	switch value {
	case "this-week", "last-week", "this-month", "last-month", "this-year", "last-year":
		return true
	default:
		return false
	}
}

func startOfISOWeek(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1-weekday)
}

func startOfISOWeekYear(year int, loc *time.Location) time.Time {
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, loc)
	return startOfISOWeek(jan4, loc)
}

func startOfMonth(year int, month time.Month, loc *time.Location) time.Time {
	return time.Date(year, month, 1, 0, 0, 0, 0, loc)
}

func startOfYear(year int, loc *time.Location) time.Time {
	return time.Date(year, 1, 1, 0, 0, 0, 0, loc)
}
