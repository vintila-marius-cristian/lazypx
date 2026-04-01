# lazypx

A `lazygit`-style terminal UI for managing **Proxmox VE** clusters. Built in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Navigate your entire datacenter without leaving the terminal. Embedded PTY shells for each VM/CT stay alive as you move around — no tmux needed.

```
┌─────────────────┬──────────────────────────────────────┐
│  Nodes          │  Detail view or embedded shell       │
│  VMs            │  (vim, top, htop all work)           │
│  Containers     │                                      │
│  Storage        │                                      │
├─────────────────┴──────────────────────────────────────┤
│  Tasks & Events                                        │
└────────────────────────────────────────────────────────┘
```

## Features

- **Embedded SSH shell** — press `e` on any VM/CT to get a full PTY. Navigate away, it keeps running. Come back, it's still there.
- **Relational filtering** — select a node and the VM/CT lists auto-filter to that node.
- **Power actions** — start, stop, reboot, delete, migrate, backup with confirmation overlays.
- **Live task log** — Proxmox async tasks stream alongside local shell events.
- **Fuzzy search** — `/` to filter any resource instantly.
- **CLI mode** — `lazypx ssh 105`, `lazypx vm start 105`, `lazypx snapshot create 105 backup` for scripting.
- **OS keychain** — store your API token secret in macOS Keychain or Linux Secret Service instead of plain text.

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+ (to build)
- [sshpass](https://linux.die.net/man/1/sshpass) (only if using password-based SSH in `ssh.yaml`)
- A Proxmox VE API token (Datacenter → Permissions → API Tokens)

## Installation

### Prebuilt binaries

Download from [Releases](https://github.com/vintila-marius-cristian/lazypx/releases) for your platform:

| Platform | File |
|----------|------|
| macOS Apple Silicon | `lazypx-darwin-arm64` |
| macOS Intel | `lazypx-darwin-amd64` |
| Linux x86_64 | `lazypx-linux-amd64` |
| Linux ARM64 | `lazypx-linux-arm64` |

```bash
chmod +x lazypx-darwin-arm64
sudo mv lazypx-darwin-arm64 /usr/local/bin/lazypx
lazypx version
```

### Build from source

Requires [Go](https://go.dev/dl/) 1.25+:

```bash
git clone <repo-url> lazypx
cd lazypx
go build -o lazypx .
sudo mv lazypx /usr/local/bin/      # system-wide
# or
mv lazypx ~/bin/                     # user-local (make sure ~/bin is in PATH)
```

Verify it works:

```bash
lazypx version
```

## Configuration

All config lives in `~/.config/lazypx/`. Nothing is stored in the codebase.

### `~/.config/lazypx/config.yaml`

```yaml
profiles:
  default:
    host: "https://10.0.0.10:8006"
    token_id: "root@pam!mytoken"
    token_secret: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    insecure: true       # skip TLS verification for self-signed certs
    refresh_s: 30        # cache refresh interval in seconds
active_profile: default
```

lazypx will create this file with an example config on first run if it doesn't exist.

#### Storing the token secret in your OS keychain

Instead of keeping `token_secret` in plain text, omit it from the config and store it in your keychain:

```bash
# macOS
security add-generic-password -s lazypx -a "root@pam!mytoken" -w "your-secret"

# Linux (requires secret-tool from libsecret)
secret-tool store --label="lazypx" service lazypx username "root@pam!mytoken"
```

lazypx looks for the secret under service `lazypx` with the key matching your `token_id`.

### `~/.config/lazypx/ssh.yaml`

Maps VMIDs to SSH hosts. Required for the embedded shell (`e` key) and `lazypx ssh` CLI.

```yaml
# Password auth (requires sshpass)
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

# Minimal — uses current username, port 22
- id: 200
  host: 192.168.1.100
```

Passwords in `ssh.yaml` are passed to `sshpass` at process start and never appear in the TUI. For production, prefer key-based auth.

## Usage

### TUI

```bash
lazypx
```

#### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Move up/down |
| `tab` / `shift+tab` | Cycle focus between sidebar panels |
| `1` / `2` / `3` / `4` | Jump to Nodes / VMs / Containers / Storage |
| `enter` | Inspect selected item |
| `/` | Fuzzy search |

#### Actions

| Key | Action |
|-----|--------|
| `s` | Start |
| `x` | Stop |
| `r` | Reboot |
| `d` | Delete (with confirmation) |
| `m` | Migrate |
| `b` | Backup |
| `f` | Force refresh cluster data |

#### Embedded Shell

| Key | Action |
|-----|--------|
| `e` | Open shell for selected VM/CT |
| `ctrl+q` | Unfocus shell (session keeps running) |
| `ctrl+w` | Close shell view (session keeps running) |
| `t` | Sessions picker — list all active sessions |

When the shell is focused, all keypresses go to the PTY. Arrow keys, ctrl+c, ctrl+z, function keys all work.

#### General

| Key | Action |
|-----|--------|
| `?` | Toggle keybindings help |
| `q` / `ctrl+c` | Quit |

### CLI

```bash
lazypx ssh 105                    # SSH by VMID
lazypx ssh mgmt                   # SSH by VM name

lazypx vm list                    # List VMs
lazypx vm start <vmid>
lazypx vm stop <vmid>
lazypx vm reboot <vmid>

lazypx snapshot list <vmid>
lazypx snapshot create <vmid> <name>
lazypx snapshot rollback <vmid> <name>
lazypx snapshot delete <vmid> <name>
```

## Security

- No credentials in the codebase — config lives in `~/.config/lazypx/`.
- API token secrets can be stored in your OS keychain instead of plain text.
- SSH passwords are passed to `sshpass` at process start, never displayed.
- Set `insecure: false` in config to enable TLS certificate verification.

## Architecture

```
lazypx/
├── main.go              Entry point
├── commands/            CLI subcommands (vm, ssh, snapshot)
├── api/                 Proxmox API client (REST + token auth)
├── audit/               Local audit log for TUI actions
├── cache/               TTL-based cluster snapshot cache
├── config/              Config + SSH yaml loaders, keyring integration
├── sessions/            PTY session manager (creack/pty)
├── state/               Shared AppState across all TUI models
└── tui/
    ├── app.go           Root Bubble Tea model, key routing, layout
    ├── layout.go        Pane dimension calculation
    ├── detail.go        Right pane detail view (VM/CT/Node/Storage)
    ├── shell_pane.go    Embedded PTY terminal pane
    ├── terminal.go      VT100/VT220 terminal emulator
    ├── tasks.go         Bottom tasks & events pane
    ├── help.go          Help, confirm, search overlays
    ├── sessions_overlay.go  Sessions picker overlay
    └── styles.go        Lip Gloss style definitions
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `go test -race ./...`
5. Open a Pull Request

## License

MIT — see [LICENSE](LICENSE) for details.
