package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazypx/api"
	"lazypx/cache"
	"lazypx/config"
	"lazypx/state"
)

// ── Messages ─────────────────────────────────────────────────────────────────

// ClusterLoaded is sent when the initial cluster fetch completes.
type ClusterLoaded struct {
	Snapshot cache.ClusterSnapshot
}

// ClusterRefreshed is sent on background refresh tick.
type ClusterRefreshed struct {
	Snapshot cache.ClusterSnapshot
}

// RefreshTick triggers a background cache refresh.
type RefreshTick struct{}

// TaskLogLine carries a single log line from a watched task.
type TaskLogLine struct {
	Index int
	Line  string
}

// TaskDone signals that a watched task has finished.
type TaskDone struct {
	Index   int
	Success bool
}

// ActionError is sent when an API action fails.
type ActionError struct {
	Err error
}

// ── Root Model ────────────────────────────────────────────────────────────────

// Model is the root Bubble Tea model for lazypx.
type Model struct {
	state     *state.AppState
	apiClient *api.Client
	cache     *cache.Cache
	cfg       *config.Profile
	sshHosts  map[int]config.SSHHost

	// Sub-models
	sidebar SidebarModel
	detail  DetailModel
	tasks   TasksModel
	help    HelpModel
	confirm ConfirmModel
	search  SearchModel

	spinner spinner.Model
	layout  Layout // centralized layout dimensions
}

// New creates the root TUI model.
func New(apiClient *api.Client, clusterCache *cache.Cache, cfg *config.Profile) Model {
	profileName := cfg.Name
	if profileName == "" {
		profileName = "default"
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSpinner

	st := state.New(profileName, cfg.Production)

	ssh, _ := config.LoadSSH() // ignore error, map will be empty

	return Model{
		state:     st,
		apiClient: apiClient,
		cache:     clusterCache,
		cfg:       cfg,
		sshHosts:  ssh,
		spinner:   s,
		sidebar:   NewSidebarModel(st),
		detail:    NewDetailModel(st),
		tasks:     NewTasksModel(st),
		help:      NewHelpModel(),
		confirm:   NewConfirmModel(st),
		search:    NewSearchModel(st),
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadCluster(),
	)
}

func (m Model) loadCluster() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		snap := m.cache.Refresh(ctx)
		return ClusterLoaded{Snapshot: snap}
	}
}

func (m Model) refreshCluster() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		snap := m.cache.Refresh(ctx)
		return ClusterRefreshed{Snapshot: snap}
	}
}

