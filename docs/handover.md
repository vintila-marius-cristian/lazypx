# lazypx — Project Handover

> **as of 2026-03-03** | Go 1.25 | Bubble Tea v1.3 | PVE API v9.1

---

## 1. What Is lazypx?

`lazypx` is a `lazygit`-style terminal UI (TUI) + CLI tool for managing Proxmox VE clusters. It is written in Go using **Bubble Tea** for the TUI and **Lip Gloss** for styling. Target users are Proxmox admins who want a fast, keyboard-driven interface without opening the web GUI.

Configuration lives in **`~/.config/lazypx/config.yaml`** (profiles) and **`~/.config/lazypx/ssh.yaml`** (VM SSH mappings).

---

## 2. Repository Layout

```
lazypx/
├── api/               # Proxmox REST API client
│   ├── client.go      # HTTP client, token auth, TLS, retry
│   ├── types.go       # All PVE structs (NodeStatus, VMStatus, Task, …)
│   ├── tasks.go       # GetClusterTasks, GetTaskLog, TrackTask
│   ├── snapshots.go   # Snapshot CRUD + rollback
│   ├── backups.go     # VZDump get/create/restore
│   ├── network.go     # Node NICs + QEMU guest-agent IPs
│   └── …
├── cache/
│   └── cache.go       # ClusterSnapshot: Nodes, VMs, CTs, Storage, Network, Tasks
│                      # TTL-based auto-refresh, Invalidate(), thread-safe
├── sessions/
│   ├── manager.go     # PTY-backed SSH session manager (creack/pty)
│   └── manager_test.go
├── state/
│   └── state.go       # AppState (Selected, Snapshot, ActiveTasks, overlay flags)
├── config/
│   ├── config.go      # Viper YAML loader, multi-profile
│   ├── ssh.go         # ssh.yaml loader → map[vmid]SSHHost
│   └── keyring.go     # OS keychain (go-keyring)
├── tui/
│   ├── app.go         # Root Bubble Tea Model: Init/Update/View
│   ├── layout.go      # ComputeLayout(): pure sizing function, all pane dims
│   ├── layout_test.go # 9 invariant + fuzz tests
│   ├── sidebar.go     # Left panel: 4 stacked ListPane accordions
│   ├── list_pane.go   # Generic accordion list with wrap-around navigation
│   ├── detail.go      # Right panel: VM/CT/Node/Storage detail renderer
│   ├── tasks.go       # Bottom panel: active tasks + cluster task log
│   ├── snapshots.go   # Overlay: snapshot list + create/rollback/delete
│   ├── backups.go     # Overlay: backup list + trigger VZDump
│   ├── sessions_overlay.go # Overlay: PTY session picker (list/attach/close)
│   ├── help.go        # Overlay: keybinding help + confirm modal
│   ├── search.go      # Overlay: fuzzy search
│   └── styles.go      # All Lip Gloss styles (Tokyo Night palette)
├── commands/
│   ├── root.go        # Cobra root + TUI launcher
│   ├── ssh.go         # `lazypx ssh <vmid>` → PTY session
│   ├── vm.go
│   ├── node.go
│   ├── cluster.go
│   └── snapshot.go
└── docs/
    ├── architecture.md
    ├── layout.md
    └── handover.md     # this file
```

---

## 3. Completed Phases

### Phases 1–7 (Baseline)
- Full TUI: tree navigation, detail panel, tasks, help, confirm, search, snapshots, backups.
- PVE API client: nodes, VMs, containers, storage, snapshots, backups, tasks.
- Multi-profile config with OS keychain for token secrets.
- `ssh.yaml` VM SSH mapping: `vmid → {user, host, port, identity_file, password}`.
- Centralized layout engine with invariant tests.
- All uppercase keybinds replaced with lowercase.

### Phase 8 — Tasks, Navigation, Networking

| Item | Detail |
|---|---|
| **Tasks & Events** | Recursive `tea.Cmd` log streaming; `watchChannelCmd`, `taskLogStreamMsg`/`taskDoneStreamMsg`; cache invalidated on completion |
| **Wrap-around navigation** | `MoveUp()`/`MoveDown()` in `list_pane.go` wrap last→first and first→last |
| **Node networking** | Fetched via `/nodes/{node}/network`, displayed in Detail pane |
| **VM/CT guest IPs** | Fetched async via QEMU guest agent; sorted alphabetically to prevent UI jitter |
| **Tasks race fix** | `cache.go`: globalTasks goroutine registered with WaitGroup **before** `close(results)` goroutine |

### Phase 10 — Persistent PTY Shell Sessions

