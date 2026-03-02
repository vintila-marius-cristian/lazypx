# lazypx — Architecture & Design Reference

> Proxmox VE terminal UI — Go · Bubble Tea · Lip Gloss · Cobra · Viper  
> Tested against PVE 9.1.4 (`px` node, 20 VMs/CTs, 7 storage pools)

---

## Design Goals & Decisions

| Goal | Decision |
|---|---|
| Zero UI lag | All API calls run in background `tea.Cmd` goroutines — `Update()` never blocks |
| Works over SSH | Pure ANSI output via Lip Gloss — no mouse required |
| Single binary | Go with `CGO_DISABLED=0` (keyring uses CGo on macOS via `go-keyring`) |
| Multi-profile | Viper YAML at `~/.config/lazypx/config.yaml` |
| Production-safe | Confirmation modals, audit log, error sentinels, secret redaction |
| CLI parity | Cobra command tree mirrors every TUI action |

---

## Repository Layout (33 source files)

```
proxmox-cli/
├── main.go                    # one-liner: commands.Root().Execute()
│
├── api/                       # Proxmox REST API client layer
│   ├── client.go              # HTTP transport, auth header, retry/backoff
│   ├── errors.go              # Sentinel errors, ClassifyError, IsRetryable, RedactMessage
│   ├── types.go               # Shared response structs (VMStatus, NodeStatus, …)
│   ├── nodes.go               # /nodes  +  /nodes/{n}/status (nested decoder)
│   ├── vms.go                 # /nodes/{n}/qemu + power ops, migrate, backup
│   ├── containers.go          # /nodes/{n}/lxc + power ops
│   ├── storage.go             # /nodes/{n}/storage
│   ├── tasks.go               # /nodes/{n}/tasks, WatchTask (log streaming)
│   ├── cluster.go             # /cluster/status, /cluster/resources, /pools CRUD
│   ├── ha.go                  # /cluster/ha/resources|groups|status
│   ├── snapshots.go           # Snapshot CRUD + rollback (QEMU + LXC)
│   ├── volumes.go             # Volume list/delete/resize/move disk
│   ├── vmconfig.go            # VM/CT full config, CloneVM, CloneCT
│   ├── access.go              # Users, groups, roles, ACLs, API tokens
│   ├── network.go             # Node network interfaces, firewall rules (r/o)
│   └── client_test.go         # Unit tests: auth, TLS, retry, errors, redaction
│
├── cache/
│   ├── cache.go               # TTL-based in-memory cluster snapshot
│   └── cache_test.go          # Concurrent access, TTL expiry, Invalidate
│
├── state/
│   └── state.go               # AppState + Selection + ActiveTask (no logic)
│
├── config/
│   ├── config.go              # YAML config loader (Viper), multi-profile
│   └── keyring.go             # OS keychain read/write (go-keyring)
│
├── audit/
│   └── audit.go               # Append-only audit log (~/.config/lazypx/audit.log)
│
├── tui/
│   ├── app.go                 # Root Bubble Tea model (Init / Update / View)
│   ├── styles.go              # All Lip Gloss styles, colour palette, GaugeBar
│   ├── tree.go                # Left pane: cluster navigation tree
│   ├── detail.go              # Main pane: resource detail (VM/CT/node/storage)
│   ├── tasks.go               # Bottom pane: live task log
│   └── help.go                # Overlays: help modal, confirm modal, search
│
└── commands/
    ├── root.go                # Cobra root + TUI launcher + PersistentPreRunE
    ├── vm.go                  # lazypx vm list/start/stop/reboot/migrate/backup
    ├── node.go                # lazypx node list/status
    ├── cluster.go             # lazypx cluster status/resources
    ├── snapshot.go            # lazypx snapshot list/create/delete/rollback
    └── access.go              # lazypx access user/group/role/acl
```

---

## Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Keyboard input                                              │
└───────────────────────┬─────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│  TUI (tui/)   Elm architecture: Init → Update → View        │
│  Renders ANSI terminal via Lip Gloss                         │
└───────────────────────┬─────────────────────────────────────┘
              reads/mutates
┌───────────────────────▼─────────────────────────────────────┐
│  State (state/state.go)   Single source of truth            │
│  Selection · FocusedPanel · ActiveTasks · Overlays          │
└───────────────────────┬─────────────────────────────────────┘
              populated by
