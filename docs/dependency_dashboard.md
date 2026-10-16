# Dependency Management & Renovate Architecture

Documentation for automated dependency management, multi-module Go workspace maintenance, and Renovate bot workflows in `stellarview-tui`.

## Overview

`stellarview-tui` uses [Renovate](https://docs.renovatebot.com/) for automated Go module and GitHub Actions dependency updates. Renovate maintains consistency across all Go modules within the `go.work` multi-module workspace.

- **Automation Engine**: Renovate (`renovate[bot]`)
- **Interactive Tracking**: [Dependency Dashboard](https://github.com/StellarViewOrg/stellarview-tui/issues/21) (Issue #21)
- **Configuration**: `renovate.json` in repository root

## Configuration (`renovate.json`)

The repository configuration enforces weekly batching, semantic commits, and multi-module Go tracking:

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:recommended", ":semanticCommits", "schedule:weekly"],
  "timezone": "UTC",
  "schedule": ["before 6am on monday"],
  "prConcurrentLimit": 3,
  "prHourlyLimit": 2,
  "labels": ["dependencies"],
  "lockFileMaintenance": {
    "enabled": true,
    "schedule": ["before 6am on monday"]
  },
  "gomod": {
    "enabled": true
  },
  "github-actions": {
    "enabled": true
  },
  "packageRules": [
    {
      "description": "Group every non-major update into one PR",
      "matchUpdateTypes": ["minor", "patch", "pin", "digest"],
      "groupName": "all non-major dependencies",
      "groupSlug": "all-non-major"
    },
    {
      "description": "Group every major update into one PR",
      "matchUpdateTypes": ["major"],
      "groupName": "all major dependencies",
      "groupSlug": "all-major"
    }
  ],
  "vulnerabilityAlerts": {
    "groupName": "security updates"
  }
}
```

## Multi-Module Go Workspace Coverage

The repository is organized as a Go workspace (`go.work`) with three core modules tracked by Renovate:

1. **`tui`**: Terminal user interface for Stellar ledger navigation, keyboard shortcuts, and transaction inspection (Bubbletea / Lipgloss stack).
2. **`tui-indexer`**: Backend service that ingests Stellar data into PostgreSQL/TimescaleDB, publishes live events through Redis, and exposes an HTTP read API that `tui` uses in Hybrid and Indexer modes.
3. **`sordecode`**: Core Soroban decoding engine, parsing binary XDR ledger entries, ScVal types, and contract storage keys.

### Policies & Scheduling

- **Weekly Schedule**: Automated PR creation runs before **6:00 AM UTC on Mondays** to keep development uninterrupted during weekdays.
- **Grouped Updates (`packageRules`)**: Updates are grouped by `packageRules` into one `all non-major dependencies` PR (covering minor, patch, pin, and digest updates) and one `all major dependencies` PR for major bumps.
- **Vulnerability Alerts**: `vulnerabilityAlerts` groups security fixes into one `security updates` PR.
- **Semantic Commits**: Pull request titles and commits follow the Conventional Commits standard (`chore(deps): ...`, `fix(deps): ...`).
- **Rate & Concurrency Limits**: Maximum 3 open PRs concurrently (`prConcurrentLimit: 3`) and 2 PRs per hour (`prHourlyLimit: 2`) to protect CI workflows.
- **Lockfile Maintenance**: Runs weekly on Mondays to re-verify `go.sum` and transitive dependencies.

## Dependency Dashboard (Issue #21) Operations

Renovate maintains Issue `#21` as an ongoing interactive Dependency Dashboard.

> [!NOTE]
> Issue `#21` must remain open indefinitely. It serves as the live control center for Renovate. Closing this issue disables interactive controls for rebase requests and scheduled update previews.

### Dashboard Features

- **Scheduled PRs**: Lists upcoming dependencies scheduled for the next Monday maintenance window.
- **On-Demand PR Generation**: Checking the checkbox next to any pending dependency prompts Renovate to open the PR immediately.
- **Rebase All Open PRs**: Check the rebase checkbox to automatically trigger rebases against `main` for all open dependency PRs.

## Verification Commands

When testing dependency updates across all Go workspace modules:

```bash
# sordecode
cd sordecode && go vet ./... && go test ./...

# tui
cd tui && gofmt -l . && go vet ./... && make test-all && make build

# tui-indexer (needs Postgres/TimescaleDB, see infra/)
cd tui-indexer && gofmt -l . && go vet ./... && make test && go build ./cmd/tui-indexer
```
