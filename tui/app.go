package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazypx/api"
	"lazypx/audit"
	"lazypx/cache"
	"lazypx/config"
	"lazypx/sessions"
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

// TaskLogLine is (historically) kept if needed elsewhere, but we will use taskLogStreamMsg internally.
type TaskLogLine struct {
	Index int
	Line  string
}

type taskLogStreamMsg struct {
	Idx  int
	Line string
	Next tea.Cmd
}

type taskDoneStreamMsg struct {
	Idx     int
	Success bool
}

func watchChannelCmd(ch <-chan api.TaskLog, idx int, client *api.Client, node, upid string) tea.Cmd {
	return func() tea.Msg {
		log, ok := <-ch
		if !ok {
			// Channel closed, task is done. Let's get the final status.
			status, err := client.GetTaskStatus(context.Background(), node, upid)
			success := true
			if err == nil && status.ExitStatus != "OK" {
				success = false
			}
			return taskDoneStreamMsg{Idx: idx, Success: success}
		}
		return taskLogStreamMsg{
			Idx:  idx,
			Line: log.T,
			Next: watchChannelCmd(ch, idx, client, node, upid),
		}
	}
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
	sidebar         SidebarModel
	detail          DetailModel
	tasks           TasksModel
	help            HelpModel
	confirm         ConfirmModel
	search          SearchModel
	snapshots       SnapshotsModel
	backups         BackupsModel
	sessionsOverlay SessionsModel

	sessionsMgr *sessions.Manager
	// shellPanes holds one embedded terminal per session key.
	shellPanes map[string]*ShellPane
	spinner    spinner.Model
	layout     Layout // centralized layout dimensions

	// VM extras debounce: avoid firing API calls on every cursor move.
	lastExtrasVMID int
	lastExtrasAt   time.Time
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

	mgr := sessions.New(profileName)

	// Initialize audit log — best effort, never blocks startup.
	audit.Init(config.ConfigDir())

	return Model{
		state:           st,
		apiClient:       apiClient,
		cache:           clusterCache,
		cfg:             cfg,
		sshHosts:        ssh,
		spinner:         s,
		sidebar:         NewSidebarModel(st),
		detail:          NewDetailModel(st),
		tasks:           NewTasksModel(st),
		help:            NewHelpModel(),
		confirm:         NewConfirmModel(st),
		search:          NewSearchModel(st),
		snapshots:       NewSnapshotsModel(st, apiClient),
		backups:         NewBackupsModel(st, apiClient),
		sessionsMgr:     mgr,
		sessionsOverlay: NewSessionsModel(st, mgr), // share the same manager
		shellPanes:      make(map[string]*ShellPane),
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

	prevVMID := m.state.Selected.VMID
	prevKind := m.state.Selected.Kind

	// Highest Priority: Confirm Dialog traps ALL keys
	if m.state.ConfirmVisible {
		if keyMsg, isKey := msg.(tea.KeyMsg); isKey {
			var cmd tea.Cmd
			newM, cmd := m.handleConfirmKey(keyMsg, cmds)
			return newM.(Model), cmd
		}
	}

	if m.state.SnapshotsVisible {
		var cmd tea.Cmd
		m.snapshots, cmd = m.snapshots.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if _, ok := msg.(tea.WindowSizeMsg); !ok {
			if _, isKey := msg.(tea.KeyMsg); isKey {
				return m, tea.Batch(cmds...)
			}
		}
	}

	if m.state.BackupsVisible {
		var cmd tea.Cmd
		m.backups, cmd = m.backups.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if _, ok := msg.(tea.WindowSizeMsg); !ok {
			if _, isKey := msg.(tea.KeyMsg); isKey {
				return m, tea.Batch(cmds...)
			}
		}
	}

	if m.state.SessionsVisible {
		if keyMsg, isKey := msg.(tea.KeyMsg); isKey {
			switch keyMsg.String() {
			case "esc":
				m.state.SessionsVisible = false
			case "k", "up":
				m.sessionsOverlay.MoveUp()
			case "j", "down":
				m.sessionsOverlay.MoveDown()
			case "d":
				if sel := m.sessionsOverlay.Selected(); sel != nil {
					m.state.ConfirmMsg = fmt.Sprintf("Terminate session %s?", sel.Key)
					keyToKill := sel.Key
					m.state.ConfirmAction = func() interface{} {
						m.sessionsMgr.CloseSession(keyToKill)
						m.sessionsOverlay.Refresh()
						return nil
					}
					m.state.ConfirmVisible = true
				}
			case "enter":
				if sel := m.sessionsOverlay.Selected(); sel != nil {
					m.state.SessionsVisible = false
					key := sel.Key
					if sp, hasSP := m.shellPanes[key]; hasSP && !sp.ended {
						// Session has an embedded pane — switch to it.
						m.state.ActiveShellKey = key
						m.state.ShellFocused = true
					} else {
						// No embedded pane — fall back to full-screen attach.
						return m, tea.Exec(m.sessionsMgr.AttachCmd(key), func(err error) tea.Msg {
							if err != nil {
								return ActionError{Err: err}
							}
							return RefreshTick{}
						})
					}
				}
			}
			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.layout = ComputeLayout(msg.Width, msg.Height, m.state.FocusedPanel)
		// Pass layout struct to sidebar, inner sizes to others
		m.sidebar.SetSize(m.layout)
		m.detail.SetSize(m.layout.DetailInnerW, m.layout.DetailInnerH)
		m.tasks.SetSize(m.layout.TasksInnerW, m.layout.TasksInnerH)
		// Resize all shell panes and their underlying PTYs.
		for key, sp := range m.shellPanes {
			sp.SetSize(m.layout.DetailInnerW, m.layout.DetailInnerH)
			m.sessionsMgr.ResizePTY(key, m.layout.DetailInnerW, m.layout.DetailInnerH) //nolint:errcheck
		}

	case ShellOutputMsg:
		if sp, ok := m.shellPanes[msg.Key]; ok {
			if msg.Data != nil {
				sp.Feed(msg.Data)
			}
			// Keep the read pipeline alive.
			if cmd := sp.StartReadCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case ShellExitedMsg:
		if sp, ok := m.shellPanes[msg.Key]; ok {
			sp.ended = true
			sp.exitErr = msg.Err
			m.state.AddLocalEvent(fmt.Sprintf("Shell exited: %s", msg.Key), "warn")
		}
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case ClusterLoaded: // Renamed from ClusterLoaded to ClusterSnapshotMsg in the instruction, but keeping original name as per "make the change faithfully"
		m.state.Loading = false
		if msg.Snapshot.Error != nil {
			m.state.Error = fmt.Sprintf("Failed to connect: %v", msg.Snapshot.Error)
		} else {
			m.state.Error = ""
			m.state.Snapshot = msg.Snapshot
			m.sidebar.Sync(m.state)
			for _, e := range msg.Snapshot.Errors {
				m.state.AddLocalEvent(e, "warn")
			}
		}
		d := time.Duration(m.cfg.RefreshInterval) * time.Second
		cmds = append(cmds, tickRefresh(d))

	case VMExtrasLoadedMsg:
		if m.state.Selected.Kind == state.KindVM && m.state.Selected.VMID == msg.VMID {
			m.state.Selected.VMConfig = msg.Config
			m.state.Selected.GuestIPs = msg.IPs
			m.detail.Sync(m.state)
		}
		return m, tea.Batch(cmds...)

	case ClusterRefreshed:
		if msg.Snapshot.Error == nil {
			m.state.Snapshot = msg.Snapshot
			m.sidebar.Sync(m.state)
			for _, e := range msg.Snapshot.Errors {
				m.state.AddLocalEvent(e, "warn")
			}
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
		ch := make(chan api.TaskLog, 100)
		go m.apiClient.WatchTask(context.Background(), msg.Node, msg.UPID, ch)
		cmds = append(cmds, watchChannelCmd(ch, idx, m.apiClient, msg.Node, msg.UPID))

	case taskLogStreamMsg:
		m.state.AppendTaskLog(msg.Idx, msg.Line)
		if msg.Next != nil {
			cmds = append(cmds, msg.Next)
		}

	case TaskLogLine: // legacy, keep around just in case
		m.state.AppendTaskLog(msg.Index, msg.Line)

	case taskDoneStreamMsg:
		m.state.MarkTaskDone(msg.Idx, msg.Success)
		m.state.Loading = true
		m.cache.Invalidate()
		cmds = append(cmds, m.refreshCluster())

	case TaskDone: // legacy
		m.state.MarkTaskDone(msg.Index, msg.Success)
		m.state.Loading = true
		m.cache.Invalidate()
		cmds = append(cmds, m.refreshCluster())

	case tea.KeyMsg:
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

	// Check if selection changed to a new VM — debounce to avoid API calls on every j/k.
	if m.state.Selected.Kind == state.KindVM {
		if prevKind != state.KindVM || prevVMID != m.state.Selected.VMID {
			now := time.Now()
			if m.lastExtrasVMID != m.state.Selected.VMID || now.Sub(m.lastExtrasAt) > 2*time.Second {
				m.lastExtrasVMID = m.state.Selected.VMID
				m.lastExtrasAt = now
				cmds = append(cmds, m.loadVMExtrasCmd(m.state.Selected.NodeName, m.state.Selected.VMID))
			}
		}
	}

	// Keep ActiveShellKey in sync with the current selection.
	// When the user navigates to a different item, show that item's shell (if any).
	if prevVMID != m.state.Selected.VMID || prevKind != m.state.Selected.Kind {
		newKey := m.shellKeyForSelection()
		if newKey != m.state.ActiveShellKey {
			m.state.ActiveShellKey = newKey
			m.state.ShellFocused = false // unfocus when navigating away
		}
	}

	return m, tea.Batch(cmds...)
}

// VMExtrasLoadedMsg contains on-demand data for a VM.
type VMExtrasLoadedMsg struct {
	VMID   int
	Config *api.VMConfig
	IPs    []api.GuestNetworkInterface
}

func (m Model) loadVMExtrasCmd(node string, vmid int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cfg, _ := m.apiClient.GetVMConfig(ctx, node, vmid)
		ips, _ := m.apiClient.GetGuestAgentNetworkInterfaces(ctx, node, vmid)

		return VMExtrasLoadedMsg{VMID: vmid, Config: cfg, IPs: ips}
	}
}

func (m Model) handleKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	// When shell is focused, all keystrokes go to the PTY.
	if m.state.ShellFocused && m.state.ActiveShellKey != "" {
		return m.handleShellKey(msg, cmds)
	}

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

	case "c":
		if m.state.Selected.Kind == state.KindVM || m.state.Selected.Kind == state.KindContainer {
			m.state.SnapshotsVisible = true
			cmds = append(cmds, m.snapshots.Load())
		}

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
		m.state.ConfirmAction = func() interface{} {
			return m.actionReboot()
		}
		m.state.ConfirmVisible = true

	case "d":
		m.state.ConfirmMsg = fmt.Sprintf("DELETE %s? This cannot be undone!", m.selectionLabel())
		m.state.ConfirmAction = func() interface{} {
			return m.actionDelete()
		}
		m.state.ConfirmVisible = true

	case "m":
		if m.state.Selected.Kind == state.KindVM && m.state.Selected.VMStatus != nil {
			vm := m.state.Selected.VMStatus
			target := m.findMigrationTarget(vm.Node)
			if target == "" {
				cmds = append(cmds, func() tea.Msg {
					return ActionError{Err: fmt.Errorf("no target node available for migration")}
				})
			} else {
				m.state.ConfirmMsg = fmt.Sprintf("Migrate VM %d to %s?", vm.VMID, target)
				m.state.ConfirmAction = func() interface{} {
					return m.actionMigrate(vm.Node, vm.VMID, target)
				}
				m.state.ConfirmVisible = true
			}
		}
	case "b":
		if m.state.Selected.Kind == state.KindVM || m.state.Selected.Kind == state.KindContainer {
			m.state.BackupsVisible = true
			cmds = append(cmds, m.backups.Load())
		}
	case "t":
		m.state.SessionsVisible = true
		m.sessionsOverlay.Refresh()

	case "e":
		// If a shell pane already exists for this VM, focus it.
		// Otherwise open a new embedded shell session.
		if key := m.shellKeyForSelection(); key != "" {
			m.state.ActiveShellKey = key
			m.state.ShellFocused = true
		} else {
			cmd := m.actionShell()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case "ctrl+w":
		// Close the shell VIEW (session keeps running).
		if m.state.ActiveShellKey != "" {
			m.closeShellView()
		}

	case "ctrl+u":
		// Scroll shell view up into history (when shell is visible but not focused).
		if sp, ok := m.shellPanes[m.state.ActiveShellKey]; ok {
			sp.ScrollUp(5)
		}

	case "ctrl+d":
		// Scroll shell view down toward the live screen.
		if sp, ok := m.shellPanes[m.state.ActiveShellKey]; ok {
			sp.ScrollDown(5)
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
		m.auditLog("START", fmt.Sprintf("vm:%d", vm.VMID))
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
		m.auditLog("START", fmt.Sprintf("ct:%d", ct.VMID))
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
		m.auditLog("STOP", fmt.Sprintf("vm:%d", vm.VMID))
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
		m.auditLog("STOP", fmt.Sprintf("ct:%d", ct.VMID))
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

func (m *Model) actionReboot() tea.Cmd {
	sel := m.state.Selected
	client := m.apiClient
	switch sel.Kind {
	case state.KindVM:
		if sel.VMStatus == nil {
			return nil
		}
		vm := *sel.VMStatus
		m.auditLog("REBOOT", fmt.Sprintf("vm:%d", vm.VMID))
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.RebootVM(ctx, vm.Node, vm.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: vm.Node, Label: fmt.Sprintf("Rebooting VM %d %s", vm.VMID, vm.Name)}
		}
	case state.KindContainer:
		if sel.CTStatus == nil {
			return nil
		}
		ct := *sel.CTStatus
		m.auditLog("REBOOT", fmt.Sprintf("ct:%d", ct.VMID))
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.RebootCT(ctx, ct.Node, ct.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: ct.Node, Label: fmt.Sprintf("Rebooting CT %d %s", ct.VMID, ct.Name)}
		}
	}
	return nil
}

func (m *Model) actionDelete() tea.Cmd {
	sel := m.state.Selected
	client := m.apiClient
	switch sel.Kind {
	case state.KindVM:
		if sel.VMStatus == nil {
			return nil
		}
		vm := *sel.VMStatus
		m.auditLog("DELETE", fmt.Sprintf("vm:%d", vm.VMID))
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.DeleteVM(ctx, vm.Node, vm.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: vm.Node, Label: fmt.Sprintf("Deleting VM %d %s", vm.VMID, vm.Name)}
		}
	case state.KindContainer:
		if sel.CTStatus == nil {
			return nil
		}
		ct := *sel.CTStatus
		m.auditLog("DELETE", fmt.Sprintf("ct:%d", ct.VMID))
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.DeleteCT(ctx, ct.Node, ct.VMID)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: ct.Node, Label: fmt.Sprintf("Deleting CT %d %s", ct.VMID, ct.Name)}
		}
	}
	return nil
}

// findMigrationTarget returns the first online node that is not currentNode.
func (m *Model) findMigrationTarget(currentNode string) string {
	for _, n := range m.state.Snapshot.Nodes {
		if n.Node != currentNode && n.Status == "online" {
			return n.Node
		}
	}
	return ""
}

func (m *Model) actionMigrate(node string, vmid int, target string) tea.Cmd {
	client := m.apiClient
	m.auditLog("MIGRATE", fmt.Sprintf("vm:%d to %s", vmid, target))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		upid, err := client.MigrateVM(ctx, node, vmid, target, true)
		if err != nil {
			return ActionError{Err: err}
		}
		return TaskStartedMsg{UPID: upid, Node: node, Label: fmt.Sprintf("Migrating VM %d → %s", vmid, target)}
	}
}

