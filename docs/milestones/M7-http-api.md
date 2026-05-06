# M7 — HTTP API

> Spec reference: §3 Architecture (ports), §13 Milestones. Builds on M5.

## Goal

Expose the same use cases over HTTP so a future web UI (or mobile app, or other automation) can call them. No domain changes. The `Authenticator` port is introduced here — designed so M8a extends it (`SessionAuthenticator`), never swaps it.

## Scope

### In
- `cmd/api/` with stdlib `net/http` + `chi` for routing groups.
- `internal/ports/Authenticator`:
  ```go
  type Authenticator interface {
      Authenticate(ctx context.Context, r *http.Request) (UserID, error)
  }
  ```
- `internal/auth/EnvBearerAuthenticator` — compares `Authorization: Bearer <X>` against `API_AUTH_TOKEN` env, resolves to dev `UserID(1)`.
- Endpoints (initial set; all wrappers around existing use cases, ≤15 lines each):
  - `POST /sync` → `ingest.Sync`
  - `POST /categorise` → `categorise.Categorise`
  - `POST /transactions/{id}/category` → `categorise.Recat`
  - `DELETE /transactions/{id}/category` → `categorise.Unrecat`
  - `GET /reports/summary?period=...` → `report.Summary`
  - `GET /reports/compare?a=...&b=...` (or `?wow|mom|yoy`) → `report.Compare`
  - `GET /transactions?...` → `report.Transactions`
- Middleware (in this order): recover → request logging (slog JSON; no PII) → CORS (configurable origins, **disabled** by default) → rate limit (token-bucket, per-IP, generous default) → `Authenticator` → handler.
- DTOs: same `report.*Result` types as M4 — JSON tags already present.
- OpenAPI spec hand-written in `docs/api/openapi.yaml` covering current endpoints; CI lints it (`spectral` or similar).
- Dockerfile for the API binary.
- Integration tests covering: unauth → 401; bad bearer → 401; valid bearer → 200; cross-tenant safety (request crafted as user A asking for user B's resource → 404 because RLS prevents the row appearing).

### Out
- Web UI itself (separate repo).
- Multi-user, sessions, password auth, registration — that's M8a.
- Webhook ingestion.

## Prerequisites

- M5 complete; M4 renderers are `io.Writer`-based and DTOs have JSON tags. If not, this milestone blocks.

## Deliverables

- [ ] Each handler is a 5–15 line wrapper around an existing use case.
- [ ] No new code in `internal/{domain,ports,report,categorise,ingest,render}` to make this milestone work — if you find yourself editing those, stop. The abstractions need fixing first.
- [ ] CSRF posture documented: bearer-token-in-header is not vulnerable to CSRF for the M7 API (no cookie auth). M8a re-evaluates if cookies/sessions are added.
- [ ] CORS is disabled by default; configurable via `API_CORS_ORIGINS` env.
- [ ] Rate-limit configurable; default values documented.
- [ ] `Authenticator` port is parallel to `TokenStore` / `KeyProvider` — M8a's `SessionAuthenticator` is a clean second implementation, not a refactor.

## Architecture context

This milestone is the validation that the hexagonal layout was worth the discipline. If anything in the use-case layer leaks `os.Stdout`, `os.Args`, or `fmt.Println`, M7 stalls until M4 (or earlier) is fixed. That's a feature of the design, not a bug — discover the leak now, not after a UI is built on top.

The `Authenticator` interface is a small but load-bearing port. It must NOT take user-id as input; it returns one. M8a's `SessionAuthenticator` reads a cookie or header, looks up the session row, and returns the user. Same shape.

## Test plan (TDD)

1. `internal/auth/env_bearer_test.go` — happy path; missing header; wrong scheme; mismatched value.
2. `cmd/api/middleware_test.go` — recover wraps panics; rate limit returns 429; CORS preflight responds correctly when configured.
3. `cmd/api/handlers_test.go` — for each endpoint: happy path, unauth, validation errors.
4. `cmd/api/cross_tenant_test.go` — seed two users; user A's bearer asking for user B's transaction returns 404; never 200.

## Pitfalls

- Don't reach for a heavy framework. `chi` + stdlib is enough.
- Don't add per-handler authn — middleware does it once.
- Don't return DB IDs in errors. Return what the user asked for and a generic message.
- Don't log the bearer token. Don't log request bodies that could include PII.
- `EnvBearerAuthenticator` returning `UserID(1)` is the single-user shortcut; M8a replaces it. Don't bake the constant `1` anywhere except this authenticator.
