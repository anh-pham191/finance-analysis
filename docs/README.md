# finance-analysis docs

Entry points for anyone (human or agent) working on this project.

## Read in this order

1. **Design spec** — the source of truth for what we're building and why:
   `superpowers/specs/2026-05-06-finance-analysis-design.md`
2. **Architecture overview** — the *why* behind the layout, kept short:
   `architecture/overview.md`
3. **Security & privacy architecture** — multi-tenancy, secrets, encryption, deletion:
   `architecture/security.md`
4. **Milestone you're working on** — full context to act in isolation:
   `milestones/M{n}-*.md`

## Philosophy

This project follows the rules in
`/Users/anhpham/Documents/Projects/script/development_rule/`. Highlights:

- NZ English in user-facing copy and docs.
- Never commit without explicit human approval.
- Tests are specifications — never modified to silence a failing run without explicit reasoning.
- Pause before destructive operations.
- TDD throughout: red → green → refactor.
- YAGNI ruthlessly. Composable over monolithic. Evolved from use.

## Milestones

| # | Title | Status |
|---|---|---|
| M1 | Skeleton & DB | Pending |
| M2 | Akahu ingest | Pending |
| M3 | Categorisation | Pending |
| M4 | Reporting MVP | Pending |
| M5 | Polish | Pending |
| M6 | Westpac | Future |
| M7 | HTTP API | Future |
| M8 | Multi-user (auth, encrypted tokens, signup, deletion) | Future |