// actionShell opens an embedded PTY shell for the selected VM/CT.
// It replaces the detail pane with a ShellPane and starts async PTY reading.
func (m *Model) actionShell() tea.Cmd {
	sel := m.state.Selected
	var vmid int
	var vmName string
	var kind state.ResourceKind

	switch sel.Kind {
	case state.KindVM:
		if sel.VMStatus == nil {
			return nil
		}
		vmid = sel.VMStatus.VMID
		vmName = sel.VMStatus.Name
		kind = state.KindVM
	case state.KindContainer:
		if sel.CTStatus == nil {
			return nil
		}
		vmid = sel.CTStatus.VMID
		vmName = sel.CTStatus.Name
		kind = state.KindContainer
	default:
		return nil
	}

	host, ok := m.sshHosts[vmid]
	if !ok {
		return func() tea.Msg {
			return ActionError{Err: fmt.Errorf("no SSH mapping for VMID %d in ~/.config/lazypx/ssh.yaml", vmid)}
		}
	}

	target := host.Host
	if host.User != "" {
		target = host.User + "@" + host.Host
	}
	var args []string
	if host.IdentityFile != "" {
		args = append(args, "-i", host.IdentityFile)
	}
	if host.Port != 0 {
		args = append(args, "-p", strconv.Itoa(host.Port))
	}
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	args = append(args, target)

	sessionKey := m.sessionsMgr.SessionKey(vmid)

	// OpenSession is a no-op if the process is still running.
	if err := m.sessionsMgr.OpenSession(sessionKey, "ssh", args); err != nil {
		return func() tea.Msg {
			return ActionError{Err: fmt.Errorf("failed to start session: %w", err)}
		}
	}

	// Create the embedded ShellPane (or reuse if it already exists).
	sp, exists := m.shellPanes[sessionKey]
	if !exists || sp.ended {
		sp = NewShellPane(
			sessionKey, vmid, vmName, kind,
			m.sessionsMgr,
			m.layout.DetailInnerW, m.layout.DetailInnerH,
		)
		m.shellPanes[sessionKey] = sp
	}

	m.state.ActiveShellKey = sessionKey
	m.state.ShellFocused = true

	// Resize PTY to match current pane dimensions.
	m.sessionsMgr.ResizePTY(sessionKey, m.layout.DetailInnerW, m.layout.DetailInnerH) //nolint:errcheck

	// Emit local event.
	kindStr := "VM"
	if kind == state.KindContainer {
		kindStr = "CT"
	}
	m.state.AddLocalEvent(fmt.Sprintf("Shell opened: %s %d (%s)", kindStr, vmid, vmName), "info")

	// Start the async PTY read pipeline.
	return sp.StartReadCmd()
}

