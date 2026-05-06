# M5 — Polish

> Spec reference: §8 Configuration & secrets, §9 Privacy & PII, §10 Observability.

## Goal

Make the tool pleasant for daily use and trivial for a fresh agent (or fresh machine) to bootstrap. No new domain features.

## Scope

### In
- **Config loading** per spec §8 precedence: flag > env > file > default. Required-but-missing settings error at startup with a message naming the setting and the env var that supplies it.
- **`.env` loading** via `godotenv` in dev mode only (`FINANCE_ENV=dev`).
- **Slog** — text handler in CLI, JSON handler ready for M7 API. `--verbose` toggles DEBUG. Redaction filter (key + value scanning) wired into the root logger.
- **Friendly error catalogue** — explicit list of N concrete failure scenarios, each with the exact expected user-facing message:
  1. `DATABASE_URL` unset.
  2. Postgres unreachable (connection refused).
  3. `AKAHU_APP_TOKEN` / `AKAHU_USER_TOKEN` missing.
  4. Akahu auth fails (401/403) — message points at "rotate your tokens at akahu.nz".
  5. Akahu rate-limit exhausted after retries — message says try again later.
  6. `categories.yaml` references a missing parent.
  7. `rules.yaml` references a missing category.
  8. `recat <txn-id>` with non-existent txn.
  9. Period parse fails (`finance summary --period 2026-W53`).
  10. (extend as gaps appear.)
  Each is asserted by an integration test producing the exact wording.
- **`make backup`** — `pg_dump` piped through `age -r <recipient>`. Recipient public key location documented; private key never committed.
- **`gitleaks` pre-commit hook** — `make hooks` installs it. CI runs it on every push.
- **README** — install, prerequisites, Akahu setup, Postgres setup, daily workflow, troubleshooting (referencing the error catalogue), and a final verification step `finance health && finance summary --period this-month` that must succeed.
- **`config/{categories,rules,config}.example.yaml`** — realistic NZ examples (Countdown, Pak'nSave, BP, Z, IRD, KiwiSaver). Already partially present from M3; finalise here.
- **GitHub Actions CI** — `lint`, `test`, `gitleaks`, `govulncheck` (foreshadowing M8b but cheap to add now).
- **`.editorconfig`**, `make help` listing targets.

### Out
- Anything that changes data model or domain behaviour.

## Prerequisites

- M4 complete.

## Deliverables

- [ ] A new agent on a clean machine: clone → follow README → `finance health && finance summary --period this-month` succeeds. Time-boxed: under 15 minutes.
- [ ] Each of the 10 error scenarios produces its exact expected message; verified by automated tests.
- [ ] `gitleaks` blocks a commit containing a fake AWS key (verified once, manually).
- [ ] `make backup` produces an `age`-encrypted file; `age -d` round-trips.
- [ ] CI green on every push.

## Test plan (TDD)

1. `internal/config/loader_test.go` — precedence, missing-required errors with named messages.
2. `cmd/cli/errors_test.go` — one test per scenario in the catalogue, asserting exact stderr.

## Acceptance criteria

- All deliverables ticked.
- README walkthrough performed by user (or a fresh agent), any rough edges fixed.

## Files an agent will touch

```
internal/config/{loader.go, loader_test.go}
internal/observability/redaction.go (extend if needed)
cmd/cli/{errors.go (catalogue + helpers), *_test.go}
.github/workflows/ci.yml
.git/hooks/* (via make hooks installer script: scripts/install-hooks.sh)
.editorconfig
Makefile (extend with backup, hooks)
README.md (rewrite)
config/categories.example.yaml, rules.example.yaml, config.example.yaml (finalise)
```

## Pitfalls

- Don't put secret values into the example yaml — only structure and shape.
- Don't make `make backup` work without `age` installed silently — fail clearly.
- Don't add error messages without a test pinning the wording — the catalogue is its own contract.
