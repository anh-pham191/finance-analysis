# M4 — Reporting MVP

> Spec reference: §7 Reporting. Builds on M3.

## Goal

Three reports — summary, comparison, drill-down — usable from the CLI in multiple output formats. Pure functions in `internal/report/`, `io.Writer`-based renderers in `internal/render/`, JSON-tagged DTOs ready for the M7 HTTP API. DST and ISO-week edge cases are explicitly tested.

## Scope

### In
- `internal/domain/Period.Resolve(loc *time.Location, now time.Time) Range` — accepts `YYYY-MM`, `YYYY-Www` (ISO 8601, Monday-start), `YYYY`, explicit `--from/--to` (interpreted as dates in `loc`), and relative forms `this-week|last-week|this-month|last-month|this-year|last-year`.
- `internal/report/`:
  - `Summary(ctx, userID, deps, period) (SummaryResult, error)` — totals + per-category, including `Uncategorised`.
  - `Compare(ctx, userID, deps, a, b Period) (CompareResult, error)` — per-category A/B/Δ/Δ%; sort by |Δ| desc; `--top N`. Δ% is `*float64` (nil/`null` when A==0).
  - `Transactions(ctx, userID, deps, filter TxnFilter) ([]TxnRow, error)` — pagination + sort done in SQL; tiebreaker `txn_id` asc.
- All result DTOs carry `json:"..."` (snake_case) tags from M4 — not added later.
- `internal/render/`:
  - `SummaryRenderer`, `CompareRenderer`, `TxnsRenderer` interfaces with `Render(w io.Writer, result T) error` shape.
  - Implementations: `Table`, `CSV`, `JSON`, `Markdown` for each. CLI selects via `--format`.
- CLI commands: `finance summary`, `finance compare`, `finance txns`. `compare` supports `--wow`, `--mom`, `--yoy` shorthand. `txns` supports `--limit` (default 100), `--offset`, `--sort`.
- Summary footer warning when `Uncategorised` total > 0.
- `docs/api/reporting-dtos.md` — schema doc for the three result types (drives M7 JSON shape).

### Out
- Trends, budgets, cashflow (post-MVP).
- Charts. Table only.

## Prerequisites

- M3 complete; transactions are categorised.

## Deliverables

- [ ] `finance summary --period 2026-04` works end-to-end.
- [ ] `finance compare --mom` produces month-on-month deltas.
- [ ] `finance txns --category Food/Groceries --period last-month --limit 50 --sort amount` lists top spends with stable tiebreaker.
- [ ] All three commands support `--format=table|csv|json|md`.
- [ ] Period parsing tested across:
  - DST start (last Sunday of September NZST→NZDT) and end (first Sunday of April).
  - ISO week 53 in years that have one; error for `2026-W53` (not a valid week in 2026).
  - Year boundary where ISO week 1 starts in the previous calendar year.
  - `--from`/`--to` half-open behaviour.
- [ ] JSON renderer output validates against `docs/api/reporting-dtos.md` schema.
- [ ] Cross-tenant test: each report scoped to caller's user; no leakage.

## Architecture context

`Period.Resolve` takes `*time.Location` as an argument — passed in from config — so `internal/domain` stays free of timezone hardcoding. Tests inject explicit locations.

Renderers take `io.Writer` so M7 can pass an `http.ResponseWriter` and the same renderer code works unchanged. Anything that calls `fmt.Println` directly is an M7-blocker waiting to happen.

Pagination and sorting belong in SQL because `txns` could return many rows. Reports above the repo are otherwise pure.

## Test plan (TDD)

1. `internal/domain/period_test.go` — table-driven, every accepted form, every edge case listed above. Fixed `now` and `loc` per case.
2. `internal/report/summary_test.go` — fixture txns with known categories sum correctly; `Uncategorised` line included.
3. `internal/report/compare_test.go` — A/B totals, Δ, Δ% (incl. nil when A==0), sort order, `--top N`.
4. `internal/report/transactions_test.go` — every filter; sort with tiebreaker; pagination edge (offset past end → empty).
5. `internal/render/*_test.go` — golden file per (renderer, result type) pair.
6. `cmd/cli/{summary,compare,txns}_test.go` — end-to-end via testcontainers Postgres + seeded data, verifying `--format` switching.

## Acceptance criteria

- All deliverables ticked.
- Real-data smoke test against the user's transactions: `--mom` numbers match a manual spot-check.
- JSON output stable across Go map iteration (use ordered serialisation).

## Files an agent will touch

```
internal/domain/period.go, period_test.go
internal/report/{summary.go, compare.go, transactions.go, types.go, *_test.go}
internal/render/{summary_table.go, summary_csv.go, summary_json.go, summary_md.go,
                 compare_*.go, txns_*.go, *_test.go}
internal/storage/postgres/txn.go (extend with paginated queries)
internal/storage/postgres/queries.sql (extend)
cmd/cli/{summary.go, compare.go, txns.go, *_test.go}
docs/api/reporting-dtos.md
```

## Pitfalls

- DST: NZ shifts late September / early April. Computing `[from, to)` in UTC and converting will give off-by-one-hour errors twice a year. Compute in `loc`, convert to UTC at the SQL boundary only.
- ISO week 53 is not always a valid week. `Period.Resolve("2026-W53")` must error, not silently return a wrong range.
- Δ% when A==0: emit `null` in JSON, `—` in table, document in DTO schema.
- Sort tiebreaker must be `txn_id` ascending to make output deterministic across runs.
- JSON tags from day one. Going back to add them later is a churn PR.
- Don't have renderers print to `os.Stdout` directly. `io.Writer` always.