┌───────────────────────▼─────────────────────────────────────┐
│  Cache (cache/cache.go)   TTL snapshot, fan-out per node    │
└───────────────────────┬─────────────────────────────────────┘
              calls
┌───────────────────────▼─────────────────────────────────────┐
│  API (api/)   HTTP + auth + retry + typed responses          │
└─────────────────────────────────────────────────────────────┘
```

---

## Component Deep-Dives

### `api/client.go` — HTTP Foundation

```go
type Client struct {
    baseURL     string
    tokenID     string
    tokenSecret string
    http        *http.Client   // single pooled transport, MaxIdleConnsPerHost=10
}
```

**Auth header** (PVE API token format):
```
Authorization: PVEAPIToken=root@pam!tokenid=uuid-secret
```

**`doWithRetry`** — exponential backoff (500ms → 1s → 2s, max 3 attempts).  
After each failure `ClassifyError()` is called. If `IsRetryable()` returns `false` (any 4xx), the loop short-circuits immediately — no wasted retries on 401/403/404.

**`do()`** — builds the request, adds the auth header, executes, decodes JSON into `out`. On `>=400` the response body is passed through `RedactMessage()` before wrapping in `ProxmoxError`.

Available HTTP methods: `get()`, `post()`, `put()`, `del()` — all go through `doWithRetry`.

---

### `api/errors.go` — Error Classification

| Sentinel | Trigger | Retryable |
|---|---|---|
| `ErrUnauthorized` | HTTP 401 | ✗ |
| `ErrForbidden` | HTTP 403 | ✗ |
| `ErrNotFound` | HTTP 404 | ✗ |
| `ErrLocked` | body contains "locked" | ✗ |
| `ErrQuorumLoss` | body contains "quorum" | ✗ |
| `ProxmoxError{5xx}` | HTTP 500–599 | ✓ |
| network/timeout error | transport failure | ✓ |

`RedactMessage(s string)` truncates error bodies to 500 chars, preventing token secrets that may appear in base64 or URL-encoded Proxmox error responses from reaching logs or TUI notifications.

---

### `api/types.go` — Response Structs (verified against PVE 9.1.4)

**`VMStatus`** — from `/nodes/{n}/qemu`:
```
vmid, name, status, cpu(float), cpus(int), mem, maxmem,
disk(used bytes), maxdisk(size bytes), diskwrite, diskread,
netout, netin, uptime, tags, template, lock, qmpstatus, ha{managed,state}
```

**`CTStatus`** — from `/nodes/{n}/lxc`:
```
vmid, name, status, cpu, cpus, mem, maxmem,
disk(used), maxdisk(allocated), diskwrite, diskread,
netout, netin, uptime, swap, maxswap, tags, type
```

**`NodeStatus`** — from `/nodes` list (flat):
```
node, status, cpu, maxcpu, mem, maxmem, disk, maxdisk, uptime, ssl_fingerprint
Extended *NodeStatusExtended  ← populated only by GetNodeStatus()
```

**`NodeStatusExtended`** — decoded from `/nodes/{n}/status` nested sub-objects (`memory{}`, `cpuinfo{}`, `rootfs{}`, `current-kernel{}`):
```
PVEVersion, KernelVer, CPUModel, CPUCores, CPUSockets, CPUMHz,
SwapUsed, SwapTotal, RootFSAvail
```

**`StorageStatus`** — from `/nodes/{n}/storage`:
```
storage, type, status, used, total, avail, active, enabled,
shared, content("images,rootdir,backup"), used_fraction(0.0–1.0)
```

---

### `api/nodes.go` — Node Status Decoder

`GetNodes()` calls `/nodes` → returns a flat list, each item maps directly to `NodeStatus`.

`GetNodeStatus()` calls `/nodes/{n}/status` which returns a **different, nested shape**. A private `nodeStatusRaw` struct decodes the nested sub-objects, then the values are mapped into a flat `NodeStatus` + a `NodeStatusExtended` pointer. This means the TUI can show the simple flat version from the list and load extended detail on selection.

---

### `cache/cache.go` — Concurrent TTL Cache

```go
type Cache struct {
    mu        sync.RWMutex
    snapshot  ClusterSnapshot
    fetchedAt time.Time
    ttl       time.Duration
    client    *api.Client
}
```

`Get(ctx)`:
1. Read-lock — if `time.Since(fetchedAt) < ttl`, return the existing snapshot immediately.
2. Write-lock — call `refresh()`.
3. `refresh()` fans out one goroutine per node (fetches VMs + CTs + Storage concurrently per node), merges results under the write lock.

**Why fan-out?** On a 10-node cluster, sequential queries = `10 × RTT`. Fan-out = `1 × max(RTT)` — practically instant on a LAN.

`Invalidate()` resets `fetchedAt` to zero, forcing the next `Get()` call to re-fetch. Called after every destructive action (start/stop/delete/snapshot).

---

### `state/state.go` — Single Source of Truth

```go
type AppState struct {
    Snapshot       cache.ClusterSnapshot
    FocusedPanel   Panel            // PanelTree | PanelDetail | PanelTasks
    Selected       Selection
    TreeOffset     int
    ActiveTasks    []ActiveTask
    TaskOffset     int
    SearchActive   bool
    SearchQuery    string
    HelpVisible    bool
    ConfirmVisible bool
    ConfirmMsg     string
    ConfirmAction  func()
    Loading        bool
    Error          string
    ProfileName    string
    Production     bool
}

