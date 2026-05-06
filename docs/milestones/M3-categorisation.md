# M3 — Categorisation

> Spec reference: §6 Categorisation. Builds on M2.

## Goal

Apply user-authored matching rules from `config/rules.yaml` against a user-declared taxonomy in `config/categories.yaml` to produce a `category_assignments` row per transaction. Manual overrides (`MANUAL`) always win and survive rule edits. Re-running is **convergent** — same inputs produce same outputs, with zero writes when nothing changed.

## Scope

### In
- `config/categories.example.yaml` — taxonomy template (per spec §6).
- `config/rules.example.yaml` — matching rules template.
- `internal/categorise/`:
  - `Category{Name, Kind, Parent}` and `Rule{Name, Priority, Predicate, Category}` types.
  - `LoadCategories(path)` and `LoadRules(path)` — validate kinds, parent references, regex compiles, and rule->category references at load time.
  - `Apply(txn, rules) (*Assignment, bool)` — pure first-match function. Tiebreaker: priority asc, then `name` asc.
  - `Categorise(ctx, userID, deps)` — use case:
    - Reconcile `categories` table: upsert from yaml by `(user_id, name)`; update kind/parent. **Never delete** (manual SQL only — protects `category_id` FKs).
    - Reconcile `rules` table: upsert from yaml by `(user_id, name)`; **delete rules absent from yaml**. `category_assignments.rule_id` becomes NULL via `ON DELETE SET NULL`.
    - For every transaction whose assignment is not `MANUAL`, evaluate rules and write the assignment **only if `category_id` or `rule_id` actually changes**.
- CLI commands:
  - `finance categorise` — runs the use case end-to-end.
  - `finance recat <txn-id> <category>` — sets a `MANUAL` assignment. Errors if the category isn't declared in `categories.yaml`.
  - `finance unrecat <txn-id>` — clears the `MANUAL` assignment so the next `categorise` re-evaluates.
  - `finance uncategorised` — lists txns currently in `Uncategorised`.
- Repository extensions: category upsert/list/find-by-name; rule upsert/delete-missing/list; assignment upsert-on-change/get-by-txn/list-uncategorised.

### Out
- Reporting (M4).
- Rule OR-semantics, rule groups, rule expiry — see spec §15 open questions.

## Prerequisites

- M2 complete; transactions populated.

## Deliverables

- [ ] Example yaml files committed.
- [ ] `finance categorise` is convergent: re-running on stable state produces zero writes (asserted by capturing tx counts before/after).
- [ ] Editing a rule and re-running correctly reassigns affected non-manual txns; `assigned_at` updates only on real changes.
- [ ] Removing a rule from yaml deletes the `rules` row; affected assignments become unmatched (re-evaluated on next pass).
- [ ] `MANUAL` assignments survive `finance categorise` runs and survive rule deletions.
- [ ] `recat` with a non-existent category errors clearly (lists candidate categories).
- [ ] `unrecat` followed by `categorise` returns the txn to its rule-derived assignment.
- [ ] Cross-tenant test: rules and assignments isolated per user.

## Architecture context

Two yamls, two concerns. The taxonomy is stable; rules churn. Letting rules implicitly create categories was the original spec's mistake — it forced `kind` to be guessed from a rule, which can't work for the check constraint. With the split, the taxonomy is declared up-front and rules just reference names that already exist.

Rules use `(user_id, name)` as the natural key so reloads are upsert-and-prune, not truncate-and-reinsert. This keeps `rules.id` stable across yaml edits, which keeps `category_assignments.rule_id` meaningful.

## Test plan (TDD)

1. `internal/categorise/predicate_test.go` — table-driven, one case per predicate field, plus combinations (AND-semantics).
2. `internal/categorise/loader_test.go`:
   - Categories: valid yaml loads; missing `kind` errors; orphan `parent` errors.
   - Rules: missing referenced category errors; bad regex errors with line context; duplicate rule names error.
3. `internal/categorise/apply_test.go` — first-match; tiebreaker on equal priority.
4. `internal/categorise/categorise_test.go`:
   - Convergence: run twice, second run does zero assignment writes.
   - Rule edit changes assignments; `assigned_at` moves only on changed rows.
   - Rule deletion → assignment becomes unmatched → next pass falls through to `Uncategorised`.
   - `MANUAL` survives all of the above.
5. `cmd/cli/recat_test.go` — manual override, error on unknown category.
6. `cmd/cli/unrecat_test.go` — clears `MANUAL`, next `categorise` re-derives.
7. `cmd/cli/uncategorised_test.go` — only returns rows in `Uncategorised`.

## Acceptance criteria

- All deliverables ticked.
- Real-data smoke test: run `finance categorise` against the user's actual M2-ingested txns, run `uncategorised`, write a rule, re-run, list shrinks.

## Files an agent will touch

```
config/categories.example.yaml
config/rules.example.yaml
internal/categorise/{predicate.go, loader.go, apply.go, categorise.go, *_test.go}
internal/storage/postgres/{category.go, rule.go, assignment.go} (extend)
internal/storage/postgres/queries.sql (extend)
cmd/cli/{categorise.go, recat.go, unrecat.go, uncategorised.go, *_test.go}
```

## Pitfalls

- Don't auto-create categories from rules — `kind` can't be inferred. Force the user to declare them in `categories.yaml`.
- Don't delete categories from yaml absence; their FKs in `assignments` and `rules` would block. Manual SQL only.
- Don't truncate-and-reinsert rules; that destabilises `rules.id`. Upsert by `(user_id, name)` and delete missing.
- Don't update `assigned_at` on every run — only when the assignment actually changed. Convergence depends on this.
- Regex compilation failures must surface at load, not at apply. Fail fast.
