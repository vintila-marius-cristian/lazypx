# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] — Bug Fix & Test Coverage Pass

### Security Fixes

- **api/client.go** — URL-encode form bodies so values containing special characters (semicolons, ampersands) are transmitted correctly
- **api/errors.go** — Redact API tokens, UUIDs, and other secrets from error messages before they surface to the user

### Bug Fixes

- **api/access.go** — Fix broken `printJSON` that used `fmt.Println` with a string representation instead of proper JSON marshaling
- **tui/app.go** — Wire reboot and delete confirmation actions to their handler functions (previously no-ops)
- **tui/sidebar.go** — Implement search filtering — pressing `/` now filters the sidebar resource list in real time
- **tui/app.go** — Add debounced VM extras loading to prevent rapid re-queries when switching selections quickly
- **cache/cache.go** — Surface per-node cache errors to the user instead of silently discarding them; added `Errors []string` to `ClusterSnapshot`
- **cache/cache.go** — Add retry logic to `WatchTask` on transient failures
- **tui/app.go** — Eliminate duplicate config load on TUI startup (config was loaded twice)
- **tui/app.go** — Fix `ConfirmAction` type from `func() interface{}` to `func() tea.Cmd`, removing all type assertion boilerplate
- **sessions/manager.go** — Fix `ListSessions` / `IsAlive` data race on `ProcessState` by using `atomic.Bool` flag set by background `Wait()` goroutine

### Performance

- **tui/app.go** — Prevent overlapping cache refresh goroutines with a debounce guard
- **tui/app.go** — Clean up dead shell panes (sessions that exited while in background)

### Features

- **tui/app.go** — Wire audit log to TUI actions (start, stop, reboot, delete, snapshot create/delete)
- **tui/app.go** — Implement VM/CT migration action (`m` key)
- **tui/app.go** — Add PTY cleanup on quit via `CloseAll()` to prevent orphaned background sessions

### Refactoring

- **api/client.go** — Remove dead `isClientError` function (unused)
- **commands/root.go** — Unify version into `config.Version` constant instead of a duplicated string
- **commands/root.go** — Clean up example config generation
- **api/timeouts.go** — Centralize timeout constants (`DefaultTimeout`, `LongTimeout`) and replace all hardcoded `time.Second * 15` values

### Test Coverage

| Package | Before | After |
|---------|--------|-------|
| api | 17% | 82.6% |
| audit | 0% | 100% |
| cache | 80% | 95.5% |
| commands | 0% | 22.8% |
| config | 21% | 92.9% |
| sessions | 13% | 63.9% |
| state | 0% | 100% |
| tui | 17% | 34.2% |

New test files added:

| File | Coverage areas |
|------|---------------|
| `api/api_test.go` | Mock server routing, class errors, retries, timeout, redaction |
| `audit/audit_test.go` | 100% — Open, Log, Close, read back, restart |
| `cache/cache_extra_test.go` | AllVMs, AllContainers, IsEmpty, error collection, edge cases |
| `commands/commands_test.go` | Root cmd structure, version, clientFromConfig, printVMs, printJSON |
| `config/config_test.go` | Load, ExampleConfig, EnsureConfigDir, SSH hosts, keyring |
| `sessions/manager_test.go` | Open/Close/List/IsAlive, race detection, PTY lifecycle |
| `state/state_test.go` | 100% — UpdateCache, Tasks, ConfirmAction, state transitions |
| `tui/components_test.go` | StatusDot, StatusStyle, truncate, Sidebar, Detail, Tasks panes |
| `tui/help_test.go` | Help overlay rendering |

### Documentation

- **docs/evaluation.md** — Full application evaluation table (security, performance, reliability, architecture)

### Production source files modified

- `api/client.go` — HTTP client, URL encoding fix, removed dead `isClientError`
- `api/errors.go` — ClassifyError, IsRetryable, RedactMessage with regex redaction
- `api/timeouts.go` — **NEW** centralized timeout constants
- `api/api_test.go` — **NEW** comprehensive API tests (mock server routing)
- `audit/audit.go` — Fixed `Close()` to clear `logPath` for clean restart
- `audit/audit_test.go` — **NEW** 100% coverage tests
- `cache/cache.go` — Added `Errors []string` to `ClusterSnapshot`, error collection
- `cache/cache_extra_test.go` — **NEW** AllVMs, AllContainers, error paths, edge cases
- `commands/root.go` — Unified version, fixed duplicate config load
- `commands/access.go` — Fixed `printJSON`
- `commands/commands_test.go` — **NEW** Root structure, version, clientFromConfig tests
- `config/config.go` — Added `Version` constant, cleaned example config
- `config/config_test.go` — **NEW** Load, ExampleConfig, EnsureConfigDir tests
- `sessions/manager.go` — Fixed `ProcessState` race (atomic.Bool), added `CloseAll()`
- `sessions/manager_test.go` — Expanded with race detection and lifecycle tests
- `state/state.go` — Fixed `AppendTaskLog`/`MarkTaskDone` bounds, `ConfirmAction` type
- `state/state_test.go` — **NEW** 100% coverage tests
- `tui/app.go` — Reboot/delete actions, audit log, migrate, refresh guard, shell cleanup, timeout constants
- `tui/sidebar.go` — Search filtering implementation
- `tui/snapshots.go` — `ConfirmAction` type fix
- `tui/backups.go` — `ConfirmAction` type fix
- `tui/components_test.go` — **NEW** Component and pane tests
- `tui/help_test.go` — **NEW** Help overlay tests
- `docs/evaluation.md` — **NEW** Full application evaluation table

---

## [v0.1.0] — Initial Release

- TUI dashboard with Bubble Tea (nodes, VMs, containers, storage panes)
- Embedded PTY SSH shell with persistent sessions
- Proxmox VE API client with token authentication
- Cluster data caching with TTL
- CLI subcommands: `vm`, `ssh`, `snapshot`
- OS keychain integration for token secrets
- Real-time task log
- Power actions (start, stop, reboot, delete)
- Snapshot management (create, rollback, delete)