type Selection struct {
    Kind        ResourceKind   // KindNone | KindNode | KindVM | KindContainer | KindStorage
    NodeName    string
    VMID        int
    VMStatus    *api.VMStatus
    CTStatus    *api.CTStatus
    NodeStatus  *api.NodeStatus
    VMConfig    *api.VMConfig  // loaded on demand; nil until fetched
    StorageName string         // for KindStorage
}
```

**No logic in state** — all mutations happen in `tui/app.go`'s `Update()`. Sub-models receive a `*AppState` pointer so they always see the latest data without copying.

---

### `tui/app.go` — Bubble Tea Root Model

Bubble Tea uses the **Elm Architecture**: `Init → Update → View`.

**`Init()`** — returns two concurrent `Cmd`s:
- `spinner.Tick` — starts the loading spinner
- `loadCluster()` — goroutine that calls `cache.Refresh()` and returns `ClusterLoaded{}`

**`Update(msg)`** handles these message types:

| Message | Action |
|---|---|
| `tea.WindowSizeMsg` | Recalculates pane dimensions, calls `SetSize` on all sub-models |
| `ClusterLoaded` | Stores snapshot, calls `tree.Sync()`, schedules `tickRefresh(ttl)` |
| `ClusterRefreshed` | Same as ClusterLoaded, called on background refresh |
| `RefreshTick` | Fires `refreshCluster()` goroutine |
| `TaskLogLine` | Appends log line to `ActiveTask` by index |
| `TaskDone` | Marks task done, invalidates cache, triggers refresh |
| `ActionError` | Stores error message in state for display |
| `tea.KeyMsg` | Routes to `handleKey`, `handleSearchKey`, or `handleConfirmKey` |

**Critical fix (v2)**: At the end of every `Update()`, both `tree.UpdateSelection()` AND `detail.Sync()` are always called — regardless of which panel is focused. Previously, the detail pane was only synced when it was the focused panel, causing it to show a stale placeholder even when a VM was selected in the tree.

**Layout math**:
```
header(1) + mainHeight + taskPaneHeight + keybar(1) = terminalHeight
mainHeight   = h − 1 − taskPaneHeight(h) − 1
taskH        = 8 lines outer (6 on short terminals < 22 rows)
treeWidth    = max(38, w × 30%)   [hard minimum 38 cols to avoid label truncation]
detailWidth  = terminalWidth − treeWidth
```

Each bordered pane consumes 2 chars of width and 2 of height. `View()` methods subtract 2 before passing to `lipgloss.Style.Width()/Height()`:
```go
innerW := outerW - 2
innerH := outerH - 2
style.Width(innerW).Height(innerH).Render(content)
```

---

### TUI Keybindings (full reference)

| Key | Action | Context |
|---|---|---|
| `↑` / `k` | Move cursor up | Tree |
| `↓` / `j` | Move cursor down | Tree |
| `enter` / `space` | Expand/collapse node | Tree |
| `tab` | Cycle focus: Tree → Detail → Tasks | Global |
| `s` | Start VM/CT | Tree (VM/CT selected) |
| `S` | Stop VM/CT (confirm) | Tree |
| `r` | Reboot VM/CT (confirm) | Tree |
| `d` | Delete VM/CT (confirm) | Tree |
| `l` | Open task log viewer | Tree |
| `c` | Clone VM (TODO: modal) | Tree |
| `m` | Migrate VM | Tree |
| `b` | Backup VM | Tree |
| `/` | Enter search mode | Global |
| `esc` | Exit search | Search mode |
| `R` | Force refresh cache | Global |
| `?` | Toggle help overlay | Global |
| `y` / `n` | Confirm / cancel action | Confirm modal |
| `q` / `ctrl+c` | Quit | Global |

---

### `tui/tree.go` — Cluster Navigation Tree

**Data structure**: All nodes and their children are flattened into `[]TreeEntry`. The cursor is an index into this slice.

```go
type TreeEntry struct {
    Kind      state.ResourceKind
    Label     string
    Status    string
    NodeName  string
    VMID      int
    Indent    int     // 0=node, 2=VM/CT/storage-header, 4=storage item
    IsHeader  bool
    CanExpand bool
    Expanded  bool
    NS        *api.NodeStatus
    VM        *api.VMStatus
    CT        *api.CTStatus
}
```

**Expand/collapse**: `nodeExpanded map[string]bool` tracks state per node. On toggle, `rebuildEntries()` re-flattens the tree, then `ApplyFilter()` re-applies any active search query.

**Virtual scroll**: Only rows that fit in the viewport are rendered. The cursor is centred in the viewport:
```go
start = max(0, cursor - innerH/2)
end   = min(len(entries), start + innerH)
```

**Label truncation**: `renderTreeEntry` truncates at `innerW − indent − 3` (3 = status dot + 2 spaces). Minimum tree width of 38 columns (36 inner) ensures labels up to ~30 chars display without clipping.

**Search** (`/`): `ApplyFilter` does case-insensitive substring match on labels. Re-runs on every keystroke for live filtering.

---

### `tui/detail.go` — Resource Detail Pane

Switches on `sel.Kind`. All render functions receive the inner width `w` for correct gauge sizing.

**VM detail** (`KindVM`):
- Header: status dot + VMID + name + lock badge + template badge + HA state
- CPU gauge: `% of %d cores`
- RAM gauge: `used / total (pct%)`
- Root disk: `used / maxdisk` if `DiskUsed > 0`, else `"X GB allocated"`
- Disk I/O: `↓ read ↑ write` (only shown when non-zero, i.e. running)
- Network I/O: `↓ netin ↑ netout` (only when non-zero)
- Config section (if `VMConfig` loaded): parsed disk list (scsi0/virtio0/…) with storage pool + size; parsed net list (net0/net1) with bridge, VLAN tag, MAC

**CT detail** (`KindContainer`):
- Same structure as VM: CPU, RAM, root disk, net I/O
- Disk shows `disk / maxdisk` with gauge

**Node detail** (`KindNode`):
- Uptime, PVE version, kernel release, CPU model, cores/sockets/MHz (from `NodeStatusExtended`)
- CPU gauge, RAM gauge, Swap gauge, OS disk gauge
- Storage pool table: `● name  ████░░  42.0%  100GB/980GB  lvmthin` for all pools on this node

**Storage detail** (`KindStorage`):
- Status, type, node, shared flag, content types
- Usage gauge: `used / total (pct%)`
- Free space

**Why** `detail.Sync` is always called: navigation events fire on the tree panel. The previous code only called `detail.Sync` when the detail panel was focused — meaning the detail pane showed a placeholder even when a VM was selected. Now both calls are unconditional.

---

### `tui/styles.go` — Design System

Tokyo Night–inspired palette. All visual tokens defined once:

| Token | Hex | Use |
|---|---|---|
| `colorBlue` | `#7AA2F7` | Accent, focused borders, titles |
| `colorGreen` | `#9ECE6A` | Running status |
| `colorRed` | `#F7768E` | Error, stopped |
| `colorYellow` | `#E0AF68` | Paused, warning |
| `colorMagenta` | `#BB9AF7` | Tags, special values |
| `colorSubtext` | `#565F89` | Dim labels, secondary info |
| `colorBg` | `#16161E` | Background |

