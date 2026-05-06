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

Even though M1–M5 run with one user (`id=1`), the data model and ports are multi-tenant from the first migration:

- Every user-owned table has `user_id bigint NOT NULL` with an index.
- Every `Repository` method takes `userID UserID` as the first non-context argument.
- Postgres Row-Level Security policies enforce `user_id = current_setting('app.user_id')::bigint` on every user-owned table. The repository implementation calls `SET LOCAL app.user_id = $1` per transaction.
- A mandatory cross-tenant test pattern (spec §11) runs for every new repository method.

This means M8 (real multi-user) does not require a schema migration or a domain rewrite — it adds auth, the `DBTokenStore`, and the registration flow on top of an already-correct foundation.

## Token storage

`TokenStore` port:

```go
type TokenStore interface {
    AkahuTokens(ctx context.Context, userID UserID) (app, user string, err error)
}
```

Two implementations:

### `EnvTokenStore` (M2–M7)
Reads `AKAHU_APP_TOKEN` and `AKAHU_USER_TOKEN` from env. Returns the same tokens regardless of `userID` (acceptable while there is one user). Acceptable because the only deployment is the developer's own machine.

### `DBTokenStore` (M8)
Tokens live in `akahu_tokens` table:

- `app_token_ciphertext bytea`
- `user_token_ciphertext bytea`
- `nonce bytea`
- `updated_at timestamptz`

Encryption: AES-256-GCM. Key from `KeyProvider` port:

```go
type KeyProvider interface {
    MasterKey(ctx context.Context) ([]byte, error)  // 32 bytes
}
```

- `EnvKeyProvider` — reads `FINANCE_MASTER_KEY` (base64). For self-hosted personal/family use.
- `KMSKeyProvider` — wraps a cloud KMS (AWS / GCP / Cloudflare) decrypt call. For hosted multi-user.

Rotation: a new key version is added to the provider, all rows are re-encrypted in a background command (`finance admin rotate-keys`), the old key is retired. Schema includes `key_version` column on `akahu_tokens` to support concurrent rotation.

## Logging redaction

Single `slog.Handler` wrapper applied at the root logger. `replace_attr` clears values for keys matching:

- `(?i)token`
- `(?i)authorization`
- `(?i)password`
- `(?i)secret`
- `(?i)api[_-]?key`

Replaced values become the literal string `***`. Tested with golden tests on the handler.

PII redaction is policy, not infrastructure: developers must not log txn amounts, descriptions, or merchants. Reviewers reject PRs that do.

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
