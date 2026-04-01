# lazypx

**lazypx** is a `lazygit`-style Terminal UI (TUI) and CLI for managing **Proxmox VE** clusters. Built in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and Lip Gloss.

Navigate your entire datacenter without leaving the terminal — with an embedded interactive SSH shell that stays visible alongside the resource tree.

---

## Features

- **Accordion TUI**: Stacked panes for Nodes, VMs, Containers, and Storage with relational filtering (select a node → VM/CT lists auto-filter).
- **Embedded SSH Terminal**: Press `e` on any VM/CT to open a full PTY shell in the right pane — vim, top, and interactive apps all work. The left tree and bottom task log stay visible.
- **Persistent Sessions**: Navigate away and back — the shell session keeps running. Reattach instantly.
- **Real-Time Task Log**: Proxmox async tasks (vzdump, qmstart, etc.) stream live in the bottom pane alongside local shell events.
- **Power Actions**: Start (`s`), Stop (`x`), Reboot (`r`), Delete (`d`) with confirmation overlays.
- **Fuzzy Search**: `/` to filter any resource instantly.
- **CLI Mode**: Headless subcommands for scripting: `lazypx vm start 105`, `lazypx ssh mgmt`.

---

## Installation

### Build from Source

Requires Go 1.25+:

```bash
git clone https://github.com/your-username/lazypx.git
cd lazypx
go build -o lazypx .
sudo mv lazypx /usr/local/bin/
```

### macOS / Linux Binary