**`GaugeBar(width, pct)`** — renders a proportional block bar using Unicode block elements. Width is `w × 25%`, min 12, max 28 chars.

**`StatusDot(status)`** — `●` green for running, `○` dim for stopped, `●` yellow for paused/suspended.

---

### `tui/help.go` — Overlays

Three overlays rendered via `lipgloss.Place` centred over a blurred background string:
- **Help** (`?`): Static keybinding table
- **Confirm** (`d`,`S`,`r`): Dynamic message + `ConfirmAction func()` called on `y`
- **Search** (`/`): Captures keypresses, updates `SearchQuery`, live-filters tree

---

### `config/config.go` — Multi-Profile YAML

```yaml
default_profile: home
profiles:
  - name: home
    host: https://10.0.18.6:8006
    token_id: root@pam!lazypx
    token_secret: ""         # leave blank → load from OS keychain
    tls_insecure: true
    refresh_interval: 30     # seconds between background refreshes
    production: false        # true → red PRODUCTION badge in header
```

Viper resolves `token_secret` from env var `LAZYPX_TOKEN_SECRET` when blank in config.

---

### `config/keyring.go` — OS Keychain

`go-keyring` maps to macOS Keychain / Linux Secret Service / Windows Credential Manager.  
Keys are namespaced: `lazypx:token_secret:{profileName}`.  
If `token_secret` is blank in YAML, `LoadSecret(profile)` is called — credentials never need to be stored in plaintext.

