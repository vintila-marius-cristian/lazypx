package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"lazypx/api"
	"lazypx/state"
)

// DetailModel renders the main pane with resource details.
type DetailModel struct {
	st     *state.AppState
	width  int
	height int
}

// NewDetailModel creates a new detail model.
func NewDetailModel(st *state.AppState) DetailModel {
	return DetailModel{st: st}
}

// SetSize sets the INNER display dimensions (frame already subtracted by layout engine).
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Sync updates the model from state.
func (m *DetailModel) Sync(st *state.AppState) {
	m.st = st
}

// View renders the detail pane. width/height are INNER dimensions;
// the method wraps content in a border style to produce the final outer pane.
func (m DetailModel) View(focused bool) string {
	if m.width < 2 || m.height < 2 {
		return ""
	}
	style := StylePaneBorder
	if focused {
		style = StylePaneBorderFocused
	}
	// width/height are already inner dimensions from the layout engine
	content := m.renderContent(m.width)
	content = clipToHeight(content, m.height)
	return style.Width(m.width).Height(m.height).Render(content)
}

func (m DetailModel) renderContent(w int) string {
	sel := m.st.Selected
	switch sel.Kind {
	case state.KindVM:
		if sel.VMStatus != nil {
			return m.renderVM(sel, w)
		}
	case state.KindContainer:
		if sel.CTStatus != nil {
			return m.renderCT(sel, w)
		}
	case state.KindNode:
		if sel.NodeStatus != nil {
			return m.renderNode(sel, w)
		}
	case state.KindStorage:
		return m.renderStorage(sel, w)
	}
	if m.st.Snapshot.IsEmpty() {
		return "\n  " + StyleSubtext.Render("No data yet — connecting…")
	}
	return "\n  " + StyleSubtext.Render("↑↓ navigate  •  enter expand  •  ? help")
}

// ── VM ───────────────────────────────────────────────────────────────────────

func (m DetailModel) renderVM(sel state.Selection, w int) string {
	vm := sel.VMStatus
	var sb strings.Builder
	gw := GaugeWidth(w)

	// Header
	sb.WriteString("\n")
	lock := ""
	if vm.Lock != "" {
		lock = "  " + StyleError2.Render("🔒 "+vm.Lock)
	}
	tmpl := ""
	if vm.Template == 1 {
		tmpl = "  " + StyleSubtext.Render("[template]")
	}
	sb.WriteString("  " + StatusDot(vm.Status) + "  " +
		StyleTitle.Render(fmt.Sprintf("VM %d — %s", vm.VMID, vm.Name)) +
		lock + tmpl + "\n\n")

	// Identity
	sb.WriteString(row("Status", StatusStyle(vm.Status).Render(vm.Status)))
	sb.WriteString(row("Node", vm.Node))
	if vm.Uptime > 0 {
		sb.WriteString(row("Uptime", formatUptime(vm.Uptime)))
	}
	if vm.Tags != "" {
		sb.WriteString(row("Tags", StyleMagenta.Render(vm.Tags)))
	}
	if vm.HA != nil && vm.HA.Managed == 1 {
		haState := vm.HA.State
		if haState == "" {
			haState = "managed"
		}
		sb.WriteString(row("HA", StyleRunning.Render("● "+haState)))
	}
	sb.WriteString("\n")

	// CPU
	cpuPct := vm.CPU
	cpuBar := GaugeBar(gw, cpuPct)
	sb.WriteString(row("CPU", fmt.Sprintf("%s  %.1f%%  (%d cores)", cpuBar, cpuPct*100, vm.MaxCPU)))

	// RAM
	memPct := safeDiv(vm.MemUsed, vm.MemTotal)
	sb.WriteString(row("RAM", fmt.Sprintf("%s  %s / %s  (%.0f%%)",
		GaugeBar(gw, memPct),
		formatBytes(vm.MemUsed), formatBytes(vm.MemTotal), memPct*100)))

	// Root disk size (always available from maxdisk)
	if vm.MaxDisk > 0 {
		diskUsedPct := safeDiv(vm.DiskUsed, vm.MaxDisk)
		if vm.DiskUsed > 0 {
			sb.WriteString(row("Root Disk", fmt.Sprintf("%s  %s / %s",
				GaugeBar(gw, diskUsedPct),
				formatBytes(vm.DiskUsed), formatBytes(vm.MaxDisk))))
		} else {
			sb.WriteString(row("Root Disk", StyleSubtext.Render(formatBytes(vm.MaxDisk)+" allocated")))
		}
	}

	// I/O (only when non-zero — running VMs)
	if vm.DiskRead+vm.DiskWrite > 0 {
		sb.WriteString(row("Disk I/O", fmt.Sprintf("↓ %s  ↑ %s",
			formatBytes(vm.DiskRead), formatBytes(vm.DiskWrite))))
	}
	if vm.NetIn+vm.NetOut > 0 {
		sb.WriteString(row("Network", fmt.Sprintf("↓ %s  ↑ %s",
			formatBytes(vm.NetIn), formatBytes(vm.NetOut))))
	}

	cfg := sel.VMConfig
	if cfg != nil {
		sb.WriteString("\n")
		sb.WriteString("  " + StyleLabel.Render("── Disks") + "\n")
		disks := extractDisks(cfg)
		if len(disks) == 0 {
			sb.WriteString("  " + StyleSubtext.Render("no disks configured") + "\n")
		}
		for _, d := range disks {
			sb.WriteString("  " + StyleSubtext.Render(d.Key+": ") + StyleValue.Render(d.Val) + "\n")
		}

		sb.WriteString("\n")
		sb.WriteString("  " + StyleLabel.Render("── Network") + "\n")
		nets := extractNets(cfg)
		if len(nets) == 0 {
			sb.WriteString("  " + StyleSubtext.Render("no network configured") + "\n")
		}
		for _, n := range nets {
			sb.WriteString("  " + StyleSubtext.Render(n.Key+": ") + StyleValue.Render(n.Val) + "\n")
		}
	}

	// Guest IPs
	if len(sel.GuestIPs) > 0 {
		sb.WriteString("\n  " + StyleLabel.Render("── Guest IPs (QEMU Agent)") + "\n")
		for _, net := range sel.GuestIPs {
			if net.Name == "lo" {
				continue
			}
			var ips []string
			for _, ip := range net.IPAddresses {
				ips = append(ips, fmt.Sprintf("%s/%d", ip.IPAddress, ip.Prefix))
			}
			if len(ips) > 0 {
				sb.WriteString(fmt.Sprintf("  %-6s %s\n", StyleValue.Render(net.Name), StyleSubtext.Render(strings.Join(ips, ", "))))
			}
		}
	} else if vm.Status == "running" {
		sb.WriteString("\n  " + StyleSubtext.Render("IP unknown (guest agent disabled/unavailable?)") + "\n")
	}

	return sb.String()
}

