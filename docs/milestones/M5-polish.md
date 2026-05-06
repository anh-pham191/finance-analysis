# M5 — Polish

> Spec reference: §8 Configuration & secrets, §9 Observability.

## Goal

Make the tool pleasant to use day-to-day and easy for a fresh agent (or fresh machine) to pick up. No new domain features.

## Scope

### In
- Config loading: `~/.config/finance-analysis/config.yaml`, `--config` override, `.env` via `godotenv` in dev, env var precedence over file for secrets.
- `log/slog` text handler in CLI, JSON handler ready for API. `--verbose` toggles DEBUG.
- Friendly error messages at CLI boundary: missing tokens explain how to fix, DB connection errors explain `make db-up`.
- README: install, setup (Akahu, Postgres), usage examples, common tasks, troubleshooting.
- `config/rules.example.yaml` with realistic NZ examples (Countdown, Pak'nSave, BP, Z, IRD payments, KiwiSaver).
- `make` targets documented; `make help` lists them.
- `.editorconfig`, basic GitHub Actions CI (build + lint + unit tests; integration on demand).

### Out
- Anything that changes data model or domain behaviour.

## Prerequisites

- M4 complete.

## Deliverables

- [ ] A new agent on a clean machine can follow README → working tool in under 15 minutes.
- [ ] Misconfigured token produces a single clear error, not a stack trace.
- [ ] CI green on every push.

## Test plan (TDD)

1. `internal/config/loader_test.go` — file precedence, env override, missing-required errors.
2. CLI smoke tests with intentionally-bad config to assert error messages.

## Acceptance criteria

- All deliverables ticked.
- README has been read by the user and any rough edges fixed.
