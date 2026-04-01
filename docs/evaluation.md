# lazypx — Full Application Evaluation

**Date:** 2026-04-01
**Version:** v0.1.0
**Go:** 1.26.0 (darwin/arm64)
**Binary:** 12 MB
**Source:** 8,297 LOC (production) + 934 LOC (tests) across 53 files

---

## 1. Project Structure

| Aspect | Grade | Notes |
|--------|-------|-------|
| Package layout | A | Clean separation: `api/`, `cache/`, `commands/`, `config/`, `sessions/`, `state/`, `tui/`, `audit/` |
| Dependency flow | A | No circular imports; `state` is the shared leaf; `tui` imports `api`/`cache`/`config`/`sessions` |
| Module hygiene | B | `go.sum` verified; `go.mod` directive at 1.25.0 (valid with 1.26 runtime) |
| Binary size | A | 12 MB single static binary — no CGO, no runtime deps |
| Config location | A | XDG-compliant `~/.config/lazypx/` |

## 2. API Client (`api/`)

| Aspect | Grade | Notes |
|--------|-------|-------|
| Token auth | A | `PVEAPIToken=USER!TOKEN=SECRET` header; no password prompts |
| TLS handling | A | Configurable insecure mode; default secure |
| Retry logic | A | Exponential backoff (500ms base) on 502/503/504/network errors; 3 attempts; skips 4xx |
| Error classification | A | Typed errors (`ErrUnauthorized`, `ErrNotFound`, `ErrQuorumLost`, `ErrClusterLocked`); retryability via `IsRetryable()` |
| Secret redaction | B | Regex redaction for UUIDs and PVEAPIToken headers in error messages; 500-char truncation |
| Form encoding | A | `url.Values.Encode()` prevents injection |
| Coverage | 17.4% | Tests cover auth header, TLS, retry, error classification, endpoint routing |
| Gaps | | No request/response logging; no metrics; no rate limiting |

## 3. Security

| Aspect | Grade | Notes |
|--------|-------|-------|
| Credential storage | B | Token secrets in `~/.config/lazypx/config.yaml`; OS keychain support planned (GoLand release) |
| Secret redaction | B | UUIDs and token patterns scrubbed from error messages |
| Audit logging | B | `~/.config/lazypx/audit.log` records START/STOP/REBOOT/DELETE/MIGRATE with user, timestamp, resource |
| SSH key storage | C | `~/.config/lazypx/ssh.yaml` may contain passwords in plaintext; no encryption |
| SSH host verification | B | `StrictHostKeyChecking=accept-new` — accepts new keys but rejects changed keys (good tradeoff) |
| Form injection | A | Fixed: `url.Values.Encode()` prevents `&`/`=` injection in API bodies |
| Context timeouts | A | All API calls bounded by centralized `TimeoutQuick/Standard/Long/Migration` constants |
| `.gitignore` | A | Ignores `config.yaml`, `ssh.yaml`, `*.secret`, binary |

## 4. TUI (`tui/`)

| Aspect | Grade | Notes |
|--------|-------|-------|
| Framework | A | Bubble Tea (elm-architecture); clean Update/View separation |
| Layout engine | A | Pure function `ComputeLayout()`; responsive to terminal resize; accordion sidebar with focus-based expansion |
| Detail pane | A | VM: status, CPU/RAM/disk gauges, disks, networks, guest IPs; Node: kernel, CPU model, storage pools, NICs; CT: resources; Storage: usage |
| Embedded terminal | A | Custom VT100/VT220 emulator (805 LOC); SGR colors, cursor movement, scroll regions, scrollback (2000 lines), alternate screen |
| Keyboard navigation | A | vim-style j/k; tab/shift+tab panel switching; 1-4 direct panel jumps |
| Actions | A | Start, Stop, Reboot, Delete, Migrate (auto-target), Snapshots (create/rollback/delete), Backups (create), Shell |
| Search | A | Case-insensitive substring match across all sidebar lists |
| Overlays | A | Help (?), Search (/), Snapshots (c), Backups (b), Sessions (t), Confirm dialogs |
| Shell sessions | A | PTY-backed SSH with tmux-free persistence; embedded pane or full-screen attach; scroll history |
| Error handling | B | Errors surface via confirm dialog; per-node cache warnings shown as local events |
| Gaps | | No VM creation dialog; no config editing; no storage management; no cluster join |

