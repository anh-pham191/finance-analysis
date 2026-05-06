# M8 — Multi-user

> Spec reference: §1, §4 users/akahu_tokens, §5 TokenStore, §8 secrets, §9 privacy. Architecture: `docs/architecture/security.md`. Builds on M7.

## Goal

Turn on real multi-user: registration, authentication, per-user encrypted Akahu tokens, per-user onboarding, and account deletion. No domain or storage redesign — the multi-tenant scaffolding has been in place since M1.

## Scope

### In
- **Registration & login**
  - `POST /auth/register` — email + password (argon2id hash), creates a `users` row.
  - `POST /auth/login` — verifies password, issues a session token (opaque random, 32 bytes, stored hashed in `sessions` table with TTL).
  - `POST /auth/logout` — invalidates the session.
  - Bearer-token middleware reads `Authorization: Bearer <session>`, looks up `user_id`, sets `app.user_id` on the connection for the request, attaches user to context.
- **Akahu onboarding**
  - `POST /akahu/tokens` — accepts the user's app+user tokens (from their Akahu dashboard), encrypts via `DBTokenStore`, stores in `akahu_tokens`.
  - `DELETE /akahu/tokens` — clears stored tokens.
  - `POST /sync` already takes the authenticated user — now resolves tokens via `DBTokenStore` instead of `EnvTokenStore`.
- **Encryption**
  - `internal/crypto/aesgcm.go` — `Encrypt(key, plaintext) (ciphertext, nonce, error)` and `Decrypt(key, ciphertext, nonce) (plaintext, error)`.
  - `KeyProvider` port + `EnvKeyProvider` (default) and `KMSKeyProvider` (stub interface for AWS/GCP/Cloudflare KMS — pick one at deploy time).
  - `finance admin rotate-keys` — re-encrypts all rows under a new key version.
- **Rules in DB**
  - Rules become first-class per-user records, edited via API endpoints. The `config/rules.yaml` file becomes a personal-mode-only convenience for the original developer; multi-user installs disable it.
- **Right to delete**
  - `DELETE /users/me` — hard-delete the user and all their data in one transaction. Returns 204.
  - `finance admin delete-user --id <n>` — same, from CLI for support.
- **Audit log (lightweight)**
  - `audit_log` table: `user_id`, `action`, `at`, `request_id`. Login, token store, token delete, user delete are logged. No PII logged.

### Out
- Email verification, password reset flows (M9 candidates — design with stubs from M8 so adding them is mechanical).
- OAuth (Google, GitHub login).
- Per-user role/permission matrix (single trust tier suffices).
- Webhook ingestion — separate milestone.

## Prerequisites

- M7 complete (HTTP API exists, single bearer-token auth).
- A decision on production master key location (KMS vs sealed secret vs env on a hardened host).
- A decision on hosting model (self-hosted single instance, or hosted SaaS).

## Deliverables

- [ ] Two real users can register, log in, connect their own Akahu accounts, and run `POST /sync`. Each sees only their own transactions in every report endpoint.
- [ ] Cross-tenant integration test: user B logs in, every endpoint that takes a path or query argument referring to a resource is exercised against a user-A-owned ID. Every such call returns 404 or 403, never 200 with data.
- [ ] Tokens in the DB are unreadable without the master key (verified by inspecting a dump).
- [ ] `finance admin rotate-keys` rotates without downtime; tested with a small fixture.
- [ ] `DELETE /users/me` removes all rows; integration test confirms.
- [ ] `gitleaks`, `govulncheck` clean.

## Architecture context

This milestone is the validation that the multi-tenant scaffolding from M1 was worth doing. If anything in `internal/domain`, `internal/report`, `internal/categorise`, or `internal/ingest` needs to be edited to support real multi-user, that's a signal an earlier milestone leaked single-user assumptions. Stop and fix the abstraction first.

The `Repository` interface gains no methods specific to auth. Auth concerns (sessions, password hashes) live in a separate `internal/auth/` package with its own narrower repository interface. Do not pollute the domain repository with auth.

## Test plan (TDD)

1. `internal/auth/password_test.go` — argon2id hash and verify; constants chosen per OWASP guidance.
2. `internal/auth/session_test.go` — create, validate, expire.
3. `internal/crypto/aesgcm_test.go` — round-trip; tampered ciphertext fails verification; wrong key fails verification.
4. `internal/storage/postgres/token_store_test.go` — encrypts on write, decrypts on read; cross-user isolation; works under RLS.
5. `cmd/api/auth_test.go` — register, login, logout, expired session.
6. `cmd/api/onboarding_test.go` — store tokens, sync uses them, delete tokens stops sync.
7. `cmd/api/cross_tenant_test.go` — exhaustive: every endpoint with another user's ID returns 404/403.
8. `cmd/api/user_delete_test.go` — full data removal.

## Pitfalls

- Argon2id parameters: pick once, document, do not tweak per environment. Aim for ~250ms hash time on prod hardware.
- Session storage: store the **hash** of the session token in the DB, not the token itself. A DB leak should not yield live sessions.
- Rotation requires `key_version` on `akahu_tokens`. Don't omit it; retrofitting is painful.
- Don't conflate `users` (auth identity) with `accounts` (bank accounts at Akahu). Two different concepts; two different tables.
- The CLI single-user mode (`EnvTokenStore`) keeps working alongside the API multi-user mode. Don't remove it; some deployments will be CLI-only forever.