---

### `audit/audit.go` — Append-Only Audit Log

Path: `~/.config/lazypx/audit.log` — mode `0600` (owner read/write only).

```
[2026-03-02T19:14:41Z] [default] [root@pam] START vm:221
[2026-03-02T19:15:02Z] [prod] [root@pam] DELETE vm:105
```

Write failures are **silently dropped** — audit must never interrupt the primary operation. Protected by `sync.Mutex` so concurrent TUI goroutines don't interleave lines.

---

## CLI Commands (full reference)

```
lazypx                              Launch TUI

lazypx vm list [--output table|json]
lazypx vm start  <vmid>
lazypx vm stop   <vmid>
lazypx vm reboot <vmid>
lazypx vm migrate <vmid> --target <node>
lazypx vm backup  <vmid> --storage <storage>

lazypx node list [--output table|json]
lazypx node status <node>

lazypx cluster status
lazypx cluster resources [--type vm|lxc|storage|node|pool] [--output table|json]

lazypx snapshot list     <vmid> [--kind auto|qemu|lxc]
lazypx snapshot create   <vmid> <snapname> [--description "..."]
lazypx snapshot delete   <vmid> <snapname>
lazypx snapshot rollback <vmid> <snapname>

lazypx access user  list
lazypx access user  create <userid> [--email ... --comment ...]
lazypx access user  delete <userid>
lazypx access group list
lazypx access role  list
lazypx access acl   list

lazypx init-config          Write example config to ~/.config/lazypx/config.yaml
lazypx version

Global flags:
  --profile / -p <name>     Override active profile
```

`resolveVMWithKind` auto-discovers a VMID's node and type (QEMU/LXC) by querying all nodes in parallel — no need to specify a node manually.

---

## Data Flows

### Startup
```
main() → Root().Execute() → runTUI(cfg)
  ├─ api.NewClient(host, tokenID, secret, tlsInsecure)
  ├─ cache.New(client, ttl)
  ├─ tui.New(client, cache, cfg)
  └─ bubbletea.NewProgram(model).Run()
       ├─ Init()  →  tea.Batch(spinner.Tick, loadCluster())
       └─ loadCluster goroutine → cache.Refresh() → ClusterLoaded{}
            → tree.Sync() → tickRefresh(30s)
```

### Background Refresh
```
tickRefresh(30s) fires RefreshTick{}
Update(RefreshTick{}) → refreshCluster() goroutine
  └─ cache.Refresh(ctx)
       ├─ goroutine: GetVMs + GetContainers + GetStorage (node-1)
       ├─ goroutine: GetVMs + GetContainers + GetStorage (node-2)
       └─ merge → ClusterRefreshed{}
            → tree.Sync() → tickRefresh(30s)
```

