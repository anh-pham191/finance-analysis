# Reporting DTOs

M4 report DTOs are the JSON contract reused by the CLI JSON renderers and the future M7 HTTP API. Monetary values are strings with two decimal places.

All period ranges are half-open: `from` is inclusive and `to` is exclusive.

## SummaryResult

```json
{
  "period": {
    "from": "2026-04-01T00:00:00+13:00",
    "to": "2026-05-01T00:00:00+12:00"
  },
  "income": "5000.00",
  "expense": "1234.56",
  "net": "3765.44",
  "categories": [
    {
      "category_id": 1,
      "category": "Food/Groceries",
      "kind": "expense",
      "total": "234.56"
    },
    {
      "category_id": 2,
      "category": "Income/Salary",
      "kind": "income",
      "total": "5000.00"
    }
  ],
  "has_uncategorised": false
}
```

## CompareResult

`delta` is `b - a`. `delta_percent` is `((b - a) / a) * 100`; it is `null` when `a` is zero.

```json
{
  "a": {
    "from": "2026-03-01T00:00:00+13:00",
    "to": "2026-04-01T00:00:00+13:00"
  },
  "b": {
    "from": "2026-04-01T00:00:00+13:00",
    "to": "2026-05-01T00:00:00+12:00"
  },
  "categories": [
    {
      "category_id": 1,
      "category": "Food/Groceries",
      "kind": "expense",
      "a": "200.00",
      "b": "250.00",
      "delta": "50.00",
      "delta_percent": 25
    },
    {
      "category_id": 3,
      "category": "New Category",
      "kind": "expense",
      "a": "0.00",
      "b": "42.00",
      "delta": "42.00",
      "delta_percent": null
    }
  ]
}
```

## TxnRow

Transaction drill-down output intentionally includes transaction fields requested by the user. It never includes `raw_json` or tokens.

```json
{
  "txn_id": "txn_123",
  "posted_at": "2026-04-15T00:00:00Z",
  "account_id": "acc_123",
  "category": "Food/Groceries",
  "direction": "DEBIT",
  "amount": "12.30",
  "merchant": "Example Merchant",
  "description": "Example transaction"
}
```
