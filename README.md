# StellarView TUI

Terminal block explorer for the Stellar network — part of [StellarView](https://github.com/StellarViewOrg).

Fast, keyboard-driven investigation of Stellar and Soroban data from the shell: live monitoring, entity lookup, related-entity traversal, source-aware reads, and local operator context (labels, notes, bookmarks, saved views) — without leaving the command line.

## Repository layout

This repository holds two Go modules plus local infrastructure:

| Path | What it is |
|---|---|
| [`tui/`](./tui) | **StellarView TUI** — the terminal client (Bubble Tea + Lip Gloss). Runs standalone against Stellar RPC / Horizon, or against the indexer backend for richer reads. Local state is kept in SQLite. |
| [`tui-indexer/`](./tui-indexer) | **StellarView TUI Indexer** — backend service that ingests Stellar data into PostgreSQL/TimescaleDB, publishes live events via Redis, and exposes a read HTTP API shaped for terminal views (lists, timelines, holders, search). |
| [`infra/`](./infra) | Docker Compose stacks for local Postgres, Redis, and Typesense. |

Data modes: **RPC** (default, no backend required), **Hybrid** (indexer with RPC fallback), and **Indexer**.

> **Status:** v0.1.0 — first standalone release foundation. Behavior and views may still change.

## Quick start

Run the client against a public RPC endpoint — no local services required:

```bash
cd tui
make build
STELLAR_RPC_URL=https://soroban-testnet.stellar.org ./bin/stellar-tui
```

On first run the client creates `~/.config/stellar-tui/config.json` and `~/.config/stellar-tui/cache.db`.

### Indexed / hybrid mode

Bring up the backend infrastructure and the indexer service for richer reads:

```bash
# 1) start local infrastructure (Postgres, Redis, Typesense)
cd infra
docker compose -f docker-compose.tui-indexer.yml up -d

# 2) run migrations and start ingesting
cd ../tui-indexer
make migrate
make run-live

# 3) serve the read API for the client
make run-serve
```

Then point the client at the backend via the `indexer_url` field in your profile config.

## Documentation

- [`tui/README.md`](./tui/README.md) — client configuration, keybindings, data modes, and caching
- [`tui-indexer/README.md`](./tui-indexer/README.md) — backend pipeline, read API endpoints, and infrastructure

## StellarView

StellarView is an open-source suite for exploring the Stellar network:

- [StellarView Explorer](https://github.com/StellarViewOrg/stellarview-explorer) — web block explorer
- [StellarView Indexer](https://github.com/StellarViewOrg/stellarview-indexer) — data ingestion service
- **StellarView TUI** — terminal block explorer (this repository)
- [StellarView Docs](https://github.com/StellarViewOrg/stellarview-docs) — documentation