Download the binary for your platform from [Releases](https://github.com/your-username/lazypx/releases), then:

```bash
chmod +x lazypx-darwin-arm64   # or lazypx-linux-amd64
sudo mv lazypx-darwin-arm64 /usr/local/bin/lazypx
```

---

## Configuration

**No credentials are ever stored in the codebase.** All configuration lives in `~/.config/lazypx/` on your local machine and is never committed to git.

### `~/.config/lazypx/config.yaml`

Create this file and fill in your Proxmox API token details. Get a token from Proxmox at **Datacenter → Permissions → API Tokens**.

```yaml
profiles:
  default:
    host: "https://10.0.0.10:8006"     # Your Proxmox host URL
    token_id: "root@pam!mytoken"        # API token ID
    token_secret: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"  # API token secret
    insecure: true                       # true = skip TLS verification (self-signed cert)
    refresh_s: 30                        # Cache refresh interval in seconds
active_profile: default
```

**Token secret storage**: Instead of keeping `token_secret` in plain text, you can omit it from the file and store it in your OS keychain (macOS Keychain / Linux Secret Service). lazypx will look for the secret under the service name `lazypx` with the key matching your `token_id`. Set the secret once with:

```bash
# macOS
security add-generic-password -s lazypx -a "root@pam!mytoken" -w "your-secret"

# Linux (requires secret-tool from libsecret)
secret-tool store --label="lazypx" service lazypx username "root@pam!mytoken"
```

### `~/.config/lazypx/ssh.yaml`

Maps VMIDs to SSH connection details. Required for the embedded shell (`e` key) and `lazypx ssh` CLI command.

```yaml
# Password auth (requires sshpass installed: brew install sshpass / apt install sshpass)
- id: 105
  host: 10.0.20.198
  user: packer
  password: packer
  port: 22

# Key-based auth
- id: 101
  host: 192.168.1.50
  user: admin
  identity_file: ~/.ssh/id_ed25519

# Minimal — uses current local username, port 22
- id: 200
  host: 192.168.1.100
```

**Security note**: Passwords in `ssh.yaml` are passed to `sshpass` at process launch — they never appear in the TUI output. For production use, prefer `identity_file` key-based auth.

---

## Usage

### TUI Dashboard

```bash
lazypx
```

The interface is split into three areas:

```
┌─────────────────┬──────────────────────────────────┐
│  Left sidebar   │  Right pane                      │
│  ─────────────  │  (detail view or embedded shell)  │
│  Nodes          │                                  │
│  VMs            │                                  │
│  Containers     │                                  │
│  Storage        │                                  │
├─────────────────┴──────────────────────────────────┤
│  Bottom: Tasks & Events                            │
└────────────────────────────────────────────────────┘
```

#### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Move up/down in the focused list |
| `tab` / `shift+tab` | Cycle focus between sidebar panels |
| `1` / `2` / `3` / `4` | Jump to Nodes / VMs / Containers / Storage |
| `enter` | Expand / inspect selected item |
| `/` | Fuzzy search |

#### Actions

| Key | Action |
|-----|--------|
| `s` | Start VM or container |
| `x` | Stop VM or container |
| `r` | Reboot |
| `d` | Delete (confirmation required) |
| `m` | Migrate |
| `b` | Backup |
| `f` | Force refresh cluster data |

#### Embedded Shell

| Key | Action |
|-----|--------|
| `e` | Open embedded shell for selected VM/CT (or focus if already open) |
| `ctrl+q` | Unfocus shell — return to tree navigation (session stays visible) |
| `ctrl+w` | Close shell view — hide from right pane (session keeps running in background) |
| `ctrl+u` | Scroll shell history up (only when tree is focused, not shell) |
| `ctrl+d` | Scroll shell history down (only when tree is focused, not shell) |
| `t` | Open sessions picker overlay — list all active sessions |

When the shell is focused, **all keypresses are forwarded to the PTY**. Standard terminal sequences work: arrow keys, ctrl+c, ctrl+z, function keys, alt+key, etc.

#### General

| Key | Action |
|-----|--------|
| `?` | Toggle keybindings help overlay |
| `q` / `ctrl+c` | Quit |

### Sessions Picker (`t`)

The sessions overlay lists all active shell sessions across VMs/CTs. Press `enter` to jump to a session — if it's currently embedded, the right pane switches to it; otherwise it opens full-screen via `tea.Exec`.

---

### CLI Commands

Use lazypx headlessly for scripting:

**SSH:**
```bash
lazypx ssh 105          # Connect by VMID
lazypx ssh mgmt         # Connect by VM name (resolves ID via Proxmox API)
```

**VM/CT power state:**
```bash
lazypx vm list
lazypx vm start <vmid>
lazypx vm stop <vmid>
lazypx vm reboot <vmid>
```

**Snapshots:**
```bash
lazypx snapshot list <vmid>
lazypx snapshot create <vmid> <name>
lazypx snapshot rollback <vmid> <name>
lazypx snapshot delete <vmid> <name>
```

---

## Security

- **No credentials in code**: `config.yaml` and `ssh.yaml` are in `~/.config/lazypx/` and excluded from git via `.gitignore`.
- **OS Keychain support**: API token secrets can be stored in macOS Keychain or Linux Secret Service instead of plain text in `config.yaml`.
- **SSH passwords**: Passed to `sshpass` at process start, never stored in memory beyond that. Prefer key-based auth.
- **TLS**: Set `insecure: false` in `config.yaml` and provide a valid cert on your Proxmox host to enable certificate verification.

---

## Architecture

```
lazypx/
├── main.go              # Entry point — routes TUI vs CLI
├── commands/            # CLI subcommands (vm, ssh, snapshot)
├── api/                 # Proxmox API client (REST + token auth)
├── audit/               # Local audit log for TUI actions
├── cache/               # TTL-based cluster snapshot cache
├── config/              # Config + SSH yaml loaders, keyring integration
├── sessions/            # PTY session manager (creack/pty)
├── state/               # Shared AppState across all TUI models
└── tui/
    ├── app.go           # Root Bubble Tea model, key routing, layout
    ├── layout.go        # Pane dimension calculation
    ├── detail.go        # Right pane detail view (VM/CT/Node/Storage)
    ├── shell_pane.go    # Embedded PTY terminal pane
    ├── terminal.go      # VT100/VT220 terminal emulator
    ├── tasks.go         # Bottom tasks & events pane
    ├── help.go          # Help, confirm, search overlays
    ├── sessions_overlay.go  # Sessions picker overlay
    └── styles.go        # Lip Gloss style definitions
```

---

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Commit your changes: `git commit -m 'Add my feature'`
4. Push: `git push origin feature/my-feature`
5. Open a Pull Request

---

## License

MIT — see [LICENSE](LICENSE) for details.
