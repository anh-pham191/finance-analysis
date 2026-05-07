# finance-analysis

Personal finance tool for NZ bank accounts. Connects to ANZ and Westpac via the [Akahu](https://akahu.nz) personal API, ingests transactions into Postgres, categorises them via user-authored rules with manual override, and exposes summary / comparison / drill-down reporting both as a CLI and as an MCP server that any MCP-compatible chat tool can call.

This README covers **local setup** and **wiring the MCP server into chat tools** (Claude Desktop, Claude Code, Cursor, and any other MCP-aware client). For project status, design docs, and milestones, see [`docs/STATUS.md`](docs/STATUS.md) and [`AGENTS.md`](AGENTS.md).

---

## 1. Prerequisites

- Go 1.22+
- Docker (for local Postgres via `docker compose`)
- An Akahu personal API account with an app token and user token ([akahu.nz](https://akahu.nz))
- A chat tool that speaks MCP over stdio (Claude Desktop, Claude Code, Cursor, Continue, Zed, Cline, etc.)

## 2. Clone and configure environment

```bash
git clone https://github.com/anh-pham191/finance-analysis.git
cd finance-analysis
cp .env.example .env
```

Edit `.env` and fill in:

- `AKAHU_APP_TOKEN` — your Akahu app token
- `AKAHU_USER_TOKEN` — your Akahu user token

The Postgres URLs in `.env.example` already point at the local Docker database — leave them as-is unless you're using a different Postgres instance.

> Never commit `.env` or real Akahu tokens.

## 3. Start the database and run migrations

```bash
make db-up          # starts Postgres in Docker on port 15432
make migrate        # applies schema migrations
```

## 4. First sync and categorisation

Load `.env` into your shell and run a sync:

```bash
set -a; . ./.env; set +a
go run ./cmd/cli sync
go run ./cmd/cli categorise
go run ./cmd/cli summary --period this-month
```

If `sync` reports `0 accounts`, check that ANZ/Westpac are connected in your Akahu dashboard and that the tokens in `.env` have the right permissions.

## 5. CLI reference

```bash
finance sync                                  # pull new txns from Akahu
finance categorise                            # apply rules from config/rules.yaml
finance summary --period 2026-04              # totals + by-category
finance compare --mom                         # this month vs last month
finance compare --wow                         # this week vs last week
finance compare --yoy                         # this year vs last year
finance txns --category Food/Groceries --period last-month --sort amount
finance recat <txn-id> Food/Eating-out        # manual override
finance uncategorised                         # what hasn't matched any rule yet
```

Output formats: `--format=table|csv|json|md`.

---

## 6. MCP server

`cmd/mcp` exposes the reporting + write tools as an MCP server over stdio. Any MCP-compatible chat client can spawn it as a subprocess and call its tools.

### Tools exposed

Read-only:

- `summary` — income/spending by category for a period
- `compare` — diff two periods
- `list_txns` — filter transactions by period/category/merchant/amount/etc.
- `list_uncategorised` — transactions still in the Uncategorised bucket
- `list_categories` — every configured category

Write:

- `assign_category` — manual override for a single transaction
- `upsert_category` — create or update a category (with optional parent + kind)
- `sync` — run an Akahu sync from the chat tool

Periods accept `this-month`, `last-month`, `this-week`, `last-week`, `this-year`, `last-year`, or explicit `YYYY` / `YYYY-MM` / `YYYY-Www`.

### Build and install the launcher

```bash
make mcp-install
```

This:

1. Builds `cmd/mcp` to `~/bin/finance-mcp`.
2. Copies a launcher script to `~/bin/finance-mcp-launch.sh`.
3. Extracts `DATABASE_URL_APP`, `DATABASE_URL`, and `AKAHU_BASE_URL` from `.env` into `~/.config/finance-mcp/env` (mode 600).

The launcher path matters on macOS — Claude Desktop runs sandboxed and can't read files inside `~/Documents`, so the env file must live outside that tree.

> ⚠️ The launcher reads database credentials and Akahu tokens. Anyone who can spawn it can call every tool, including writes (`assign_category`, `upsert_category`, `sync`). Don't expose it beyond your own machine.

### Wire it into a chat tool

The launcher speaks plain stdio — every MCP client config follows the same shape, only the file location differs.

#### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "finance-analysis": {
      "command": "/Users/<you>/bin/finance-mcp-launch.sh"
    }
  }
}
```

Restart Claude Desktop. The tools appear under the 🔧 menu.

#### Claude Code (CLI)

```bash
claude mcp add finance-analysis ~/bin/finance-mcp-launch.sh
```

Or edit `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "finance-analysis": {
      "command": "/Users/<you>/bin/finance-mcp-launch.sh"
    }
  }
}
```

#### Cursor

Settings → MCP → *Add new MCP server*, or edit `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "finance-analysis": {
      "command": "/Users/<you>/bin/finance-mcp-launch.sh"
    }
  }
}
```

#### Continue (VS Code / JetBrains)

In `~/.continue/config.yaml`:

```yaml
mcpServers:
  - name: finance-analysis
    command: /Users/<you>/bin/finance-mcp-launch.sh
```

#### Zed

In `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "finance-analysis": {
      "command": {
        "path": "/Users/<you>/bin/finance-mcp-launch.sh",
        "args": []
      }
    }
  }
}
```

#### Cline / other VS Code extensions

In the extension's MCP settings:

```json
{
  "mcpServers": {
    "finance-analysis": {
      "command": "/Users/<you>/bin/finance-mcp-launch.sh",
      "transportType": "stdio"
    }
  }
}
```

#### Anything else (generic MCP client)

If your tool documents an MCP stdio transport, point it at the launcher with no arguments. The server reads its env from `~/.config/finance-mcp/env`, so the client doesn't need to forward any environment variables.

If you'd rather skip the launcher and pass env directly:

```json
{
  "mcpServers": {
    "finance-analysis": {
      "command": "/Users/<you>/bin/finance-mcp",
      "env": {
        "DATABASE_URL_APP": "postgres://finance_app:...@localhost:15432/finance?sslmode=disable",
        "AKAHU_APP_TOKEN": "...",
        "AKAHU_USER_TOKEN": "...",
        "AKAHU_BASE_URL": "https://api.akahu.io/v1"
      }
    }
  }
}
```

### Verify

In your chat tool, ask "list my categories" or "summarise this month". The client should call `list_categories` / `summary` and return JSON.

If a tool fails:

- Run the launcher manually: `~/bin/finance-mcp-launch.sh` — it should sit silently waiting for MCP frames over stdin. Ctrl-C exits.
- Confirm Postgres is up (`make db-up`) and migrations are applied (`make migrate`).
- For sync errors, re-run `go run ./cmd/cli sync` from a shell to surface the underlying Akahu error.

---

## 7. Updating after code changes

```bash
git pull
make migrate         # if migrations changed
make mcp-install     # rebuilds and reinstalls the MCP binary
```

Restart your chat tool to pick up the new binary.

## 8. Documentation

| Read first | What it is |
|---|---|
| [`docs/STATUS.md`](docs/STATUS.md) | Project state and what's next |
| [`AGENTS.md`](AGENTS.md) | Entry point for AI agents working on this repo |
| [`docs/architecture/overview.md`](docs/architecture/overview.md) | The *why* behind the layout |
| [`docs/architecture/security.md`](docs/architecture/security.md) | Multi-tenancy, secrets, encryption, deletion |
| [`docs/milestones/`](docs/milestones/) | Per-milestone briefs (M1–M8) |

## License

Personal project. License TBD before any third party contributes.
