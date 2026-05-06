# Security & Privacy Architecture

> Companion to spec §8 (Configuration & secrets) and §9 (Privacy & PII). Read those first.

This project handles **per-user financial data** and **bank-API tokens**. Treat both as high-sensitivity throughout. The single-user MVP runs on the user's own machine; the multi-user mode (M8+) may run as a shared self-hosted instance for friends/family or as a small hosted service. The architectural choices below cover both.

## Threat model

In scope:
- **Accidental leakage** — secrets committed to git, tokens in logs, txn data in error messages.
- **Cross-tenant leakage** — user A's queries returning user B's rows.
- **Token theft from the database** — a leaked DB dump must not yield usable Akahu tokens.
- **Rogue dependency** — minimise blast radius if a transitive Go dep is compromised.

Out of scope (for now):
- Full SOC2 / ISO27001 compliance.
- Defending against a malicious authenticated user trying to escalate privileges (single trust tier in M8).
- Hardening against a compromised host OS.

## Data classification

| Class | Examples | Handling |
|---|---|---|
| **Critical** | Akahu app/user tokens, master encryption key, password hashes | Encrypted at rest, never logged, never in errors propagated past the use case boundary, never in fixtures committed to git |
| **Sensitive** | Transactions (amount, merchant, description), account numbers, user email | Stored unencrypted in DB (the DB itself is the protection boundary), never logged at INFO, never sent to third parties |
| **Internal** | Rule definitions, category names | Per-user; not commitable to a public repo |
| **Public** | Documentation, code, synthetic fixtures | Commit freely |

## Multi-tenancy from day 1

Even though M1–M5 run with one user (`id=1`), the data model and ports are multi-tenant from the first migration. The implementation discipline is non-negotiable because misconfigured RLS silently passes cross-tenant tests:

1. **Every user-owned table has `user_id bigint NOT NULL`** with an index, and (where the natural key is external) a composite PK including `user_id`.
2. **`ON DELETE CASCADE` from `users`** on every owned row — single-statement user deletion.
3. **RLS is enabled AND forced:**
   ```sql
   ALTER TABLE <t> ENABLE ROW LEVEL SECURITY;
   ALTER TABLE <t> FORCE ROW LEVEL SECURITY;
   CREATE POLICY <t>_tenant_isolation ON <t>
     USING (user_id = current_setting('app.user_id')::bigint)
     WITH CHECK (user_id = current_setting('app.user_id')::bigint);
   ```
   Without `FORCE`, the table owner bypasses the policy and tests pass with broken isolation.
4. **The application connects as `finance_app`** — a dedicated non-owner role without `BYPASSRLS`. Migrations run as the owner; the app does not.
5. **Every repo call runs in a tx** that begins with `SET LOCAL app.user_id = $userID`. `SET LOCAL` is transaction-scoped — outside a `BEGIN/COMMIT` it has no effect, and a connection-pool checkout must not inherit a previous user's setting. Enforced in one place: a `withUserTx(ctx, userID, fn)` helper in the postgres adapter; all queries go through it.
6. **Per-aggregate repo ports** (`AccountRepo`, `TxnRepo`, ...). Each method takes `userID UserID` as the first non-context argument.
7. **Mandatory cross-tenant test** (spec §11) for every new repository method.
8. **Architecture test** (`internal/archtest`) ensures the import graph stays within the layout.

Together these mean M8a (real multi-user) does not require a schema migration or a domain rewrite — it adds auth, `DBTokenStore`, and the registration flow on top of an already-correct foundation.

## Token storage

`TokenStore` port:

```go
type TokenStore interface {
    AkahuTokens(ctx context.Context, userID UserID) (app, user string, err error)
}
```

Two implementations:

### `EnvTokenStore` (M2 → M7)
Reads `AKAHU_APP_TOKEN` and `AKAHU_USER_TOKEN` from env. Returns the same tokens regardless of `userID` — an intentional weakening of the multi-tenancy invariant for single-user MVP. M8a's cross-tenant tests for sync re-run with `DBTokenStore` to validate isolation under real multi-user.

### `DBTokenStore` (M8a)
Tokens live in `akahu_tokens`:

```
user_id bigint PK FK users(id) ON DELETE CASCADE
app_token_ciphertext bytea
app_token_nonce bytea
user_token_ciphertext bytea
user_token_nonce bytea
key_version int NOT NULL DEFAULT 1
updated_at timestamptz
```

**Per-ciphertext nonces are mandatory.** Reusing a nonce with the same key across two plaintexts breaks AES-GCM catastrophically (the XOR of plaintexts becomes recoverable). The single-`nonce` schema from earlier drafts was wrong.