// shellKeyForSelection returns the session key for the currently selected VM/CT
// if an embedded ShellPane exists for it, otherwise "".
func (m Model) shellKeyForSelection() string {
	var vmid int
	switch m.state.Selected.Kind {
	case state.KindVM:
		if m.state.Selected.VMStatus != nil {
			vmid = m.state.Selected.VMStatus.VMID
		}
	case state.KindContainer:
		if m.state.Selected.CTStatus != nil {
			vmid = m.state.Selected.CTStatus.VMID
		}
	}
	if vmid == 0 {
		return ""
	}
	key := m.sessionsMgr.SessionKey(vmid)
	if _, ok := m.shellPanes[key]; ok {
		return key
	}
	return ""
}

// closeShellView hides the shell pane for the current VM (session keeps running).
func (m *Model) closeShellView() {
	key := m.state.ActiveShellKey
	m.state.ActiveShellKey = ""
	m.state.ShellFocused = false
	if key != "" {
		m.state.AddLocalEvent(fmt.Sprintf("Shell view closed: %s", key), "info")
	}
}

// handleShellKey routes keypresses to the PTY when the shell has focus.
// ctrl+q unfocuses (returns to tree navigation); ctrl+w closes the shell view.
func (m Model) handleShellKey(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	s := msg.String()

	switch s {
	case "ctrl+q":
		// Detach focus; shell stays visible and running.
		m.state.ShellFocused = false
		return m, tea.Batch(cmds...)

	case "ctrl+w":
		// Close shell view entirely (session keeps running).
		m.closeShellView()
		return m, tea.Batch(cmds...)
	}

	// Forward everything else to the PTY.
	if sp, ok := m.shellPanes[m.state.ActiveShellKey]; ok && !sp.ended {
		if data := KeyToShellBytes(msg); data != nil {
			sp.WriteToShell(data)
			// Any input resets the scroll view to live.
			if sp.IsScrolled() {
				sp.ScrollReset()
			}
		}
	}

	return m, tea.Batch(cmds...)
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

// auditUser extracts the user portion from a token_id like "root@pam!mytoken".
func (m *Model) auditUser() string {
	if m.cfg == nil || m.cfg.TokenID == "" {
		return "unknown"
	}
	// token_id format: "user@realm!tokenname"
	id := m.cfg.TokenID
	if idx := strings.Index(id, "!"); idx > 0 {
		return id[:idx]
	}
	return id
}

// auditLog writes an audit entry. Never blocks the primary operation.
func (m *Model) auditLog(action, resource string) {
	profile := "default"
	if m.cfg != nil && m.cfg.Name != "" {
		profile = m.cfg.Name
	}
	audit.Log(profile, m.auditUser(), action, resource)
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

	// Overlays (rendered on bottom-to-top z-index)
	if m.state.HelpVisible {
		full = renderOverlay(full, m.help.View(l.TermW), l.TermW, l.TermH)
	} else if m.state.SnapshotsVisible {
		full = renderOverlay(full, m.snapshots.View(), l.TermW, l.TermH)
	} else if m.state.BackupsVisible {
		full = renderOverlay(full, m.backups.View(), l.TermW, l.TermH)
	} else if m.state.SearchActive {
		full = renderOverlay(full, m.search.View(l.TermW), l.TermW, l.TermH)
	} else if m.state.SessionsVisible {
		full = renderOverlay(full, m.sessionsOverlay.View(l.TermW, l.TermH), l.TermW, l.TermH)
	}

	// Confirm renders over EVERYTHING else
	if m.state.ConfirmVisible {
		full = renderOverlay(full, m.confirm.View(l.TermW), l.TermW, l.TermH)
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

	var detailPane string
	if m.state.ActiveShellKey != "" {
		if sp, ok := m.shellPanes[m.state.ActiveShellKey]; ok {
			detailPane = sp.View(m.state.ShellFocused)
		} else {
			detailPane = m.detail.View(m.state.FocusedPanel == state.PanelDetail)
		}
	} else {
		detailPane = m.detail.View(m.state.FocusedPanel == state.PanelDetail)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarPane, detailPane)
}

func (m Model) renderKeyBar() string {
	l := m.layout

	var keys []struct{ k, d string }
	if m.state.ShellFocused && m.state.ActiveShellKey != "" {
		// Shell input mode: show detach / close / scroll hints.
		keys = []struct{ k, d string }{
			{"ctrl+q", "unfocus shell"},
			{"ctrl+w", "close shell view"},
			{"ctrl+u", "scroll ↑"},
			{"ctrl+d", "scroll ↓"},
		}
	} else if m.state.ActiveShellKey != "" {
		// Shell visible but tree-focused: show shell nav hints.
		keys = []struct{ k, d string }{
			{"e", "focus shell"},
			{"ctrl+w", "close shell view"},
			{"ctrl+u", "scroll ↑"},
			{"ctrl+d", "scroll ↓"},
			{"s", "start"}, {"x", "stop"}, {"c", "snapshots"},
			{"/", "search"}, {"?", "help"}, {"q", "quit"},
		}
	} else {
		keys = []struct{ k, d string }{
			{"s", "start"}, {"x", "stop"}, {"r", "reboot"}, {"d", "delete"},
			{"c", "snapshots"}, {"e", "shell"}, {"m", "migrate"}, {"b", "backup"},
			{"/", "search"}, {"f", "refresh"}, {"?", "help"}, {"q", "quit"},
		}
	}

	bar := ""
	for _, kd := range keys {
		entry := StyleKeyBracket.Render("["+kd.k+"]") + StyleKeyHint.Render(kd.d+" ")
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
