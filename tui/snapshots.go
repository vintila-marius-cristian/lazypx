package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"lazypx/api"
	"lazypx/state"
)

// SnapshotsLoadedMsg indicates the snapshots have been fetched from the API.
type SnapshotsLoadedMsg struct {
	Snapshots []api.Snapshot
	Err       error
}

// SnapshotsModel represents the interactive snapshots overlay.
type SnapshotsModel struct {
	st      *state.AppState
	client  *api.Client
	spinner spinner.Model

	snapshots []api.Snapshot
	cursor    int
	loading   bool
	err       error
	width     int
	height    int
}

func NewSnapshotsModel(st *state.AppState, client *api.Client) SnapshotsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSpinner
	return SnapshotsModel{
		st:      st,
		client:  client,
		spinner: s,
	}
}

func (m SnapshotsModel) Init() tea.Cmd {
	return nil
}

// Load executes the API call to get snapshots for the currently selected VM/CT.
func (m *SnapshotsModel) Load() tea.Cmd {
	m.loading = true
	m.err = nil
	m.snapshots = nil
	m.cursor = 0

	sel := m.st.Selected
	if sel.Kind != state.KindVM && sel.Kind != state.KindContainer {
		m.loading = false
		m.err = fmt.Errorf("no VM or Container selected")
		return nil
	}

	node := sel.NodeName
	var vmid int
	var kind string

	if sel.Kind == state.KindVM && sel.VMStatus != nil {
		node = sel.VMStatus.Node
		vmid = sel.VMStatus.VMID
		kind = "qemu"
	} else if sel.Kind == state.KindContainer && sel.CTStatus != nil {
		node = sel.CTStatus.Node
		vmid = sel.CTStatus.VMID
		kind = "lxc"
	} else {
		m.loading = false
		return nil
	}

	client := m.client
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			snaps, err := client.GetSnapshots(ctx, node, vmid, kind)
			return SnapshotsLoadedMsg{Snapshots: snaps, Err: err}
		},
	)
}

func (m SnapshotsModel) Update(msg tea.Msg) (SnapshotsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case SnapshotsLoadedMsg:
		m.loading = false
		m.err = msg.Err
		m.snapshots = msg.Snapshots
		if m.cursor >= len(m.snapshots) {
			m.cursor = len(m.snapshots) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.st.SnapshotsVisible = false
		case "j", "down":
			if m.cursor < len(m.snapshots)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter", "r": // rollback
			if len(m.snapshots) > 0 && m.cursor < len(m.snapshots) {
				snap := m.snapshots[m.cursor]
				m.st.ConfirmMsg = fmt.Sprintf("Rollback to snapshot '%s'?", snap.Name)
				m.st.ConfirmAction = m.actionRollback(snap.Name)
				m.st.ConfirmVisible = true
			}
		case "d", "x": // delete
			if len(m.snapshots) > 0 && m.cursor < len(m.snapshots) {
				// Prevent deleting "current" which is often just a placeholder in PVE, though PVE handles it
				snap := m.snapshots[m.cursor]
				if snap.Name == "current" {
					break // cannot delete current state in standard sense
				}
				m.st.ConfirmMsg = fmt.Sprintf("DELETE snapshot '%s'?", snap.Name)
				m.st.ConfirmAction = m.actionDelete(snap.Name)
				m.st.ConfirmVisible = true
			}
		case "a", "c", "n": // add/create
			m.st.ConfirmMsg = "Snapshot name? (Currently auto-names using timestamp)"
			// A proper text input is required for custom names.
			// For now, auto-create one with a timestamp as we don't have a textinput model wired yet.
			// Or we just implement the action directly. Let's auto-name for simplicity until text input is added.
			m.st.ConfirmAction = m.actionCreate()
			m.st.ConfirmVisible = true
		}
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *SnapshotsModel) actionRollback(snapname string) func() interface{} {
	node := m.targetNode()
	vmid := m.targetVMID()
	kind := m.targetKind()
	client := m.client

	return func() interface{} {
		m.st.SnapshotsVisible = false
		var cmd tea.Cmd = func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.RollbackSnapshot(ctx, node, vmid, kind, snapname)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: node, Label: fmt.Sprintf("Rollback %d to %s", vmid, snapname)}
		}
		return cmd
	}
}

func (m *SnapshotsModel) actionDelete(snapname string) func() interface{} {
	node := m.targetNode()
	vmid := m.targetVMID()
	kind := m.targetKind()
	client := m.client

	return func() interface{} {
		m.st.SnapshotsVisible = false
		var cmd tea.Cmd = func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.DeleteSnapshot(ctx, node, vmid, kind, snapname)
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: node, Label: fmt.Sprintf("Deleting snapshot %s on %d", snapname, vmid)}
		}
		return cmd
	}
}

func (m *SnapshotsModel) actionCreate() func() interface{} {
	node := m.targetNode()
	vmid := m.targetVMID()
	kind := m.targetKind()
	client := m.client
	snapname := fmt.Sprintf("auto_%d", time.Now().Unix())

	return func() interface{} {
		m.st.SnapshotsVisible = false
		var cmd tea.Cmd = func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			upid, err := client.CreateSnapshot(ctx, node, vmid, kind, snapname, "lazypx auto snapshot")
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: node, Label: fmt.Sprintf("Creating snapshot %s on %d", snapname, vmid)}
		}
		return cmd
	}
}

func (m *SnapshotsModel) targetNode() string {
	sel := m.st.Selected
	if sel.VMStatus != nil {
		return sel.VMStatus.Node
	}
	if sel.CTStatus != nil {
		return sel.CTStatus.Node
	}
	return sel.NodeName
}
func (m *SnapshotsModel) targetVMID() int {
	sel := m.st.Selected
	if sel.VMStatus != nil {
		return sel.VMStatus.VMID
	}
	if sel.CTStatus != nil {
		return sel.CTStatus.VMID
	}
	return 0
}
func (m *SnapshotsModel) targetKind() string {
	sel := m.st.Selected
	if sel.VMStatus != nil {
		return "qemu"
	}
	if sel.CTStatus != nil {
		return "lxc"
	}
	return ""
}

// View overlays the snapshots list on the center of the terminal.
func (m SnapshotsModel) View() string {
	w := OverlayWidth(60, m.width) // Make it fairly wide

	var content string

	if m.loading {
		content = fmt.Sprintf("  %s Loading snapshots...\n", m.spinner.View())
	} else if m.err != nil {
		content = StyleTaskErr.Render(fmt.Sprintf("  Error loading snapshots:\n  %v", m.err))
	} else if len(m.snapshots) == 0 {
		content = "  No snapshots found for this resource.\n"
	} else {
		for i, snap := range m.snapshots {
			var cursor string
			if m.cursor == i {
				cursor = ">"
			} else {
				cursor = " "
			}

			// Format name
			name := snap.Name
			if name == "current" {
				name = StyleLabel.Render("current (running state)")
			}

			// Format row
			row := fmt.Sprintf("%s %-15s", cursor, name)

			if snap.Description != "" {
				row += StyleValue.Render("  - " + snap.Description)
			}

			if m.cursor == i {
				content += StyleTreeNodeSelected.Render(" "+row) + "\n"
			} else {
				content += StyleTreeItem.Render(" "+row) + "\n"
			}
		}
	}

	footer := StyleHelpDesc.Render("\n\n  [enter]: rollback   [x]: delete   [c]: create   [esc]: close")

	res := StyleConfirmTitle.Render(" Snapshots ") + "\n\n" + content + footer
	return StyleConfirm.Width(w).Render(res)
}