### Tree Navigation → Detail Pane Update
```
User presses ↓ (KeyMsg)
Update() → handleKey() → tree.MoveDown()
End of Update():
  tree.UpdateSelection(state)   ← updates state.Selected
  detail.Sync(state)             ← detail reads state.Selected → renders VM info
View() called on next frame → detail pane shows VM CPU/RAM/disk gauges
```

### Action (e.g. Start VM)
```
User presses 's'
Update() → handleKey() → returns tea.Cmd (goroutine)
  goroutine: client.StartVM(node, vmid) → returns UPID
  → state.AddTask(upid, node, label) → watchTask(upid) goroutine
      stream: TaskLogLine{} → state.AppendTaskLog()
      done:   TaskDone{} → state.MarkTaskDone() → cache.Invalidate() → refreshCluster()
```

---

## API Coverage Summary

| Domain | Endpoints |
|---|---|
| Nodes | list, per-node status (with nested decoder) |
| QEMU VMs | list, start/stop/reboot, migrate, backup, config, clone |
| LXC Containers | list, start/stop/reboot, config, clone |
| Snapshots | list, create, delete, rollback (QEMU + LXC) |
| Volumes | list content, delete, resize disk, move disk |
| Cluster | status, all resources (filterable) |
| HA | resources, groups, status |
| Resource Pools | list, get, create, delete |
| RBAC | users CRUD, groups CRUD, roles CRUD, ACL list/update, tokens list |
| Network | node interface list |
| Firewall | read-only (cluster/node/VM) |
| Tasks | list, log streaming, status polling |

**~55 endpoints implemented** (~30% of the full PVE API surface).

---

## Testing

**Unit tests** (`api/client_test.go`, `cache/cache_test.go`):
- `httptest.NewTLSServer` — in-process HTTPS server, no real PVE required
- `sequentialServer` helper — serves responses in order (used to test retry: 2×502 then 200)
- Race detector (`-race`) on all tests
- Tests: auth header format, TLS insecure, retry success after transient errors, retry exhaustion, 401/403/404/500 error classification, `GetNodes` field mapping, secret redaction truncation, 100-goroutine concurrent cache access, TTL expiry, `Invalidate`

**Integration** (against real PVE 9.1.4 cluster `px`):
```
go test -race ./api/... ./cache/...    ✓ 11 tests, race-clean
./lazypx cluster status                ✓ px online
./lazypx cluster resources             ✓ 20 resources
./lazypx node status px                ✓ CPU 2.6% / RAM 67.3GB / 125.6GB / PVE 9.1.4
./lazypx snapshot list 221             ✓ current snapshot
./lazypx access user list              ✓ 3 users
./lazypx access role list              ✓ 18 roles
./lazypx access acl list               ✓ 3 ACL entries
```

---

## Security Controls

| Control | Implementation |
|---|---|
| API token auth (no session cookies) | `PVEAPIToken=` header |
| TLS verification (opt-in disable) | `tls_insecure: true` in config |
| Secret redaction in error messages | `RedactMessage()` truncates to 500 chars |
| OS keychain for token secrets | `config/keyring.go` via `go-keyring` |
| Append-only audit log (mode 0600) | `audit/audit.go` |
| Confirmation modal for destructive ops | `ConfirmVisible`, `ConfirmAction func()` |
| PRODUCTION badge in red | `production: true` in config |
| Context timeout on every API call | 15s reads, 60–120s for long ops |

---

## Known Limitations

| Item | Status |
|---|---|
| TUI snapshot/clone/resize via keyboard | API ready; TUI hotkeys are `// TODO` |
| Audit log not called from TUI actions | Wire into `actionStart()`/`actionStop()` |
| `VMConfig` not auto-loaded on VM select | Requires background `GetVMConfig` cmd |
| Metrics/RRD graphs (`/rrddata`) | Not implemented |
| ISO upload (multipart streaming) | Out of scope |
| SDN / zones / vnets | Not implemented |
| Firewall write operations | Read-only only |
| Shell completion (`zsh`/`bash`) | Cobra stub present, not wired |
| Goreleaser multi-arch builds | Not configured |