// ── Container ─────────────────────────────────────────────────────────────────

func (m DetailModel) renderCT(sel state.Selection, w int) string {
	ct := sel.CTStatus
	var sb strings.Builder
	gw := GaugeWidth(w)

	sb.WriteString("\n")
	sb.WriteString("  " + StatusDot(ct.Status) + "  " +
		StyleTitle.Render(fmt.Sprintf("CT %d — %s", ct.VMID, ct.Name)) + "\n\n")

	sb.WriteString(row("Status", StatusStyle(ct.Status).Render(ct.Status)))
	sb.WriteString(row("Node", ct.Node))
	if ct.Uptime > 0 {
		sb.WriteString(row("Uptime", formatUptime(ct.Uptime)))
	}
	if ct.Tags != "" {
		sb.WriteString(row("Tags", StyleMagenta.Render(ct.Tags)))
	}
	sb.WriteString("\n")

	cpuPct := ct.CPU
	sb.WriteString(row("CPU", fmt.Sprintf("%s  %.1f%%  (%d cores)", GaugeBar(gw, cpuPct), cpuPct*100, ct.MaxCPU)))

	memPct := safeDiv(ct.MemUsed, ct.MemTotal)
	sb.WriteString(row("RAM", fmt.Sprintf("%s  %s / %s  (%.0f%%)",
		GaugeBar(gw, memPct),
		formatBytes(ct.MemUsed), formatBytes(ct.MemTotal), memPct*100)))

	diskPct := safeDiv(ct.DiskUsed, ct.DiskMax)
	if ct.DiskMax > 0 {
		sb.WriteString(row("Disk", fmt.Sprintf("%s  %s / %s  (%.0f%%)",
			GaugeBar(gw, diskPct),
			formatBytes(ct.DiskUsed), formatBytes(ct.DiskMax), diskPct*100)))
	}

	if ct.NetIn+ct.NetOut > 0 {
		sb.WriteString(row("Network", fmt.Sprintf("↓ %s  ↑ %s",
			formatBytes(ct.NetIn), formatBytes(ct.NetOut))))
	}

	return sb.String()
}

// ── Node ─────────────────────────────────────────────────────────────────────

