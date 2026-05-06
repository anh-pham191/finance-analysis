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
[anh-pham191/development-rule](https://github.com/anh-pham191/development-rule). Highlights:

- NZ English in user-facing copy and docs.
- Never commit without explicit human approval.
- Tests are specifications — never modified to silence a failing run without explicit reasoning.
- Pause before destructive operations.
- TDD throughout: red → green → refactor.
- YAGNI ruthlessly. Composable over monolithic. Evolved from use.

## Milestones

See [`STATUS.md`](STATUS.md) for the live status tracker. Briefs themselves are at [`milestones/`](milestones/).
