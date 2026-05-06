# M6 — Westpac (later)

> Spec reference: §1 Purpose. Builds on M5.

## Goal

Add Westpac accounts via Akahu. If M2 was built right, this is mostly a configuration task — no code changes to the ingest path.

## Scope

### In
- User connects Westpac in their Akahu dashboard.
- `finance sync` picks up new accounts automatically (M2 already iterates accounts from `AkahuClient.ListAccounts`).
- Verify that direction inference, merchant fields, and dates from Westpac data work the same as ANZ.
- Add fixtures captured from Westpac for the `internal/akahu/` test suite.
- Add Westpac-specific rules to `config/rules.example.yaml` if any merchant strings differ.

### Out
- Multi-bank reconciliation features.
- Per-bank UI.

## Deliverables

- [ ] Westpac transactions visible in `finance summary`.
- [ ] No regressions in ANZ ingest.
- [ ] Test fixtures committed for Westpac response shapes.

## Pitfalls

- Banks differ in date precision, merchant string format, and how transfers are labelled. Capture real data first, then write tests, then adjust mappers if needed.
- If Westpac requires changes to the mapper, add a focused test in `internal/akahu/` rather than branching by bank in domain code. The domain stays bank-agnostic.
