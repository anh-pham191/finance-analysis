# M8a — Auth & encrypted token store

> Spec reference: §4 (akahu_tokens, sessions added here), §5 (DBTokenStore), §8 token encryption. Architecture: `docs/architecture/security.md`. Builds on M7.

## Goal

Real multi-user. Users register, log in, and connect their own Akahu accounts. Akahu tokens move from env into encrypted DB storage. The `Authenticator` port from M7 gets a second implementation (`SessionAuthenticator`) — extension, not swap.

## Scope

### In
- **Schema migrations:**
  - `users` add `password_hash text NOT NULL DEFAULT ''` (default removed in a follow-up after seeding).
  - `sessions` table — `token_hash bytea PK`, `user_id bigint FK ON DELETE CASCADE`, `created_at`, `expires_at`. Token plaintext is never stored.
  - `akahu_tokens` — schema per spec §8 token encryption: per-ciphertext nonces, `key_version int NOT NULL DEFAULT 1`, FK ON DELETE CASCADE.
- **`internal/crypto/aesgcm.go`** — `Encrypt(key, plaintext) (ciphertext, nonce []byte, err error)`, `Decrypt(key, ciphertext, nonce []byte) (plaintext []byte, err error)`. AES-256-GCM, random 96-bit nonces.
- **`internal/ports/KeyProvider`** + `internal/auth/EnvKeyProvider` (reads `FINANCE_MASTER_KEY` for the current version, `FINANCE_MASTER_KEY_v<n>` for prior versions used by decrypt fallback).
- **`internal/auth/`:**
  - `Argon2id{m, t, p}` parameters chosen per OWASP guidance, **documented in code with citation and rationale**, e.g. `m=64MiB, t=3, p=4` aiming for ~250ms on prod hardware. Constants exported and tested for "minimum strength" boundaries.
  - `HashPassword(plain) (hash string)`, `VerifyPassword(plain, hash) (bool, error)`.
  - `CreateSession(ctx, userID) (token string, expiresAt time.Time)` — generates 32-byte random token, stores SHA-256 of token in `sessions`. Returns plaintext token to caller (only seen once).
  - `SessionAuthenticator` — implements the M7 `Authenticator` port; reads `Authorization: Bearer <token>`, hashes, looks up session, returns `UserID`.
- **`internal/storage/postgres/db_token_store.go`** — `DBTokenStore` implementing `TokenStore`. Encrypts on write with current key version; decrypt path tries `MasterKey(ctx, row.key_version)`.
- **API endpoints:**
  - `POST /auth/register` — email + password → user row + session.
  - `POST /auth/login` — verify password → session.
  - `POST /auth/logout` — invalidate session.
  - `POST /akahu/tokens` — body has app + user tokens; encrypted via `DBTokenStore`.
  - `DELETE /akahu/tokens` — clears the user's tokens.
- Wire `cmd/api` to use `SessionAuthenticator` (not `EnvBearerAuthenticator`) in production; keep both for dev.
- Cross-tenant test exhaustively: every endpoint that takes a path or query argument referring to a resource is exercised against another user's ID and must return 404 (or 403 where appropriate).

### Out
- Email verification, password reset (M9 candidates).
- OAuth.
- Per-user roles.
- Key rotation command, audit log, user delete (M8b).
- Rules-in-DB primary (M8b — yaml stays primary in M8a).

## Prerequisites

- M7 complete with the `Authenticator` port designed correctly.
- Decision: which `KeyProvider` is used in dev (`EnvKeyProvider`) vs production (TBD — KMS or env on a hardened host).

## Deliverables

- [ ] Two real users can register, log in, connect their own Akahu accounts via API, and run `POST /sync`. Each sees only their own transactions in every report endpoint.
- [ ] Tokens in the DB are unreadable without the master key (verified by inspecting a `pg_dump`).
- [ ] Wrong key produces a typed `ErrDecrypt` from `DBTokenStore`; surfaced as 500 with no token leak.
- [ ] Every API endpoint passes a cross-tenant test.
- [ ] Single-user CLI mode still works (uses `EnvTokenStore` + dev user). M8a doesn't break the developer's local workflow.

## Test plan (TDD)

1. `internal/crypto/aesgcm_test.go` — round-trip; tampered ciphertext fails verification; wrong key fails verification; nonces always unique across many encryptions.
2. `internal/auth/password_test.go` — hash/verify; OWASP boundary check.
3. `internal/auth/session_test.go` — create, validate, expire, revoke. Plaintext token never stored.
4. `internal/storage/postgres/db_token_store_test.go` — encrypt on write; decrypt under current and previous key versions; cross-user isolation under RLS.
5. `cmd/api/auth_test.go` — register/login/logout flows; bad credentials; expired session.
6. `cmd/api/onboarding_test.go` — store tokens, sync uses them, delete tokens stops sync.
7. `cmd/api/cross_tenant_test.go` — exhaustive matrix.

## Pitfalls

- Argon2id parameters: pick once, document, do not tweak per environment. Aim ~250ms on prod hardware. Cite OWASP version in the comment.
- Session storage stores **the hash** of the token. A DB leak should not yield live sessions.
- Don't share a nonce column between two ciphertexts. The schema has separate `app_token_nonce` and `user_token_nonce` for this reason.
- Don't forget `key_version` from day one of the table — retrofitting it is painful (see M8b).
- Don't leak the master key into logs, errors, or error paths. The `KeyProvider` returns `[]byte`; don't `fmt.Sprintf` it.
- `cmd/api` config: don't conditionally pick `SessionAuthenticator` vs `EnvBearerAuthenticator` based on a magic env. Make it explicit (`API_AUTH_MODE=session|env_bearer`) so dev/prod is obvious.
- Don't conflate `users` (auth identity) with `accounts` (Akahu bank accounts). Two different concepts.
