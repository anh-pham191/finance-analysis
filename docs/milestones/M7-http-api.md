# M7 — HTTP API (later)

> Spec reference: §3 Architecture. Builds on M6.

## Goal

Expose the same use cases over HTTP so a future web UI (or mobile app, or other automation) can call them. No domain changes.

## Scope

### In
- `cmd/api/` with `net/http` (or `chi` if routing complexity warrants — start with stdlib).
- Endpoints (initial set):
  - `POST /sync` — runs ingest.
  - `POST /categorise` — runs the rule engine over uncategorised txns.
  - `POST /transactions/{id}/category` — manual override.
  - `GET /reports/summary?period=...`
  - `GET /reports/compare?a=...&b=...` (with `wow|mom|yoy` shortcuts)
  - `GET /transactions?...` (filters from M4)
- DTOs are the same structs `internal/report` already returns (or thin JSON wrappers if struct field names need adjusting).
- Auth: bearer token from env (`API_AUTH_TOKEN`), single-user. Don't build user accounts yet.
- `slog` JSON handler. Request logging middleware. Recover middleware.
- OpenAPI spec generated from handlers (or hand-written — pick the cheaper path at the time).
- Dockerfile for the API binary.

### Out
- Web UI itself (separate repo or M8).
- Multi-user, OAuth, per-user data scoping.
- Webhook ingestion (could land here or as its own milestone).

## Deliverables

- [ ] Each handler is a 5–15 line wrapper around an existing use case.
- [ ] Integration tests hit the HTTP layer end-to-end against testcontainers Postgres.
- [ ] No new code in `internal/domain`, `internal/report`, `internal/categorise`, `internal/ingest` to make this milestone work — if you find yourself editing those, stop and revisit; the abstractions are wrong and need fixing first.

## Architecture context

This milestone is the validation that the hexagonal layout from M1 was worth doing. If it isn't easy, that's a signal that some use case leaked CLI assumptions (e.g. directly writing to stdout, reading from `os.Args`) and needs refactoring.

## Pitfalls

- Don't reach for a heavy framework. Stdlib `net/http` plus `chi` if you need routing groups is plenty.
- DTOs for HTTP and report results should be the same types unless there's a real reason to diverge. Two parallel sets is a maintenance trap.
- Don't add auth complexity; bearer token is enough for personal use.