func tickRefresh(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return RefreshTick{}
	})
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.layout = ComputeLayout(msg.Width, msg.Height, m.state.FocusedPanel)
		// Pass layout struct to sidebar, inner sizes to others
		m.sidebar.SetSize(m.layout)
		m.detail.SetSize(m.layout.DetailInnerW, m.layout.DetailInnerH)
		m.tasks.SetSize(m.layout.TasksInnerW, m.layout.TasksInnerH)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case ClusterLoaded:
		m.state.Loading = false
		if msg.Snapshot.Error != nil {
			m.state.Error = fmt.Sprintf("Failed to connect: %v", msg.Snapshot.Error)
		} else {
			m.state.Error = ""
			m.state.Snapshot = msg.Snapshot
			m.sidebar.Sync(m.state)
		}
		d := time.Duration(m.cfg.RefreshInterval) * time.Second
		cmds = append(cmds, tickRefresh(d))

	case ClusterRefreshed:
		if msg.Snapshot.Error == nil {
			m.state.Snapshot = msg.Snapshot
			m.sidebar.Sync(m.state)
		}
		d := time.Duration(m.cfg.RefreshInterval) * time.Second
		cmds = append(cmds, tickRefresh(d))

	case RefreshTick:
		cmds = append(cmds, m.refreshCluster())

	case ActionError:
		errMsg := fmt.Sprintf("Action failed: %v", msg.Err)
		m.state.Error = errMsg

		// Surface the error immediately via the confirm dialog
		m.state.ConfirmMsg = errMsg
		m.state.ConfirmAction = nil
		m.state.ConfirmVisible = true

	case TaskStartedMsg:
		idx := m.state.AddTask(msg.UPID, msg.Node, msg.Label)
		// Now that it's in state, we can watch it
		cmd := func() tea.Msg {
			ctx := context.Background() // could use timeout, but WatchTask is long running
			ch := make(chan api.TaskLog, 50)
			go m.apiClient.WatchTask(ctx, msg.Node, msg.UPID, ch)
			for {
				line, ok := <-ch
				if !ok {
					// Channel closed, task done
					break
				}
				// We don't receive log lines dynamically in the TUI yet, but if we did we'd dispatch them here
				_ = line
			}
			return TaskDone{Index: idx, Success: true} // Simplify: assume success if it finishes
		}
		cmds = append(cmds, cmd)

	case TaskLogLine:
		m.state.AppendTaskLog(msg.Index, msg.Line)

	case TaskDone:
		m.state.MarkTaskDone(msg.Index, msg.Success)
		m.state.Loading = true
		cmds = append(cmds, m.refreshCluster())

	case tea.KeyMsg:
		// Overlays take priority
		if m.state.ConfirmVisible {
			return m.handleConfirmKey(msg, cmds)
		}
		if m.state.HelpVisible {
			if msg.String() == "?" || msg.String() == "q" || msg.String() == "esc" {
				m.state.HelpVisible = false
			}
			return m, tea.Batch(cmds...)
		}
		if m.state.SearchActive {
			return m.handleSearchKey(msg, cmds)
		}
		return m.handleKey(msg, cmds)
	}

	// Always keep selection and detail in sync regardless of focused panel.
	m.sidebar.UpdateSelection()
	m.detail.Sync(m.state)
	if m.state.FocusedPanel == state.PanelTasks {
		m.tasks.Sync(m.state)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.state.HelpVisible = true

	case "tab":
		m.state.FocusedPanel = (m.state.FocusedPanel + 1) % 4
		// Whenever focus changes, trigger a layout recompute via window size msg shortcut
		m.layout = ComputeLayout(m.layout.TermW, m.layout.TermH, m.state.FocusedPanel)
		m.sidebar.SetSize(m.layout)

	case "shift+tab":
		if m.state.FocusedPanel == 0 {
			m.state.FocusedPanel = 3
		} else {
			m.state.FocusedPanel--
		}
		m.layout = ComputeLayout(m.layout.TermW, m.layout.TermH, m.state.FocusedPanel)
		m.sidebar.SetSize(m.layout)

	case "1":
		m.state.FocusedPanel = state.PanelNodes
		m.layout = ComputeLayout(m.layout.TermW, m.layout.TermH, m.state.FocusedPanel)
		m.sidebar.SetSize(m.layout)
	case "2":
		m.state.FocusedPanel = state.PanelVMs
		m.layout = ComputeLayout(m.layout.TermW, m.layout.TermH, m.state.FocusedPanel)
		m.sidebar.SetSize(m.layout)
	case "3":
		m.state.FocusedPanel = state.PanelCTs
		m.layout = ComputeLayout(m.layout.TermW, m.layout.TermH, m.state.FocusedPanel)
		m.sidebar.SetSize(m.layout)
	case "4":
		m.state.FocusedPanel = state.PanelStorage
		m.layout = ComputeLayout(m.layout.TermW, m.layout.TermH, m.state.FocusedPanel)
		m.sidebar.SetSize(m.layout)

	case "j", "down":
		m.sidebar.MoveDown()
		m.detail.Sync(m.state)

	case "k", "up":
		m.sidebar.MoveUp()
		m.detail.Sync(m.state)

	case "enter":
		// Currently accordions are not foldable like the tree was, enter might jump focus to detail
		// For now we'll just ignore enter or have it perform a specific action later.

	case "/":
		m.state.SearchActive = true
		m.state.SearchQuery = ""
		m.search = NewSearchModel(m.state)

	case "f":
		m.state.Loading = true
		cmds = append(cmds, m.refreshCluster())

	case "s":
		cmd := m.actionStart()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case "x":
		m.state.ConfirmMsg = fmt.Sprintf("Stop %s?", m.selectionLabel())
		m.state.ConfirmAction = func() interface{} {
			return m.actionStop()
		}
		m.state.ConfirmVisible = true

	case "r":
		m.state.ConfirmMsg = fmt.Sprintf("Reboot %s?", m.selectionLabel())
		m.state.ConfirmAction = func() interface{} { return nil }
		m.state.ConfirmVisible = true

	case "d":
		m.state.ConfirmMsg = fmt.Sprintf("DELETE %s? This cannot be undone!", m.selectionLabel())
		m.state.ConfirmAction = func() interface{} { return nil }
		m.state.ConfirmVisible = true

	case "l":
		// TODO: open log viewer for selected VM
	case "m":
		// TODO: migrate dialog
	case "b":
		// TODO: backup dialog
	case "e":
		cmd := m.actionShell()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

type TaskStartedMsg struct {
	UPID  string
	Node  string
	Label string
}

func (m Model) handleConfirmKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter": // removed "Y"
		if m.state.ConfirmAction != nil {
			cmdIace := m.state.ConfirmAction()
			if cmd, ok := cmdIace.(tea.Cmd); ok && cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.state.ConfirmVisible = false
	case "n", "esc", "q": // removed "N"
		m.state.ConfirmVisible = false
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleSearchKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.state.SearchActive = false
		m.sidebar.ApplyFilter(m.state.SearchQuery, m.state)
	case "backspace":
		if len(m.state.SearchQuery) > 0 {
			m.state.SearchQuery = m.state.SearchQuery[:len(m.state.SearchQuery)-1]
		}
		m.sidebar.ApplyFilter(m.state.SearchQuery, m.state)
	default:
		if len(msg.String()) == 1 {
			m.state.SearchQuery += msg.String()
			m.sidebar.ApplyFilter(m.state.SearchQuery, m.state)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) actionStart() tea.Cmd {
	sel := m.state.Selected
	client := m.apiClient
	switch sel.Kind {
	case state.KindVM:
		if sel.VMStatus == nil {
			return nil
		}
		vm := *sel.VMStatus
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.StartVM(ctx, vm.Node, vm.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: vm.Node, Label: fmt.Sprintf("Starting VM %d %s", vm.VMID, vm.Name)}
		}
	case state.KindContainer:
		if sel.CTStatus == nil {
			return nil
		}
		ct := *sel.CTStatus
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.StartCT(ctx, ct.Node, ct.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: ct.Node, Label: fmt.Sprintf("Starting CT %d %s", ct.VMID, ct.Name)}
		}
	}
	return nil
}