| Item | Detail |
|---|---|
| **sessions/manager.go** | Native Go PTY manager using `creack/pty` + `golang.org/x/term`. No tmux required. |
| **Persistence model** | `e` opens ssh in background PTY. `Ctrl+Q` detaches (TUI resumes), session stays alive. Pressing `e` again re-attaches same session. |
| **Multiple sessions** | Different VMs get independent PTY sessions, all running concurrently. |
| **Sessions picker** | `t` overlay: lists all active sessions, `enter` = attach, `d` = close (confirm), `esc` = back |
| **CLI parity** | `lazypx ssh <vmid>` also uses PTY manager → same persistence, same `Ctrl+Q` detach |
| **Tests** | `sessions/manager_test.go`: 6 tests covering keying, sanitization, same-VM idempotency, different-VM isolation, attacher, empty list |

---

## 4. Key Keybindings

| Key | Action |
|---|---|
| `j` / `k` / `↑↓` | Navigate list |
| `1`–`4` | Jump to Nodes / VMs / CTs / Storage panel |
| `tab` | Switch panel focus |
| `e` | Open / re-attach PTY shell (VM/CT) |
| `t` | Sessions picker overlay |
| `c` | Snapshots modal |
| `b` | Backups modal |
| `s` / `x` / `r` / `d` | Start / Stop / Reboot / Delete (with confirmation) |
| `f` | Force cache refresh |
| `/` | Fuzzy search |
| `?` | Help overlay |
| `q` | Quit |

---

## 5. Session Persistence — How It Works

```
User presses 'e' on VM 105
  └→ sessions.Manager.OpenSession("lazypx-default-105", "ssh", [args])
       └→ pty.Start(exec.Command("ssh", ...))  // background PTY process
  └→ tea.Exec(PTYAttacher) suspends TUI, raw stdin → PTY
  └→ Ctrl+Q → detach → TUI resumes, SSH still running

User presses 'e' on VM 150
  └→ New PTY session for VM 150, VM 105 still running

User navigates back to VM 105, presses 'e'
  └→ OpenSession() sees alive process → no-op
  └→ tea.Exec re-attaches to the SAME running session

User presses 't'
  └→ Lists: "lazypx-default-105 (running, 12m)" and "lazypx-default-150 (running, 3m)"
  └→ enter to reattach, d to close
```

---

## 6. Configuration Examples

### `~/.config/lazypx/config.yaml`
```yaml
default_profile: default
profiles:
  - name: default
    host: https://192.168.1.10:8006
    token_id: root@pam!lazypx
    token_secret: <uuid>
    tls_insecure: false
    refresh_interval: 1
```

### `~/.config/lazypx/ssh.yaml`
```yaml
hosts:
  105:
    user: root
    host: 192.168.1.105
    port: 22
    identity_file: ~/.ssh/id_ed25519
  150:
    user: ubuntu
    host: 192.168.1.150
```

---

## 7. Dependencies Added (Phase 10)

| Package | Purpose |
|---|---|
| `github.com/creack/pty v1.1.24` | PTY start for background SSH sessions |
| `golang.org/x/term v0.40.0` | Raw terminal mode for attach |
| `golang.org/x/sys v0.41.0` | Upgraded (transitive) |

---

## 8. Known Limitations & Pending Work

| Item | Status | Notes |
|---|---|---|
| PTY sessions orphaned on TUI exit | ⚠️ Known | Processes not tracked across restarts; future: persist PIDs to disk |
| Embedded terminal preview in Detail pane | ❌ Not done | Plan called for in-pane preview; scoped out, only suspend+attach done |
| Task streaming completeness | ⚠️ Partial | Local active tasks stream; cluster tasks polled at refresh interval |
| Password SSH in sessions | ⚠️ Best-effort | ssh prompts interactively; `sshpass` used if installed and password in config |
| `api/firewall.go` | ❌ Not started | Phase 1 item |
| Error toast notifications | ❌ Not started | Phase 2 item |
| GoReleaser / Homebrew Tap | ❌ Not started | Phase 9 item |

---

## 9. Build & Run

```bash
# Build
go build -o lazypx .

# TUI
./lazypx

# SSH with persistence
./lazypx ssh 105          # by VMID
./lazypx ssh mgmt         # by name (resolves via API)

# Tests
go test ./...
```

---

## 10. Architecture Quick Reference

- **Bubble Tea** message loop: `Init → Update(msg) → View → render`.
- **`Update()` never blocks** — all I/O in `tea.Cmd` goroutines.
- **Overlays** rendered last via `lipgloss.Place`: Confirm > Help > Sessions > Search > Snapshots > Backups.
- **Cache** is a TTL `ClusterSnapshot` refreshed in background. `Invalidate()` forces next-tick refresh.
- **`AppState`** is a single pointer shared across all sub-models; mutations happen in `Update()` only.
- **Layout** computed by `tui.ComputeLayout(termW, termH)` once per `tea.WindowSizeMsg`.
