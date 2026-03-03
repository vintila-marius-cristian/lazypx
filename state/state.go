// Package state holds the global application state shared between TUI and commands.
package state

import (
	"lazypx/api"
	"lazypx/cache"
)

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
	ConfirmAction  func() interface{}

	SnapshotsVisible bool
	BackupsVisible   bool

	// Loading
	Loading bool
	Error   string

	// Profile name for display
	ProfileName string
	Production  bool
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
	if idx < len(s.ActiveTasks) {
		s.ActiveTasks[idx].Logs = append(s.ActiveTasks[idx].Logs, line)
	}
}

// MarkTaskDone marks a task as done.
func (s *AppState) MarkTaskDone(idx int, success bool) {
	if idx < len(s.ActiveTasks) {
		s.ActiveTasks[idx].Done = true
		s.ActiveTasks[idx].Success = success
	}
}