func (m DetailModel) renderNode(sel state.Selection, w int) string {
	n := sel.NodeStatus
	var sb strings.Builder
	gw := GaugeWidth(w)

	sb.WriteString("\n")
	sb.WriteString("  " + StatusDot(n.Status) + "  " +
		StyleTitle.Render("Node: "+n.Node) + "\n\n")

	sb.WriteString(row("Status", StyleRunning.Render(n.Status)))
	if n.Uptime > 0 {
		sb.WriteString(row("Uptime", formatUptime(n.Uptime)))
	}

	// Extended info from /nodes/{node}/status
	if ext := n.Extended; ext != nil {
		if ext.PVEVersion != "" {
			sb.WriteString(row("PVE", StyleSubtext.Render(ext.PVEVersion)))
		}
		if ext.KernelVer != "" {
			sb.WriteString(row("Kernel", StyleSubtext.Render(ext.KernelVer)))
		}
		if ext.CPUModel != "" {
			// Truncate very long CPU model names
			model := ext.CPUModel
			if len(model) > w-16 && w > 20 {
				model = model[:w-19] + "…"
			}
			sb.WriteString(row("CPU Model", model))
		}
		sb.WriteString(row("CPU Cores", fmt.Sprintf("%d cores / %d sockets @ %s MHz",
			ext.CPUCores, ext.CPUSockets, ext.CPUMHz)))
	} else {
		sb.WriteString(row("CPU Cores", fmt.Sprintf("%d", n.MaxCPU)))
	}
	sb.WriteString("\n")

	cpuPct := n.CPUUsage
	sb.WriteString(row("CPU", fmt.Sprintf("%s  %.2f%%", GaugeBar(gw, cpuPct), cpuPct*100)))

	memPct := safeDiv(n.MemUsed, n.MemTotal)
	sb.WriteString(row("RAM", fmt.Sprintf("%s  %s / %s  (%.0f%%)",
		GaugeBar(gw, memPct),
		formatBytes(n.MemUsed), formatBytes(n.MemTotal), memPct*100)))

	if n.Extended != nil && n.Extended.SwapTotal > 0 {
		swapPct := safeDiv(n.Extended.SwapUsed, n.Extended.SwapTotal)
		sb.WriteString(row("Swap", fmt.Sprintf("%s  %s / %s",
			GaugeBar(gw, swapPct),
			formatBytes(n.Extended.SwapUsed), formatBytes(n.Extended.SwapTotal))))
	}

	if n.DiskTotal > 0 {
		diskPct := safeDiv(n.DiskUsed, n.DiskTotal)
		sb.WriteString(row("OS Disk", fmt.Sprintf("%s  %s / %s  (%.0f%%)",
			GaugeBar(gw, diskPct),
			formatBytes(n.DiskUsed), formatBytes(n.DiskTotal), diskPct*100)))
	}

	// Show storage pools for this node
	snap := m.st.Snapshot
	storage := snap.Storage[n.Node]
	if len(storage) > 0 {
		sb.WriteString("\n")
		sb.WriteString("  " + StyleLabel.Render("── Storage Pools") + "\n")
		for _, s := range storage {
			active := StyleRunning.Render("●")
			if s.Active == 0 {
				active = StyleSubtext.Render("○")
			}
			usedPct := s.UsedFraction
			bar := GaugeBar(gw, usedPct)
			line := fmt.Sprintf("%s %-16s  %s  %4.1f%%  %s/%s  %s",
				active, s.Storage, bar, usedPct*100,
				formatBytes(s.Used), formatBytes(s.Total),
				StyleSubtext.Render(s.Type))
			sb.WriteString("  " + line + "\n")
		}
	}

	// Show network interfaces
	nics := snap.Network[n.Node]
	if len(nics) > 0 {
		sortedNics := make([]api.NetworkInterface, len(nics))
		copy(sortedNics, nics)
		sort.Slice(sortedNics, func(i, j int) bool {
			return sortedNics[i].Iface < sortedNics[j].Iface
		})

		sb.WriteString("\n")
		sb.WriteString("  " + StyleLabel.Render("── Network Interfaces") + "\n")
		for _, nic := range sortedNics {
			active := StyleRunning.Render("●")
			if nic.Active == 0 {
				active = StyleSubtext.Render("○")
			}

			addr := nic.Address
			if addr == "" {
				addr = "-"
			} else if nic.Netmask != "" {
				addr += "/" + nic.Netmask
			}

			details := fmt.Sprintf("%-6s %-15s", strings.ToLower(nic.Type), addr)
			if nic.Gateway != "" {
				details += fmt.Sprintf(" gw:%s", nic.Gateway)
			}
			if nic.BridgePorts != "" {
				details += fmt.Sprintf(" ports:%s", nic.BridgePorts)
			}

			line := fmt.Sprintf("%s %-10s  %s", active, StyleValue.Render(nic.Iface), StyleSubtext.Render(details))
			sb.WriteString("  " + line + "\n")
		}
	}

	return sb.String()
}

