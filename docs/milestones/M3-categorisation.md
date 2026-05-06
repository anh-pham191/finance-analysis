# M3 — Categorisation

> Spec reference: §6 Categorisation. Builds on M2.

## Goal

Apply user-authored rules from `config/rules.yaml` to all ingested transactions, producing a `category_assignments` row per transaction. Manual overrides via `finance recat` always win.

## Scope

### In
- `internal/categorise/Rule` type matching the YAML predicate fields.
- YAML loader: `LoadRules(path) ([]Rule, error)`. Validates priorities are unique-or-stable-ordered, regex compiles, referenced categories exist (auto-create category rows on apply).
- `internal/categorise/Apply(txn, rules) (*Assignment, bool)` — pure function, returns first match.
- `internal/categorise/Categorise(ctx, repo, rules)` — use case that walks all txns lacking a `MANUAL` assignment and writes/updates `category_assignments` with `source = RULE`.
- `finance categorise` command — runs the use case.
- `finance recat <txn-id> <category>` command — writes a `MANUAL` assignment, creating the category if needed.
- `finance uncategorised` command — lists transactions whose assignment is `Uncategorised`.
- Repository extensions: get/upsert categories, get/upsert assignments, list uncategorised.

### Out
- Reporting (M4).
- Rule OR-semantics, rule groups, rule expiry (revisit in spec §14 open questions).

## Prerequisites

- M2 complete and green; transactions populated.

## Deliverables

- [ ] Example `config/rules.yaml` shipped (in `config/rules.example.yaml`).
- [ ] `finance categorise` is idempotent — running twice produces no diff.
- [ ] `finance recat` overrides a rule-based assignment and survives subsequent `finance categorise` runs.
- [ ] Re-running `finance categorise` after editing `rules.yaml` correctly re-categorises any non-manual txns.
- [ ] `finance uncategorised` lists exactly the txns where no rule matched.
- [ ] Unit tests for the rule engine cover every predicate field independently and in combination.

## Architecture context

The rule engine is a pure function. No DB access, no logging, no file I/O. The use case orchestrates: load rules → fetch txns → apply rule → write assignment. This separation lets us table-test the engine exhaustively with cheap in-memory fixtures.

Categories are auto-created on first reference by name. The `name` is the natural key and uses `/` for hierarchy (e.g. `Food/Groceries`). Parent categories are auto-created too.

`source` precedence on read: `MANUAL > RULE > AKAHU`. The `Apply` function never produces `AKAHU`; that source is reserved for a possible future fallback that uses Akahu's own enrichment.

## Test plan (TDD)

1. `internal/categorise/rule_test.go` — table-driven tests, one per predicate field.
2. `internal/categorise/loader_test.go` — valid YAML loads; invalid regex errors with line context; missing category name errors.
3. `internal/categorise/categorise_test.go` (in-memory repo):
   - First run assigns all matching txns; unmatched get `Uncategorised`.
   - Second run is a no-op (no writes if already correct).
   - Editing a rule and re-running updates non-manual assignments.
   - `MANUAL` assignments are not touched.
4. `cmd/cli/recat_test.go` — `finance recat` writes `MANUAL` and survives `categorise`.
5. `cmd/cli/uncategorised_test.go` — only returns rows whose category is `Uncategorised`.

## Acceptance criteria

- All deliverables ticked.
- A real-data smoke test: run against the user's actual transactions, inspect `finance uncategorised` output, write a rule, re-run, verify shrinkage.

## Files an agent will touch

```
config/rules.example.yaml
internal/categorise/{rule.go, rule_test.go, loader.go, loader_test.go,
                    categorise.go, categorise_test.go}
internal/storage/postgres/repository.go (extend)
internal/storage/postgres/queries.sql (extend)
cmd/cli/{categorise.go, recat.go, uncategorised.go, *_test.go}
```

## Pitfalls

- Regex compilation failures must be surfaced at load time, not at apply time. Fail fast.
- Idempotency: don't `INSERT ... ON CONFLICT` blindly with a new `assigned_at`. Only update when category actually changes.
- Auto-creating categories: do it in a transaction with a unique constraint to avoid races (currently single-user so unlikely, but cheap to do right).
- A rule that no longer matches a previously-categorised txn must move it back to `Uncategorised`, not leave the stale category.