Encryption: AES-256-GCM with random 96-bit nonces, fresh on every write.

`KeyProvider` port:

```go
type KeyProvider interface {
    MasterKey(ctx context.Context, version int) ([]byte, error)  // 32 bytes
}
```

- `EnvKeyProvider` (M8a default) — reads `FINANCE_MASTER_KEY` (base64) for the current version, optionally `FINANCE_MASTER_KEY_v<n>` for prior versions during rotation.
- `KMSKeyProvider` (M8b or later, when hosted) — wraps cloud KMS. Deferred until a hosting decision is made.

### Rotation

Online and concurrent-safe:

1. Add new key as version `N+1` (e.g. `FINANCE_MASTER_KEY` rotates; previous value moves to `FINANCE_MASTER_KEY_v<N>`).
2. `finance admin rotate-keys` walks `akahu_tokens` in chunks, `SELECT ... FOR UPDATE` per row, decrypts with the row's `key_version`, re-encrypts under `N+1`, updates row. Concurrent reads during rotation succeed because the decrypt path tries `MasterKey(ctx, row.key_version)` first.
3. After all rows rotated, the old key value can be removed from the env / KMS.

Decryption is the only place where multi-version awareness lives.

## Logging redaction

Single `slog.Handler` wrapper applied at the root logger. Two layers:

1. **Key-side `replace_attr`** clears values for keys matching `(?i)token|authorization|password|secret|api[_-]?key`. Replaced values become `***`.
2. **Value-side scanner** runs over any string-typed attr value and replaces token-shaped substrings — `(?i)\bbearer\s+\S+`, `(?i)\b(app|user)_token\s*[:=]\s*\S+`, base64-shaped strings ≥32 chars in error contexts. Catches the case where someone logs `slog.Info("got response", "body", string(respBody))` with an `Authorization: Bearer xxx` header in the body.

Tested with golden tests that:
- Synthesise an HTTP error string containing a real-shaped bearer token, log it, capture output, assert the token is absent.
- Log structured attrs with token-shaped keys; assert redaction.
- Log a transaction struct (which would contain PII); assert it doesn't go through the standard handler — there's a `slog.LogValuer` on `domain.Transaction` returning `slog.Attr{Key: "txn_id", Value: ...}` only.

PII redaction is **policy, not infrastructure**: developers must not log txn amounts, descriptions, or merchants. Reviewers reject PRs that do. The `LogValuer` on `Transaction` makes it hard to do accidentally — `slog.Info("found", "txn", t)` emits only the id, never the body.

`raw_json` (the full Akahu payload stored in `transactions.raw_json`) **never enters logs or error chains**. Adapter parse-errors include the txn id only; parse-success drops the raw bytes after extracting fields. A unit test grep's the formatted-error paths to prove no `raw_json` references exist.

## Errors

- Errors propagated to the CLI/HTTP boundary contain only the operation name, a brief reason, and (for HTTP) a request ID for log correlation.
- Internal errors may include row IDs but never tokens, raw bodies that contain auth headers, or PII.
- Adapter errors that wrap an HTTP response excerpt run the response through the redaction filter before wrapping.

## Files never to commit

`.gitignore` from M1 includes (extend as needed):

```
.env
.env.local
config/config.yaml
config/rules.yaml
**/fixtures/**/real.*
*.dump
*.sql.gz
```

`.env.example`, `config/config.example.yaml`, and `config/rules.example.yaml` are committed templates with no real values.

## Pre-commit secret scanning

`gitleaks` runs as a pre-commit hook (installed via `make hooks`) and in CI. Any matched pattern fails the commit/build.

## Backups

The Makefile does not provide an unencrypted dump target. The documented backup path uses `pg_dump | age -e ...` (or `gpg --encrypt`) with the recipient key documented out-of-band. Encrypted dumps may be transferred over normal channels.

## Right to delete

`finance user delete --user <id>` (and M8's `DELETE /users/me`) hard-deletes a user and **all** their data — accounts, transactions, categories, category_assignments, rules, sync_state, akahu_tokens — in a single transaction. An integration test seeds the deleted user and an unrelated user, runs the delete, asserts zero rows for user A and unchanged rows for user B.

## Dependency hygiene

- Pin direct deps in `go.mod`; review `go.sum` diffs in PRs.
- Run `govulncheck ./...` in CI weekly (and on PRs touching `go.mod`).
- Avoid deps that pull large transitive trees for trivial features.