func (m *Model) actionStop() tea.Cmd {
	sel := m.state.Selected
	client := m.apiClient
	switch sel.Kind {
	case state.KindVM:
		if sel.VMStatus == nil {
			return nil
		}
		vm := *sel.VMStatus
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.StopVM(ctx, vm.Node, vm.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: vm.Node, Label: fmt.Sprintf("Stopping VM %d %s", vm.VMID, vm.Name)}
		}
	case state.KindContainer:
		if sel.CTStatus == nil {
			return nil
		}
		ct := *sel.CTStatus
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.StopCT(ctx, ct.Node, ct.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: ct.Node, Label: fmt.Sprintf("Stopping CT %d %s", ct.VMID, ct.Name)}
		}
	}
	return nil
}

func (m *Model) actionShell() tea.Cmd {
	sel := m.state.Selected
	var vmid int
	switch sel.Kind {
	case state.KindVM:
		if sel.VMStatus != nil {
			vmid = sel.VMStatus.VMID
		}
	case state.KindContainer:
		if sel.CTStatus != nil {
			vmid = sel.CTStatus.VMID
		}
	}

	if vmid == 0 {
		return nil
	}

	host, ok := m.sshHosts[vmid]
	if !ok {
		// Can't show error right now unless we dispatch an ActionError, so let's dispatch ActionError.
		return func() tea.Msg {
			return ActionError{Err: fmt.Errorf("no SSH mapping for VMID %d in ~/.config/lazypx/ssh.yaml", vmid)}
		}
	}

	target := host.Host
	if host.User != "" {
		target = host.User + "@" + host.Host
	}

	args := []string{}
	if host.IdentityFile != "" {
		args = append(args, "-i", host.IdentityFile)
	}
	if host.Port != 0 {
		args = append(args, "-p", strconv.Itoa(host.Port))
	}
	// Avoid StrictHostKeyChecking issues for dynamic VMs sometimes
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	args = append(args, target)

	var c *exec.Cmd
	// Use sshpass if password is provided
	if host.Password != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			args = append([]string{"-e", "ssh"}, args...)
			c = exec.Command("sshpass", args...)
			c.Env = append(os.Environ(), "SSHPASS="+host.Password)
		} else {
			// Fallback: regular SSH (will prompt)
			c = exec.Command("ssh", args...)
		}
	} else {
		c = exec.Command("ssh", args...)
	}

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return ActionError{Err: fmt.Errorf("ssh session ended: %w", err)}
		}
		// Refresh UI when we get back
		return RefreshTick{}
	})
}

