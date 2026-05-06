# STATUS — where we are, what's next

> Single source of truth for project state. Update this file every time the situation changes (new milestone started, plan written, milestone completed).

**Last updated:** 2026-05-06

## Current state

- **Phase:** Design approved; implementation has not started.
- **Code in repo:** None. Only docs and `.gitignore`.
- **Spec status:** `docs/superpowers/specs/2026-05-06-finance-analysis-design.md` — approved by the user.
- **Architecture docs:** `docs/architecture/overview.md` and `docs/architecture/security.md` — current.
- **Per-milestone briefs:** M1–M8 written under `docs/milestones/`.

## Next action

**Write the implementation plan for M1.** Use the `superpowers:writing-plans` skill (or equivalent on your platform) to produce a detailed, step-by-step plan that an executor can follow. Inputs:

- M1 brief: `docs/milestones/M1-skeleton-and-db.md`
- Spec sections referenced there (§3 Architecture, §4 Data model, §11 Testing).
- Architecture overview and security doc — for invariants the plan must respect.

The plan should land at `docs/superpowers/plans/2026-05-06-M1-skeleton-and-db-plan.md` and be reviewed by the user before any code is written.

## Milestone tracker

| # | Title | Brief | Plan | Implementation |
|---|---|---|---|---|
| M1 | Skeleton & DB | ✅ written | ⏳ next action | ⏳ |
| M2 | Akahu ingest | ✅ written | ⏳ | ⏳ |
| M3 | Categorisation | ✅ written | ⏳ | ⏳ |
| M4 | Reporting MVP | ✅ written | ⏳ | ⏳ |
| M5 | Polish | ✅ written | ⏳ | ⏳ |
| M6 | Westpac | ✅ written | ⏳ | ⏳ |
| M7 | HTTP API | ✅ written | ⏳ | ⏳ |
| M8 | Multi-user | ✅ written | ⏳ | ⏳ |

Legend: ✅ done · ⏳ pending · 🚧 in progress · ❌ blocked.

## Decisions log

Record decisions made outside of the spec here so they survive across sessions and agents.

- **2026-05-06** — Repo made public on GitHub (`anh-pham191/finance-analysis`). Reinforces the gitignore policy: nothing personal lands here.
- **2026-05-06** — Multi-tenancy baked in from M1 (was originally going to be a non-goal). Triggered by user's "down the track people will authenticate their bank account" requirement.
- **2026-05-06** — Postgres chosen over SQLite for forward compatibility with the multi-user web service.
- **2026-05-06** — Akahu integration: on-demand pull only for MVP; webhooks deferred.

## Open decisions (not blocking implementation)

- Production master key location for M8 (KMS vs sealed-secret vs env on hardened host) — decide at M8 kickoff based on hosting choice.
- Self-hosted single shared instance vs hosted SaaS for M8+ — default assumption is self-hosted shared instance.
- Whether to vendor `golang-migrate` as a library or shell out to its CLI — decide during M1 plan.