## 5. Sessions (`sessions/`)

| Aspect | Grade | Notes |
|--------|-------|-------|
| PTY management | A | Native Go PTY via `creack/pty`; no tmux dependency |
| Session lifecycle | A | Open/close/attach/detach/resize; process monitoring via `cmd.Wait()` goroutine |
| Concurrency | B | Manager mutex + per-session mutex; race-safe after fix for `attached` field |
| Cleanup | A | `CloseAll()` kills all children and closes PTY FDs on app quit |
| Coverage | 13.2% | Basic session lifecycle tested; PTY I/O tested via mock server |

## 6. Cache (`cache/`)

| Aspect | Grade | Notes |
|--------|-------|-------|
| Design | A | Snapshot-based with TTL expiry; `singleflight` prevents thundering herd |
| Concurrency | A | 100-concurrent-refresh test passes with race detector |
| Error handling | B | Per-node errors collected and surfaced; global errors set on snapshot |
| Data freshness | B | Configurable refresh interval (default 30s); `Invalidate()` for forced refresh |
| Coverage | 79.8% | Highest coverage package; tests TTL, concurrent access, invalidation |
| Gaps | | No incremental updates (full refresh each cycle); network interfaces re-fetched despite near-static nature |

## 7. Configuration (`config/`)

| Aspect | Grade | Notes |
|--------|-------|-------|
| Viper integration | A | Multi-profile YAML config with environment variable support |
| Profile system | A | Named profiles with per-profile host/token/TLS/refresh/production settings |
| SSH config | B | Per-VMID SSH mappings with user/port/identity file |
| Keyring support | B | OS keychain integration for token secrets (GoLand release) |
| Coverage | 21.4% | Config loading, SSH loading, keyring tested |

## 8. Error Handling

| Aspect | Grade | Notes |
|--------|-------|-------|
| Error types | A | `ProxmoxError` with status code; `ClassifyError()` maps to typed sentinels |
| Retry decisions | A | `IsRetryable()` checks error type; 4xx never retried, 5xx always retried |
| Error display | B | TUI surfaces errors via overlay; CLI prints to stderr |
| Error context | B | Errors wrap with `fmt.Errorf("...: %w", err)` throughout |
| Gaps | | No structured error codes; no error correlation IDs |

## 9. Testing

| Aspect | Grade | Notes |
|--------|-------|-------|
| Test infrastructure | B | `httptest.NewTLSServer` mocks; sequential server for retry testing |
| Race safety | A | All tests pass with `-race`; 100-concurrent refresh test |
| Coverage | C | Overall ~17% weighted average; cache (80%) is the bright spot |
| Unit tests | C | `api/`, `cache/`, `config/`, `sessions/`, `tui/` have tests; `commands/`, `audit/`, `state/` have none |
| Integration tests | D | None; no end-to-end workflow tests |
| Benchmarks | D | None |

### Per-Package Coverage

| Package | Coverage | Assessment |
|---------|----------|------------|
| `api` | 17.4% | Low — core HTTP client undertested |
| `cache` | 79.8% | High — concurrency and TTL well covered |
| `config` | 21.4% | Low-mid — loading tested, edge cases missing |
| `sessions` | 13.2% | Low — PTY lifecycle undertested |
| `tui` | 16.8% | Low — model init and terminal emulator tested |
| `commands` | 0.0% | None — CLI wiring untested |
| `audit` | 0.0% | None |
| `state` | 0.0% | None — pure data functions could be tested cheaply |

## 10. Code Quality