func (m *Model) selectionLabel() string {
	switch m.state.Selected.Kind {
	case state.KindVM:
		if m.state.Selected.VMStatus != nil {
			return fmt.Sprintf("VM %d (%s)", m.state.Selected.VMStatus.VMID, m.state.Selected.VMStatus.Name)
		}
	case state.KindContainer:
		if m.state.Selected.CTStatus != nil {
			return fmt.Sprintf("CT %d (%s)", m.state.Selected.CTStatus.VMID, m.state.Selected.CTStatus.Name)
		}
	case state.KindNode:
		return "node " + m.state.Selected.NodeName
	}
	return "selection"
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	l := m.layout
	if l.TermW == 0 || l.TermH == 0 {
		return ""
	}

	header := m.renderHeader()
	var body string

	if m.state.Error != "" && m.state.Snapshot.IsEmpty() {
		body = renderErrorScreen(m.state.Error, l.SidebarOuterW+l.DetailOuterW, l.SidebarOuterH)
	} else if m.state.Loading && m.state.Snapshot.IsEmpty() {
		body = renderLoadingScreen(m.spinner, l.SidebarOuterW+l.DetailOuterW, l.SidebarOuterH)
	} else {
		body = m.renderMain()
	}

	taskPane := m.tasks.View(l.TasksInnerW, l.TasksInnerH, m.state.FocusedPanel == state.PanelTasks)
	keyBar := m.renderKeyBar()

	full := lipgloss.JoinVertical(lipgloss.Left, header, body, taskPane, keyBar)

	// Overlays (rendered on top)
	if m.state.HelpVisible {
		full = renderOverlay(full, m.help.View(l.TermW), l.TermW, l.TermH)
	} else if m.state.ConfirmVisible {
		full = renderOverlay(full, m.confirm.View(l.TermW), l.TermW, l.TermH)
	} else if m.state.SearchActive {
		full = renderOverlay(full, m.search.View(l.TermW), l.TermW, l.TermH)
	}

	return full
}

func (m Model) renderHeader() string {
	l := m.layout
	profileStyle := StyleTitle
	prodIndicator := ""
	if m.state.Production {
		profileStyle = StyleTitleProd
		prodIndicator = StyleTitleProd.Render(" ● PRODUCTION")
	}

	left := StyleTitle.Render("  lazypx") + StyleSubtext.Render(" v0.1.0")
	right := profileStyle.Render(" ⎔ "+m.state.ProfileName) + prodIndicator

	gap := l.TermW - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 0 {
		gap = 0
	}
	return lipgloss.NewStyle().
		Background(colorBgAlt).
		Width(l.TermW).
		Render(left + spaces(gap) + right)
}

func (m Model) renderMain() string {
	sidebarPane := m.sidebar.View(m.state.FocusedPanel)
	detailPane := m.detail.View(m.state.FocusedPanel == state.PanelDetail)
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarPane, detailPane)
}

func (m Model) renderKeyBar() string {
	l := m.layout
	keys := []struct{ k, d string }{
		{"s", "start"}, {"x", "stop"}, {"r", "reboot"}, {"d", "delete"},
		{"l", "logs"}, {"e", "shell"}, {"m", "migrate"}, {"b", "backup"},
		{"/", "search"}, {"f", "refresh"}, {"?", "help"}, {"q", "quit"},
	}
	bar := ""
	for _, kd := range keys {
		entry := StyleKeyBracket.Render("["+kd.k+"]") + StyleKeyHint.Render(kd.d+" ")
		// Stop adding keys if we'd overflow
		if lipgloss.Width(bar+entry) > l.TermW-2 {
			break
		}
		bar += entry
	}
	return lipgloss.NewStyle().
		Background(colorKeyBg).
		Width(l.TermW).
		Render(bar)
}

func renderLoadingScreen(s spinner.Model, w, h int) string {
	msg := StyleSpinner.Render(s.View()) + "  " + StyleValue.Render("Connecting to cluster…")
	return lipgloss.NewStyle().Width(w).Height(h).
		Align(lipgloss.Center, lipgloss.Center).Render(msg)
}

func renderErrorScreen(err string, w, h int) string {
	msg := StyleError2.Render("✗  "+err+"\n\n") +
		StyleSubtext.Render("  Check ~/.config/lazypx/config.yaml and press R to retry.")
	return lipgloss.NewStyle().Width(w).Height(h).
		Align(lipgloss.Center, lipgloss.Center).Render(msg)
}

func renderOverlay(bg, overlay string, w, h int) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#00000088")))
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
