# lazypx

A terminal UI for managing [Proxmox VE](https://www.proxmox.com/) clusters. Think [lazygit](https://github.com/jesseduffield/lazygit) but for Proxmox.

Written in Go. No dependencies other than the binary itself. Works on macOS and Linux.

## What it does

- Shows your nodes, VMs, containers, and storage in a split-pane TUI
- Lets you SSH into any VM/CT directly from the terminal — the shell stays alive as you navigate around
- Start, stop, reboot, delete, migrate, and snapshot VMs with keyboard shortcuts
- Shows Proxmox task progress in real time
- Also works as a CLI for scripting: `lazypx vm start 105`, `lazypx ssh myvm`

## What it doesn't do

- No web UI — this is terminal only
- No Windows support
- Password auth for SSH requires [sshpass](https://linux.die.net/man/1/sshpass) to be installed separately
- The embedded shell needs a real terminal (won't work in pipes or CI)

## Requirements

- A Proxmox VE host with API access enabled
- An API token (Datacenter → Permissions → API Tokens in the Proxmox web UI)
- [Go](https://go.dev/dl/) 1.25+ to build from source (or download a prebuilt binary)
- [sshpass](https://linux.die.net/man/1/sshpass) only if you use password-based SSH (key auth doesn't need it)

## Install

### Download a prebuilt binary

Grab the right file for your platform from [Releases](https://github.com/vintila-marius-cristian/lazypx/releases):

| Platform | File |
|----------|------|
| macOS Apple Silicon (M1/M2/M3) | `lazypx-darwin-arm64` |
| macOS Intel | `lazypx-darwin-amd64` |
| Linux x86_64 | `lazypx-linux-amd64` |
| Linux ARM64 | `lazypx-linux-arm64` |

Then:

```bash
chmod +x lazypx-darwin-arm64
sudo mv lazypx-darwin-arm64 /usr/local/bin/lazypx
```

### Build from source

```bash
git clone https://github.com/vintila-marius-cristian/lazypx.git
cd lazypx
go build -o lazypx .
sudo mv lazypx /usr/local/bin/
```

Verify:

```bash
lazypx version
# lazypx v0.1.0
```

## First-time setup

### 1. Create the config file

lazypx looks for config at `~/.config/lazypx/config.yaml`. If the file doesn't exist, running `lazypx` will show you an example.

Create it:

```bash
mkdir -p ~/.config/lazypx
```

Create `~/.config/lazypx/config.yaml`:

```yaml
default_profile: default

profiles:
  - name: default
    host: https://192.168.1.10:8006
    token_id: root@pam!mytoken
    token_secret: "your-secret-here"
    tls_insecure: true        # set to false if your Proxmox has a valid TLS cert
    refresh_interval: 30      # how often to poll the cluster (seconds)
    production: false         # shows a red header when true
```

Fields:
- `host` — your Proxmox API URL (usually `https://<ip>:8006`)
- `token_id` — the token ID you created in Proxmox (format: `user@realm!tokenname`)
- `token_secret` — the secret Proxmox gave you when creating the token
- `tls_insecure` — `true` to skip certificate verification (common with self-signed certs)
- `refresh_interval` — how often to reload cluster data in the TUI (default: 30s)

#### Using the OS keychain instead of plain text

Instead of putting `token_secret` in the YAML file, you can store it in your OS keychain:

```bash
# macOS
security add-generic-password -s lazypx -a "root@pam!mytoken" -w "your-secret"

# Linux (needs secret-tool from libsecret-utils)
secret-tool store --label="lazypx" service lazypx username "root@pam!mytoken"
```

Then leave `token_secret: ""` in the config file. lazypx will look it up automatically.

### 2. (Optional) Set up SSH access

The embedded shell (`e` key) and `lazypx ssh` command need to know how to reach your VMs. Create `~/.config/lazypx/ssh.yaml`:

```yaml
# Key-based auth (recommended)
- id: 101
  host: 192.168.1.50
  user: admin
  identity_file: ~/.ssh/id_ed25519

# Password auth (needs sshpass installed)
- id: 105
  host: 10.0.20.198
  user: packer
  password: packer
  port: 22

# Minimal — uses your local username, port 22
- id: 200
  host: 192.168.1.100
```

`id` is the Proxmox VMID. Without this file, the `e` key and `lazypx ssh` won't work — you'll get a "no SSH mapping" error. All other TUI features (viewing VMs, start/stop, snapshots, etc.) work fine without it.

### 3. Run it

```bash
lazypx
```

## Usage

### TUI

Run without arguments to open the interactive dashboard:

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

#### Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `1` | Jump to Nodes panel |
| `2` | Jump to VMs panel |
| `3` | Jump to Containers panel |
| `4` | Jump to Storage panel |
| `tab` | Next panel |
| `shift+tab` | Previous panel |
| `enter` | Inspect selected item (show details) |
| `/` | Search / filter |
| `f` | Force refresh cluster data |
| `?` | Show/hide keybindings help |
| `q` / `ctrl+c` | Quit |

#### VM/Container actions

Select a VM or container, then:

| Key | Action |
|-----|--------|
| `s` | Start |
| `x` | Stop (graceful shutdown) |
| `r` | Reboot |
| `d` | Delete (asks for confirmation) |
| `m` | Migrate to another node |
| `b` | Backup |
| `c` | Show snapshots |
| `e` | Open embedded SSH shell |
| `t` | Show all active shell sessions |

#### Embedded shell

When you press `e` on a VM/CT, a full PTY shell opens in the right pane. This is a real terminal — vim, htop, tmux inside the guest all work.

| Key | Action |
|-----|--------|
| `e` | Open or focus shell for selected VM/CT |
| `ctrl+q` | Unfocus shell — go back to the left tree (shell keeps running) |
| `ctrl+w` | Hide shell view (shell keeps running in background) |
| `ctrl+u` | Scroll shell history up (when tree is focused) |
| `ctrl+d` | Scroll shell history down (when tree is focused) |
| `t` | Open sessions picker to switch between shells |

When the shell is focused, all keyboard input goes to the remote machine. Arrow keys, ctrl+c, ctrl+z, alt+key, function keys — they all work.

Shells are persistent: if you press `ctrl+w` to hide a shell and navigate away, the session keeps running. Press `e` again on the same VM to reattach. You can have shells open on multiple VMs simultaneously.

#### Confirmations

For destructive actions (delete, reboot), a confirmation prompt appears:

| Key | Action |
|-----|--------|
| `y` / `enter` | Confirm |
| `n` / `esc` / `q` | Cancel |

#### Search

Press `/` to filter the current list. Type to filter, `enter` to apply, `esc` to cancel, `backspace` to delete.

### CLI

Use lazypx headlessly for scripts and automation:

```bash
# SSH into a VM by ID or name
lazypx ssh 105
lazypx ssh myvm

# VM lifecycle
lazypx vm list
lazypx vm start 105
lazypx vm stop 105
lazypx vm reboot 105

# Snapshots
lazypx snapshot list 105
lazypx snapshot create 105 "before-update"
lazypx snapshot rollback 105 "before-update"
lazypx snapshot delete 105 "before-update"

# Cluster info
lazypx cluster status
lazypx cluster resources
lazypx cluster resources -t vm    # filter by type

# Nodes
lazypx node list
lazypx node status pve1

# Access management
lazypx access user list
lazypx access user create newuser@pve --email user@example.com
lazypx access user delete newuser@pve
lazypx access group list
lazypx access role list
lazypx access acl list
```

All commands support `-p` to select a profile: `lazypx -p prod vm list`.

## Configuration reference

### Profiles

The config file supports multiple profiles. Use `default_profile` to pick which one loads automatically, or pass `-p <name>` on the command line.

```yaml
default_profile: default

profiles:
  - name: default
    host: https://192.168.1.10:8006
    token_id: root@pam!mytoken
    token_secret: "secret"
    tls_insecure: true
    refresh_interval: 30
    production: false

  - name: prod
    host: https://pve-prod.example.com:8006
    token_id: root@pam!lazypx
    token_secret: ""
    tls_insecure: false
    refresh_interval: 60
    production: true
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | yes | — | Profile name (used with `-p`) |
| `host` | yes | — | Proxmox API URL |
| `token_id` | yes | — | API token ID (`user@realm!tokenname`) |
| `token_secret` | yes* | — | API token secret (or use OS keychain) |
| `tls_insecure` | no | `false` | Skip TLS certificate verification |
| `refresh_interval` | no | `30` | Cluster data refresh interval (seconds) |
| `production` | no | `false` | Shows red header bar when `true` |

*`token_secret` can be empty if you store it in the OS keychain.

### SSH config

`~/.config/lazypx/ssh.yaml` maps Proxmox VMIDs to SSH endpoints:

```yaml
- id: 101                    # Proxmox VMID (required)
  host: 192.168.1.50         # SSH host (required)
  user: admin                # SSH user (optional, defaults to local username)
  port: 22                   # SSH port (optional, defaults to 22)
  identity_file: ~/.ssh/id   # Key file (optional)
  password: secret           # Password (optional, needs sshpass)
```

## Security

- Config files live in `~/.config/lazypx/` — they're never in the git repo.
- Token secrets can be stored in macOS Keychain or Linux Secret Service instead of plain text.
- SSH passwords are passed to `sshpass` at process start and never shown in the TUI.
- Use key-based SSH auth (`identity_file`) instead of passwords when possible.
- Set `tls_insecure: false` and use a valid certificate on your Proxmox host for production.

## Project structure

```
lazypx/
├── main.go                  Entry point
├── api/                     Proxmox REST API client
├── audit/                   Local audit log
├── cache/                   Cluster data cache with TTL
├── config/                  Config and SSH YAML loading, keyring
├── commands/                CLI subcommands (vm, ssh, snapshot, etc.)
├── sessions/                PTY session manager
├── state/                   Shared application state
└── tui/                     Bubble Tea UI
    ├── app.go               Main model, key routing
    ├── sidebar.go           Left panel (nodes, VMs, containers, storage)
    ├── detail.go            Right panel (VM/CT/node/storage details)
    ├── shell_pane.go        Embedded terminal pane
    ├── terminal.go          VT100 terminal emulator
    ├── tasks.go             Bottom panel (tasks and events)
    ├── snapshots.go         Snapshot list overlay
    ├── backups.go           Backup list overlay
    ├── sessions_overlay.go  Session picker overlay
    ├── help.go              Help, confirm, search overlays
    ├── layout.go            Pane sizing
    └── styles.go            Colors and styles
```

## Contributing

1. Fork and clone
2. Make your changes
3. Run `go test -race ./...` and `go vet ./...`
4. Open a PR

## License

MIT
