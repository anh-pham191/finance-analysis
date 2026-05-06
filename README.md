# finance-analysis

Personal finance tool for NZ bank accounts. Connects to ANZ (Westpac later) via the [Akahu](https://akahu.nz) personal API, ingests transactions into Postgres, categorises them via user-authored rules with manual override, and produces summary, comparison, and drill-down reports from the command line.

CLI today, multi-user web service later. Hexagonal architecture, TDD throughout, written in Go.

> **Status:** spec approved, implementation not yet started. See [`docs/STATUS.md`](docs/STATUS.md) for what's next.

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
