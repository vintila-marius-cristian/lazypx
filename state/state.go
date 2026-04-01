// Package state holds the global application state shared between TUI and commands.
package state

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazypx/api"
	"lazypx/cache"
)

// LocalEvent is a UI-level event (not a Proxmox task) such as shell open/close.
type LocalEvent struct {
	Label string
	Level string // "info", "warn", "error"
	At    time.Time
}

// PanelType identifies which pane is currently focused.
type PanelType int

const (
	PanelNodes PanelType = iota
	PanelVMs
	PanelCTs
	PanelStorage
	PanelDetail
	PanelTasks
)

// ResourceKind identifies what type of resource is selected.
type ResourceKind int

const (
	KindNone ResourceKind = iota
	KindNode
	KindVM
	KindContainer
	KindStorage
)

// Selection represents the currently selected item in the tree.
type Selection struct {
	Kind        ResourceKind
	NodeName    string
	VMID        int
	VMStatus    *api.VMStatus
	CTStatus    *api.CTStatus
	NodeStatus  *api.NodeStatus
	VMConfig    *api.VMConfig               // loaded on demand; may be nil
	GuestIPs    []api.GuestNetworkInterface // loaded on demand; may be nil
	StorageName string                      // for KindStorage
}

// ActiveTask is a task that was triggered and is being watched.
type ActiveTask struct {
	UPID    string
	Node    string
	Label   string
	Done    bool
	Success bool
	Logs    []string
}

// AppState is the top-level state object.
type AppState struct {
	// Cluster data (from cache)
	Snapshot cache.ClusterSnapshot

	// UI focus
	FocusedPanel PanelType

	// Sidebar selections (independent cursors per panel)
	SelectedNode    string
	SelectedVM      int
	SelectedCT      int
	SelectedStorage string

	// The active overall selection (what detail pane shows)
	Selected Selection

	// Task pane
	ActiveTasks []ActiveTask
	TaskOffset  int

	// Search
	SearchActive bool
	SearchQuery  string

	// Overlays
	HelpVisible    bool
	ConfirmVisible bool
	ConfirmMsg     string
	ConfirmAction  func() tea.Cmd

	SnapshotsVisible bool
	BackupsVisible   bool
	SessionsVisible  bool

	// Shell pane state
	ShellFocused   bool   // true when keystrokes route to the embedded shell
	ActiveShellKey string // session key shown in the detail pane ("" = show details)

	// Loading
	Loading bool
	Error   string

	// Profile name for display
	ProfileName string
	Production  bool

	// Local events (shell open/close, etc.) shown in the tasks pane.
	LocalEvents []LocalEvent
}

// New creates an initial (empty) AppState.
func New(profileName string, production bool) *AppState {
	return &AppState{
		ProfileName: profileName,
		Production:  production,
	}
}

// AddTask adds a new active task. Returns its index.
func (s *AppState) AddTask(upid, node, label string) int {
	s.ActiveTasks = append(s.ActiveTasks, ActiveTask{
		UPID:  upid,
		Node:  node,
		Label: label,
	})
	return len(s.ActiveTasks) - 1
}

// AppendTaskLog appends a log line to an active task by index.
func (s *AppState) AppendTaskLog(idx int, line string) {
	if idx >= 0 && idx < len(s.ActiveTasks) {
		s.ActiveTasks[idx].Logs = append(s.ActiveTasks[idx].Logs, line)
	}
}

// MarkTaskDone marks a task as done.
func (s *AppState) MarkTaskDone(idx int, success bool) {
	if idx >= 0 && idx < len(s.ActiveTasks) {
		s.ActiveTasks[idx].Done = true
		s.ActiveTasks[idx].Success = success
	}
}

// AddLocalEvent appends a local (non-Proxmox) event to the event log.
// Keeps the last 50 entries.
func (s *AppState) AddLocalEvent(label, level string) {
	s.LocalEvents = append(s.LocalEvents, LocalEvent{
		Label: label,
		Level: level,
		At:    time.Now(),
	})
	if len(s.LocalEvents) > 50 {
		s.LocalEvents = s.LocalEvents[len(s.LocalEvents)-50:]
	}
}