| Aspect | Grade | Notes |
|--------|-------|-------|
| Naming | A | Go idioms: exported/unexported clearly separated; `api.VMStatus` not `VM` |
| Error handling | A | Errors always checked; no `err` discarded without comment |
| Dead code | A | Removed: `isClientError`, duplicate `Version`, `injectConfig` body |
| Type safety | A | `ConfirmAction` typed as `func() tea.Cmd`; no `interface{}` at call sites |
| Lint | A | `go vet` clean; no compiler warnings |
| Comments | B | Good doc comments on public API; package-level comments on all packages |
| Code duplication | B | `actionStart/Stop/Reboot/Delete` share pattern; could be generic but fine for v0.1 |

## 11. Performance

| Aspect | Grade | Notes |
|--------|-------|-------|
| Startup time | A | ~200ms to first render; single API call for config, then parallel cluster fetch |
| Refresh efficiency | B | Parallel per-node fetches; singleflight prevents duplicate work; refresh guard prevents overlaps |
| Memory | B | Snapshot-based (no incremental state); shell pane cleanup on refresh cycles |
| TUI responsiveness | A | No blocking I/O in Update(); all network calls async via `tea.Cmd` |
| Embedded terminal | A | Custom VT100 emulator (not xterm.js); minimal allocations per frame |

## 12. Reliability

| Aspect | Grade | Notes |
|--------|-------|-------|
| Timeouts | A | All operations bounded; centralized constants in `api/timeouts.go` |
| Retry | A | Exponential backoff with jitter-free power-of-2; max 3 attempts |
| Quorum detection | A | `ErrQuorumLost` blocks state-changing operations |
| Lock detection | A | `ErrClusterLocked` blocks VM operations during backups |
| PTY cleanup | A | `CloseAll()` on quit; orphan prevention |
| WatchTask resilience | B | Retries up to 5 consecutive log fetch failures |
| Session safety | A | Race-free with mutex hierarchy (manager → session) |

## 13. Feature Completeness

| Feature | Status | Notes |
|---------|--------|-------|
| VM listing | Done | Per-node, sorted by VMID |
| VM start/stop/reboot | Done | With confirmation dialogs |
| VM delete | Done | With confirmation |
| VM migration | Done | Auto-target first available node |
| Container start/stop/reboot | Done | Mirrors VM actions |
| Snapshots (list/create/rollback/delete) | Done | Full lifecycle |
| Backups (list/create) | Done | VZDump with auto-storage detection |
| Embedded SSH shell | Done | PTY-backed, persistent sessions |
| Node status | Done | CPU, RAM, disk, storage pools, NICs |
| Storage overview | Done | Per-node, with usage gauges |
| Cluster resources | Done | CLI subcommand with JSON/table output |
| User/token management | Done | CLI subcommands for CRUD |
| Search | Done | Case-insensitive substring filter |
| Audit log | Done | Destructive actions logged |
| Fuzzy search | Missing | Current search is exact substring |
| VM creation | Missing | No `lazypx vm create` or TUI dialog |
| Config editing | Missing | No in-app profile management |
| Storage management | Missing | No volume create/delete/resize in TUI |
| HA management | Missing | API exists but no CLI/TUI commands |
| Network management | Missing | View-only; no create/edit bridge/VLAN |

## 14. Summary Scorecard

| Category | Grade | Weight | Score |
|----------|-------|--------|-------|
| Security | A- | 20% | 90 |
| Reliability | A | 20% | 95 |
| Performance | A- | 15% | 90 |
| Code Quality | A | 15% | 95 |
| Testing | C+ | 15% | 70 |
| Feature Coverage | B | 15% | 80 |
| **Overall** | **A-** | | **87** |

### Key Strengths
- Clean architecture with no circular dependencies
- Rock-solid retry/error classification system
- Custom VT100 terminal emulator is impressive for v0.1
- Concurrency-safe cache with singleflight
- Comprehensive keyboard navigation

### Key Weaknesses
- Test coverage is low (~17% overall) — especially `commands/`, `audit/`, `state/` at 0%
- No integration or end-to-end tests
- VM/CT creation not implemented
- No in-app config editing
- SSH password storage in plaintext