// ── Storage ───────────────────────────────────────────────────────────────────

func (m DetailModel) renderStorage(sel state.Selection, w int) string {
	// Find the storage status from snapshot
	snap := m.st.Snapshot
	var s *api.StorageStatus
	for i := range snap.Storage[sel.NodeName] {
		if snap.Storage[sel.NodeName][i].Storage == sel.StorageName {
			s = &snap.Storage[sel.NodeName][i]
			break
		}
	}

	var sb strings.Builder
	sb.WriteString("\n")

	if s == nil {
		sb.WriteString("  " + StyleSubtext.Render("Storage details unavailable") + "\n")
		return sb.String()
	}

	gw := GaugeWidth(w)
	active := StyleRunning.Render("● active")
	if s.Active == 0 {
		active = StyleError2.Render("○ inactive")
	}

	sb.WriteString("  " + StyleTitle.Render("Storage: "+s.Storage) + "\n\n")
	sb.WriteString(row("Status", active))
	sb.WriteString(row("Type", s.Type))
	sb.WriteString(row("Node", sel.NodeName))
	shared := "no"
	if s.Shared == 1 {
		shared = "yes"
	}
	sb.WriteString(row("Shared", shared))
	if s.Content != "" {
		parts := strings.Split(s.Content, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		sort.Strings(parts)
		sortedContent := strings.Join(parts, ", ")
		sb.WriteString(row("Content", StyleSubtext.Render(sortedContent)))
	}
	sb.WriteString("\n")
	usedPct := s.UsedFraction
	sb.WriteString(row("Usage", fmt.Sprintf("%s  %s / %s  (%.1f%%)",
		GaugeBar(gw, usedPct),
		formatBytes(s.Used), formatBytes(s.Total), usedPct*100)))
	sb.WriteString(row("Free", formatBytes(s.Avail)))

	return sb.String()
}

// ── Disk/Net config parsing ───────────────────────────────────────────────────

type kv struct{ Key, Val string }

// extractDisks parses scsi0/virtio0/ide0/sata0 lines from VMConfig.
func extractDisks(cfg *api.VMConfig) []kv {
	var out []kv
	pairs := []struct {
		k string
		v string
	}{
		{"scsi0", cfg.Scsi0}, {"scsi1", cfg.Scsi1},
		{"virtio0", cfg.Virtio0},
	}
	for _, p := range pairs {
		if p.v != "" && !strings.Contains(p.v, "media=cdrom") {
			// Parse size from e.g. "SlowSmsgSSD:vm-221-disk-0,replicate=0,size=384G"
			size := ""
			for _, part := range strings.Split(p.v, ",") {
				if strings.HasPrefix(part, "size=") {
					size = strings.TrimPrefix(part, "size=")
				}
			}
			storage := strings.SplitN(p.v, ":", 2)[0]
			val := storage
			if size != "" {
				val = fmt.Sprintf("%s  (%s)", storage, size)
			}
			out = append(out, kv{p.k, val})
		}
	}
	return out
}

// extractNets parses net0/net1 lines from VMConfig.
func extractNets(cfg *api.VMConfig) []kv {
	var out []kv
	netFields := []struct {
		k string
		v string
	}{
		{"net0", cfg.Net0}, {"net1", cfg.Net1},
	}
	for _, p := range netFields {
		if p.v != "" {
			// e.g. "virtio=BC:24:11:3B:92:A1,bridge=vmbr0,tag=4"
			bridge := ""
			tag := ""
			mac := ""
			for _, part := range strings.Split(p.v, ",") {
				if strings.HasPrefix(part, "bridge=") {
					bridge = strings.TrimPrefix(part, "bridge=")
				} else if strings.HasPrefix(part, "tag=") {
					tag = "vlan " + strings.TrimPrefix(part, "tag=")
				} else if strings.Contains(part, "=") {
					// virtio=MAC
					kv2 := strings.SplitN(part, "=", 2)
					if len(kv2[1]) == 17 { // MAC length
						mac = kv2[1]
					}
				}
			}
			val := bridge
			if tag != "" {
				val += "  " + tag
			}
			if mac != "" {
				val += "  " + StyleSubtext.Render(mac)
			}
			out = append(out, kv{p.k, val})
		}
	}
	return out
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func row(label, value string) string {
	return "  " + StyleLabel.Render(fmt.Sprintf("%-10s", label)) + "  " + StyleValue.Render(value) + "\n"
}

func safeDiv(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func formatBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatUptime(secs int64) string {
	d := time.Duration(secs) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// StyleMagenta renders tags and special values in magenta/purple.
var StyleMagenta = lipgloss.NewStyle().Foreground(colorMagenta)
