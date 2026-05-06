# finance-analysis

Personal finance tool for NZ bank accounts. Connects to ANZ (Westpac later) via the [Akahu](https://akahu.nz) personal API, ingests transactions into Postgres, categorises them via user-authored rules with manual override, and produces summary, comparison, and drill-down reports from the command line.

CLI today, multi-user web service later. Hexagonal architecture, TDD throughout, written in Go.

> **Status:** M4 reporting implementation is in final verification. See [`docs/STATUS.md`](docs/STATUS.md) for what's next.

## What it does

```bash
finance sync                                  # pull new txns from Akahu
finance categorise                            # apply rules from config/rules.yaml
finance summary --period 2026-04              # totals + by-category
finance compare --mom                         # this month vs last month
finance compare --wow                         # this week vs last week
finance compare --yoy                         # this year vs last year
finance txns --category Food/Groceries --period last-month --sort amount
finance recat <txn-id> Food/Eating-out        # manual override
finance uncategorised                         # what hasn't matched any rule yet
```

Output formats: `--format=table|csv|json|md`.

## Akahu Sync Smoke Test

After M2 is implemented, run a local sync against Akahu with synthetic local database credentials and real Akahu tokens kept only in `.env`:

```bash
cp .env.example .env
# Fill AKAHU_APP_TOKEN and AKAHU_USER_TOKEN in .env.
set -a; . ./.env; set +a
make db-up
make migrate
go run ./cmd/cli sync
```

Acceptance checks:

- ANZ is connected in the Akahu dashboard and transactions sync into Postgres.
- Re-running `go run ./cmd/cli sync` does not create duplicate transactions.
- Westpac is connected in the Akahu dashboard, then `go run ./cmd/cli sync` brings in Westpac transactions without code changes.

Never commit `.env` or real Akahu tokens.

## M3 Categorisation Smoke Test

After syncing transactions into the local Docker database, run categorisation with environment variables loaded from `.env`:

```bash
set -a; . ./.env; set +a
go run ./cmd/cli categorise
go run ./cmd/cli uncategorised
```

Write one rule in `config/rules.yaml`, rerun `go run ./cmd/cli categorise`, then run `go run ./cmd/cli uncategorised` again. The uncategorised count should shrink when the new rule matches existing transactions.

## M4 Reporting Smoke Test

After syncing and categorising local data, run the reporting commands with `.env` loaded:

```bash
set -a; . ./.env; set +a
go run ./cmd/cli summary --period this-month
go run ./cmd/cli compare --mom
go run ./cmd/cli txns --period this-month --limit 20 --sort date
```

For a real-data spot check, compare the month-on-month figures against a small manual sample from the database or bank export before marking M4 complete.

## Documentation

| Read first | What it is |
|---|---|
| [`docs/STATUS.md`](docs/STATUS.md) | What state the project is in and what to do next |
| [`AGENTS.md`](AGENTS.md) | Entry point for AI agents working on this repo |
| [`docs/superpowers/specs/2026-05-06-finance-analysis-design.md`](docs/superpowers/specs/2026-05-06-finance-analysis-design.md) | Full design spec — source of truth |
| [`docs/architecture/overview.md`](docs/architecture/overview.md) | The *why* behind the layout |
| [`docs/architecture/security.md`](docs/architecture/security.md) | Multi-tenancy, secrets, encryption, deletion |
| [`docs/milestones/`](docs/milestones/) | Per-milestone briefs (M1–M8) |

## Philosophy

This project follows the rules in [anh-pham191/development-rule](https://github.com/anh-pham191/development-rule). Highlights:

- **NZ English** in user-facing copy and docs.
- **Tests are specifications** — never modified to silence a failing run.
- **Never commit without explicit human approval.**
- **TDD throughout** — red → green → refactor.
- **YAGNI ruthlessly.** Composable over monolithic. Evolved from use.

See [`AGENTS.md`](AGENTS.md) for the full agent-facing rules.

## License

Personal project. License TBD before any third party contributes.
