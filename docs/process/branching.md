# Branching & PR convention

> Adapted from the [development-rule](https://github.com/anh-pham191/development-rule) philosophy. This is the local enforcement contract — every contributor (human or agent) follows it.

## Branches

- **`main`** — stable, releasable. Only updated via PR from `develop` at release time. Never commit directly. Never force-push.
- **`develop`** — integration branch. All work merges here first via PR. Never commit directly. Never force-push.
- **`feature/<slug>`** — new functionality. Branched off `develop`. Merged back via PR.
- **`fix/<slug>`** — bug fix. Branched off `develop`. Merged back via PR.
- **`docs/<slug>`** — docs-only changes. Same flow.
- **`chore/<slug>`** — tooling, deps, or housekeeping. Same flow.
- **`hotfix/<slug>`** *(rare)* — urgent fix branched off `main`, PR'd into `main` **and** cherry-picked / merged back into `develop`. Use only for production breakage.

`<slug>` is short, kebab-case, descriptive. Examples: `feature/m1-skeleton-and-db`, `fix/sync-pagination-cursor`, `docs/m2-akahu-fixtures`.

## Workflow

For every unit of work:

1. **Sync develop:** `git checkout develop && git pull --ff-only`.
2. **Branch:** `git checkout -b feature/<slug>` (or `fix/<slug>`, etc.).
3. **Plan, then code.** Each milestone has a brief in `docs/milestones/`. Before writing code, write or read the implementation plan in `docs/superpowers/plans/`. TDD throughout — failing test first.
4. **Commit in small, coherent steps.** Imperative subject ≤ 72 chars; body explains *why*. Co-author trailer if an AI assistant materially contributed.
5. **Push:** `git push -u origin feature/<slug>`.
6. **Open a PR against `develop`.** Title mirrors the work; description follows the template below. Keep PRs reviewable — under ~400 lines of diff is the target; split if larger.
7. **Address review.** Push follow-up commits; do not force-push during review (preserves comment anchoring). If you must rebase, communicate first.
8. **Merge.** Squash or merge commit — pick one and stay consistent (default: **squash** for feature branches; **merge commit** for release PRs `develop → main`). Delete the branch after merge.
9. **Update [`docs/STATUS.md`](../STATUS.md)** as part of the same PR if the work changes project state (milestone progress, decisions, next action).

## What goes in a PR

Every PR description includes:

- **What** — one-paragraph summary of the change.
- **Why** — the motivation. Reference the milestone (e.g. "M1") and the spec section if applicable.
- **How** — high-level approach; flag any deviation from the milestone brief or design spec.
- **Testing** — what tests were added/changed; how to run them locally; whether `golangci-lint run` and `go test ./...` pass.
- **Risk / rollback** — anything to watch out for; how to revert.
- **Screenshots / output** — for CLI changes, paste the new help text or example output.

## What does NOT go in a PR

- Multiple unrelated changes. One concern per PR.
- Schema changes without a migration. No exceptions.
- A migration that omits `user_id` on user-owned tables. See [security doc](../architecture/security.md).
- A change that modifies an existing test to silence a failing run. Tests are specifications.
- Anything that bypasses hooks (`--no-verify`) without explicit approval.
- Secrets, real bank data, or anything matching the [`.gitignore`](../../.gitignore) policy.

## PR review (self or peer)

Before requesting review (or before an agent declares work complete):

- [ ] Tests pass locally (`make test` and, for repo-touching work, `make test-integration`).
- [ ] `golangci-lint run` clean.
- [ ] No secrets / no PII in diffs or new files.
- [ ] If a new repository method was added: cross-tenant test included (per spec §11).
- [ ] No new env-var reads outside `cmd/` or `internal/akahu/`.
- [ ] [`docs/STATUS.md`](../STATUS.md) updated if state changed.
- [ ] Commit messages are imperative and explain *why*.

## Releases (`develop → main`)

When `develop` is in a stable state and you want to mark a release:

1. Open a PR `develop → main` titled `Release vX.Y.Z`.
2. Description summarises everything since the last release tag (one line per merged PR).
3. Merge with a **merge commit** (preserves history of integration).
4. Tag `main` with `vX.Y.Z` and push the tag.
5. No cherry-picking from feature branches directly to `main`. Always integrate via `develop` first (except hotfixes).

## Branch protection (recommended GitHub settings)

When the repo is no longer a solo effort, configure on GitHub:

- `main` — require PR, require linear history, require status checks (CI), no force pushes, no deletions.
- `develop` — require PR, require status checks, no force pushes, no deletions.
- Allow squash merges and merge commits; disable rebase merges to keep history readable.

For solo work today, treat the rules as self-enforced; flip the protections on as soon as anyone else (human or agent) is committing.
