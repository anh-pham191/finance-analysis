# M8b — Rotation, rules-in-DB, audit, deletion

> Spec reference: §8 token encryption (rotation), §9 right to delete, §13 milestones. Builds on M8a.

## Goal

Operationalise multi-user: rotate encryption keys without downtime, move rule management from yaml-on-disk to DB-primary, record an audit trail of sensitive events, and implement the right-to-delete endpoint.

## Scope

### In
- **Key rotation:**
  - `finance admin rotate-keys` — walks `akahu_tokens` in chunks, `SELECT ... FOR UPDATE` per row, decrypts under current `key_version`, re-encrypts under the new key, increments `key_version`, commits per chunk. Concurrent reads succeed because decrypt tries the row's stored version.
  - Configurable chunk size; documented operational runbook.
- **Rules in DB primary:**
  - `POST /rules`, `PUT /rules/{id}`, `DELETE /rules/{id}`, `GET /rules` API endpoints.
  - `finance rules export > rules.yaml` and `finance rules import < rules.yaml` for backup/migration.
  - `finance categorise` reads from `rules` table (not yaml) by default in multi-user installs.
  - The dev user's existing yaml-driven workflow continues via `--rules-from-yaml` flag for local CLI use.
- **Audit log:**
  - `audit_log` table — `id bigserial PK`, `user_id bigint FK`, `action text`, `at timestamptz DEFAULT now()`, `request_id text NULL`, `metadata jsonb`. **No PII** in `metadata` — counts and IDs only.
  - Writes from: register, login (success/fail), logout, store/delete Akahu tokens, rotate-keys (admin), delete user.
- **Right to delete:**
  - `DELETE /users/me` — hard-deletes the user; cascades remove every owned row in one transaction. 204 on success.
  - `finance admin delete-user --id <n>` — same, from CLI.
- **`govulncheck`** runs in CI on every push.
- **Audit log retention:** documented (current: unbounded; revisit if it becomes a cost).

### Out
- Email verification, password reset (M9).
- KMS implementation of `KeyProvider` — still deferred; `EnvKeyProvider` covers self-hosted.
- Webhook ingestion.
- Multi-tenancy promotion of the dev user from M1 (separate "data import" milestone if/when needed).

## Prerequisites

- M8a complete.
- Decision: hosting model. KMS provider integration only needed for hosted deployment.

## Deliverables

- [ ] `finance admin rotate-keys` rotates without downtime; tested with a multi-row fixture and concurrent reads via a load script.
- [ ] All rules-on-DB CRUD endpoints work and pass cross-tenant tests.
- [ ] `rules export | import` round-trips losslessly.
- [ ] Audit rows written for all listed actions; integration test asserts each.
- [ ] `DELETE /users/me`: integration test seeds two users with overlapping data, deletes user A, asserts zero rows for A across every table and unchanged rows for B.
- [ ] CI gates merge on `govulncheck`.

## Test plan (TDD)

1. `internal/storage/postgres/rotate_test.go` — rotation walks rows; partial failure mid-rotation can resume; rows already at new version are skipped.
2. `cmd/api/rules_test.go` — CRUD + cross-tenant.
3. `internal/rules/yaml_test.go` — export/import round-trip preserves order, priority, predicate.
4. `cmd/api/audit_test.go` — each sensitive action writes the expected row; PII not present in metadata.
5. `cmd/api/user_delete_test.go` — full data removal under RLS.

## Pitfalls

- Rotation per-row tx: don't lock the whole table. `SELECT ... FOR UPDATE` per chunk.
- Keep the previous key available throughout rotation; only retire after completion. Coordinate with deployment.
- Audit log is append-only — no UPDATE, no DELETE permitted to the app role. Enforce via `REVOKE`.
- Audit metadata: never include passwords, tokens, amounts, descriptions, merchants. Confirm via redaction-style test.
- `DELETE /users/me`: re-prompt for password on the API side before honouring (defence against stolen session).
- Rules export/import: don't make export include DB ids — natural key (`name`) only — so importing into another instance works.
