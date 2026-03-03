# lazypx 🚀

[![Go Report Card](https://goreportcard.com/badge/github.com/your-username/lazypx)](https://goreportcard.com/report/github.com/your-username/lazypx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**lazypx** is a blazing-fast, `lazygit`-inspired Terminal UI (TUI) and Command-Line Interface (CLI) for managing **Proxmox VE** clusters. Built in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea) and Lip Gloss.

Tired of context-switching to a heavy web browser just to reboot a VM or check memory usage? `lazypx` brings your entire Proxmox datacenter into your terminal, complete with k9s-style seamless SSH sessions directly into your VMs!

---

## ✨ Features

- **Blazing Fast TUI**: Zero-latency navigation of Nodes, Virtual Machines, Containers, and Storage via stacked accordion panes.
- **Relational Filtering**: Select a Node, and the VM/CT lists automatically filter to show exactly what's running on that node.
- **Seamless SSH Shell**: Hit `e` on any VM to seamlessly suspend the UI, drop into a native SSH session, and restore the UI perfectly upon logging out.
- **Action Management**: Start (`s`), Stop (`x`), Reboot (`r`), and Delete (`d`) resources instantly with safe, confirmation overlays.
- **Real-Time Task Monitoring**: Proxmox async tasks (like starting a VM or Backups) are tracked dynamically at the bottom of the screen. Watch logs stream live into the TUI superimposed over global cluster events!
- **Deep Networking Insights**: Node interfaces, bridges, bonds, and live QEMU Guest Agent IPs are fetched asynchronously and displayed contextually in the details pane without lagging the UI.
- **Dual-Mode**: Don't want the UI? Use it via pure CLI: `lazypx vm start mgmt` or `lazypx ssh mgmt`.

---

## 📸 Screenshots

*(Add a screenshot of your TUI here: `![lazypx TUI](./docs/assets/demo.gif)`)*

---

## 📦 Installation

To use `lazypx` from anywhere on your machine, you must download the executable and place it in your system's `PATH`.

### macOS

**Using Homebrew (Coming Soon):**
```bash
brew install your-username/tap/lazypx
```

**Manual Installation:**
1. Download the latest Darwin architecture binary from the [Releases](https://github.com/your-username/lazypx/releases).
2. Make it executable and move it to your PATH:
```bash
chmod +x lazypx-darwin-amd64
sudo mv lazypx-darwin-amd64 /usr/local/bin/lazypx
```

### Ubuntu / Linux

**Manual Installation:**
1. Download the latest Linux binary from the [Releases](https://github.com/your-username/lazypx/releases).
2. Make it executable and move it to your PATH:
```bash
chmod +x lazypx-linux-amd64
sudo mv lazypx-linux-amd64 /usr/local/bin/lazypx
```

### From Source (All Platforms)

If you have Go 1.22+ installed:
```bash
git clone https://github.com/your-username/lazypx.git
cd lazypx
go build -o lazypx .
sudo mv lazypx /usr/local/bin/
```

---

## ⚙️ Configuration

`lazypx` requires an API Token from your Proxmox server.
Create a new API token in Proxmox at **Datacenter -> Permissions -> API Tokens**.

Generate a starter configuration file by running:
```bash
lazypx init-config
```

Save the output to `~/.config/lazypx/config.yaml` and fill in your details:

```yaml
profiles:
  default:
    host: "https://10.0.0.10:8006"
    token_id: "root@pam!mytoken"
    token_secret: "12345678-1234-1234-1234-123456789abc"
    insecure: true   # Set to false if you have a valid SSL cert
    refresh_s: 30    # Cache refresh interval
active_profile: default
```

### SSH Shell Configuration (`~/.config/lazypx/ssh.yaml`)

To enable the seamless SSH feature (`lazypx ssh <name>` or pressing `e` in the TUI), you need to map your VMIDs to their connection details.

Create `~/.config/lazypx/ssh.yaml`:

```yaml
# Map VMID 105 to packer@10.0.20.198
- id: 105
  host: 10.0.20.198   # Required
  user: packer        # Optional (defaults to current local user if omitted)
  password: packer    # Optional (If using password auth, requires 'sshpass' installed locally)
  port: 22            # Optional

# Key-based Auth Example
- id: 101
  host: 192.168.1.50
  user: admin
  identity_file: ~/.ssh/id_ed25519
```

*Note: If a password is provided in this file, `lazypx` will attempt to securely pipe it using `sshpass`. Make sure `sshpass` is installed via `apt install sshpass` or `brew install sshpass`.*

---

## 🕹️ Usage

### Interactive TUI Dashboard

Simply type `lazypx` and press enter!

**TUI Hotkeys:**
- `tab / shift+tab` : Cycle focus between Nodes, VMs, CTs, and Storage panels.
- `1`, `2`, `3`, `4` : Instantly jump to a specific panel.
- `j / k` or `↓ / ↑` : Navigate up and down the lists.
- `s` : Start a VM / CT
- `x` : Stop a VM / CT
- `r` : Reboot
- `d` : Delete (with destructive confirmation)
- `e` : Seamless SSH into the selected machine!
- `f` : Force dynamic cache refresh
- `/` : Fuzzy search any resource

### Direct CLI Commands

`lazypx` can also be fully utilized headlessly via subcommands to integrate with scripts or for quick one-off actions.

**SSH:**
```bash
lazypx ssh 105          # Connect by VMID
lazypx ssh mgmt         # Connect by exact VM Name (dynamically resolves ID via API!)
```

**Power State:**
```bash
lazypx vm list
lazypx vm start <vmid>
lazypx vm stop <vmid>
lazypx vm reboot <vmid>
```

**Snapshots:**
```bash
lazypx snapshot list <vmid>
lazypx snapshot create <vmid> <snapshot-name>
lazypx snapshot rollback <vmid> <snapshot-name>
lazypx snapshot delete <vmid> <snapshot-name>
```

---

## 🤝 Contributing
Contributions are extremely welcome! 
1. Fork it
2. Create your feature branch (`git checkout -b feature/fooBar`)
3. Commit your changes (`git commit -am 'Add some fooBar'`)
4. Push to the branch (`git push origin feature/fooBar`)
5. Create a new Pull Request

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
