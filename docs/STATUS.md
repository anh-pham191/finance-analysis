# STATUS — where we are, what's next

> Single source of truth for project state. Update this file every time the situation changes (new milestone started, plan written, milestone completed, decision made).

**Last updated:** 2026-05-06 (M1 implementation in progress)

## Current state

- **Phase:** M1 implementation in progress on `feature/m1-skeleton-db`.
- **Code in repo:** M1 skeleton, Postgres migrations, domain/ports, Postgres repos, CLI health/migrate/version, Makefile, and unit CI are being implemented.
- **Spec status:** `docs/superpowers/specs/2026-05-06-finance-analysis-design.md` — revised after review (RLS hardening, per-aggregate repos, M6 demoted, M8 split into M8a/M8b, akahu_tokens deferred to M8a, etc.). Approved by user.
- **Architecture docs:** `docs/architecture/overview.md` and `docs/architecture/security.md` — current.
- **Per-milestone briefs:** M1, M2, M3, M4, M5, M7, M8a, M8b under `docs/milestones/`. (No M6: Westpac is now a smoke-test acceptance under M2.)

## Next action

Finish M1 verification and review on `feature/m1-skeleton-db`, then stage changes for human review. Do not commit without explicit approval.

## Branching

This project uses Git Flow-lite. See [`docs/process/branching.md`](process/branching.md).

- `main` — stable; updated only via `develop → main` release PRs.
- `develop` — integration; all work PRs merge here. **Default branch on GitHub.**
- `feature/<slug>`, `fix/<slug>`, `docs/<slug>`, `chore/<slug>` — branched off `develop`, PR'd into `develop`.

**Never commit directly to `main` or `develop`.**

## Milestone tracker

| # | Title | Brief | Plan | Implementation |
|---|---|---|---|---|
| M1 | Skeleton & DB (RLS hardened) | ✅ written | ✅ written | 🚧 in progress |
| M2 | Akahu ingest (incl. Westpac smoke-test) | ✅ written | ⏳ | ⏳ |
| M3 | Categorisation | ✅ written | ⏳ | ⏳ |
| M4 | Reporting MVP | ✅ written | ⏳ | ⏳ |
| M5 | Polish | ✅ written | ⏳ | ⏳ |
| M7 | HTTP API (Authenticator port) | ✅ written | ⏳ | ⏳ |
| M8a | Auth & encrypted token store | ✅ written | ⏳ | ⏳ |
| M8b | Rotation, rules-in-DB, audit, deletion | ✅ written | ⏳ | ⏳ |

Legend: ✅ done · ⏳ pending · 🚧 in progress · ❌ blocked.

## Branching

This project uses Git Flow-lite. See [`docs/process/branching.md`](process/branching.md) for the full convention.

- `main` — stable; updated only via `develop → main` release PRs.
- `develop` — integration; all work PRs merge here.
- `feature/<slug>`, `fix/<slug>`, `docs/<slug>`, `chore/<slug>` — branched off `develop`, PR'd into `develop`.

**Never commit directly to `main` or `develop`.**
(M6 — Westpac — folded into M2 acceptance as a smoke-test verification.)

## Decisions log

Record decisions made outside of the spec here so they survive across sessions and agents.

- **2026-05-06** — Repo made public on GitHub (`anh-pham191/finance-analysis`). Reinforces the gitignore policy: nothing personal lands here.
- **2026-05-06** — Multi-tenancy baked in from M1 (was originally going to be a non-goal). Triggered by user's "down the track people will authenticate their bank account" requirement.
- **2026-05-06** — Postgres chosen over SQLite for forward compatibility with the multi-user web service.
- **2026-05-06** — Akahu integration: on-demand pull only for MVP; webhooks deferred.
- **2026-05-06** — Adopted Git Flow-lite branching: `main` / `develop` long-lived, `feature/*` and `fix/*` short-lived, all changes via PR. Convention in `docs/process/branching.md`.
- **2026-05-06** — Default branch on GitHub set to `develop` so cloners land on integration, not main.
- **2026-05-06** (post-review) — RLS implementation hardened: `FORCE ROW LEVEL SECURITY` on every table; dedicated `finance_app` non-owner role; every repo call wrapped in `withUserTx` with `SET LOCAL`. Documented in spec §4 and security doc.
- **2026-05-06** (post-review) — `Repository` god interface split into per-aggregate ports (`AccountRepo`, `TxnRepo`, etc.) under `internal/ports/`.
- **2026-05-06** (post-review) — Categorisation split: `categories.yaml` declares taxonomy with `kind`; `rules.yaml` declares matching predicates referencing category names. Solves the `kind`-not-derivable-from-rule problem.
- **2026-05-06** (post-review) — `akahu_tokens` table deferred from M1 to M8a, with `key_version` + per-ciphertext nonces from day one. M1 stays focused; encryption schema is designed once against real rotation requirements.
- **2026-05-06** (post-review) — M6 demoted to smoke-test acceptance under M2.
- **2026-05-06** (post-review) — M8 split into M8a (auth + encrypted tokens) and M8b (rotation + rules-in-DB + audit + deletion).
- **2026-05-06** (post-review) — `Authenticator` port introduced in M7 (not M8) so M8a's `SessionAuthenticator` extends rather than swaps.
- **2026-05-06** (post-review) — ISO weeks Monday-start (8601). First-sync default lookback 30d. Renderers `io.Writer`-based. JSON tags on DTOs from M4. archtest mandated from M1.

## Open decisions (not blocking implementation)

- Production master key location for M8b/multi-user (KMS vs sealed-secret vs env on hardened host) — decide at M8a kickoff based on hosting choice.
- Self-hosted single shared instance vs hosted SaaS for M8a+ — default assumption is self-hosted shared instance.
- Whether to vendor `golang-migrate` as a library or shell out to its CLI — decide during M1 plan.
- Akahu transaction corrections (different amount for same id) — current: keep amount stable, refresh raw_json. Revisit if observed.
- Akahu account un-link — current: keep, mark `enabled=false` once that flag exists.
