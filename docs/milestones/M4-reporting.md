# M4 — Reporting MVP

> Spec reference: §7 Reporting. Builds on M3.

## Goal

Three reports — summary, comparison, drill-down — usable from the CLI in multiple output formats. All report logic is pure; renderers are interchangeable.

## Scope

### In
- `internal/domain/Period` type:
  - Parses `YYYY-MM`, `YYYY-Www`, `YYYY`, explicit `--from/--to`, and the relative shorthands `this-week|last-week|this-month|last-month|this-year|last-year`.
  - Resolves to a `[from, to)` half-open interval in `Pacific/Auckland`.
- `internal/report/Summary(repo, period) (SummaryResult, error)` — totals + per-category breakdown.
- `internal/report/Compare(repo, a, b Period) (CompareResult, error)` — per-category A/B/Δ/Δ%.
- `internal/report/Transactions(repo, filter) ([]TxnRow, error)` — drill-down with sorts and filters.
- `internal/render/Renderer` interface with `Table`, `CSV`, `JSON`, `Markdown` implementations.
- CLI commands `finance summary`, `finance compare`, `finance txns` with `--format` flag.
- CLI convenience flags on `compare`: `--wow`, `--mom`, `--yoy`.

### Out
- Trends (C), budgets (E), cashflow (F) — these are post-MVP per spec §2.
- Charts. Table only.

## Prerequisites

- M3 complete; transactions are categorised.

## Deliverables

- [ ] `finance summary --period 2026-04` works end-to-end.
- [ ] `finance compare --mom` produces month-on-month deltas.
- [ ] `finance txns --category Food/Groceries --period last-month --sort amount` lists the top spends.
- [ ] All three commands support `--format=table|csv|json|md`.
- [ ] `Period` parsing has exhaustive unit tests for every accepted form, including edge cases (year boundaries, ISO week 53, daylight saving).
- [ ] Pure-function tests for each report against fixture data.

## Architecture context

Reports take a `Repository` (or a narrower read-only interface — prefer narrower) and return DTOs. The CLI does not format numbers or sort rows — that's done in the report or renderer. This keeps a future HTTP API a thin pass-through.

`Period` has one resolver. WoW/MoM/YoY shorthand flags simply construct two `Period` values and call `Compare`. Don't add parallel logic.

Renderers receive typed result structs. `JSON` renderer produces the same shape the future HTTP API will return.

## Test plan (TDD)

1. `internal/domain/period_test.go` — table-driven across every accepted form. Fix `time.Now()` via clock injection.
2. `internal/report/summary_test.go` — fixture txns with known categories sum correctly.
3. `internal/report/compare_test.go` — A/B totals, Δ, Δ%; sort order; `--top N`.
4. `internal/report/transactions_test.go` — every filter and sort.
5. `internal/render/*_test.go` — golden-file tests for each format.
6. `cmd/cli/{summary,compare,txns}_test.go` — end-to-end via testcontainers Postgres + seeded fixture data.

## Acceptance criteria

- All deliverables ticked.
- Real-data smoke test against the user's transactions: produces sensible numbers; `--mom` matches manual spot-check.
- JSON output validates against a documented schema (write the schema in `docs/api/reporting-dtos.md` even though no API yet).

## Files an agent will touch

```
internal/domain/period.go, period_test.go
internal/report/{summary.go, compare.go, transactions.go, types.go, *_test.go}
internal/render/{table.go, csv.go, json.go, markdown.go, renderer.go, *_test.go}
cmd/cli/{summary.go, compare.go, txns.go, *_test.go}
docs/api/reporting-dtos.md
```

## Pitfalls

- Daylight saving: NZ shifts in late September/early April. ISO week and month boundaries must be computed in the configured timezone before being converted to UTC for query.
- Δ% when A is zero: define explicitly (return `nil` / `null` in JSON, `—` in table; document this in DTO schema).
- Don't sort in SQL only; sort in the report function so output is identical regardless of repo implementation.
- `--from/--to` are inclusive in user input, half-open internally. Convert at the CLI boundary.
