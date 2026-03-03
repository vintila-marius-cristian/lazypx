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

type BackupsLoadedMsg struct {
	Backups []api.BackupVolume
	Err     error
}

// BackupsModel represents the interactive backups overlay.
type BackupsModel struct {
	st      *state.AppState
	client  *api.Client
	spinner spinner.Model

	backups []api.BackupVolume
	cursor  int
	loading bool
	err     error
	width   int
	height  int
}

func NewBackupsModel(st *state.AppState, client *api.Client) BackupsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleSpinner
	return BackupsModel{
		st:      st,
		client:  client,
		spinner: s,
	}
}

func (m BackupsModel) Init() tea.Cmd {
	return nil
}

// Load executes the API call to get backups for the currently selected VM/CT.
func (m *BackupsModel) Load() tea.Cmd {
	m.loading = true
	m.err = nil
	m.backups = nil
	m.cursor = 0

	sel := m.st.Selected
	if sel.Kind != state.KindVM && sel.Kind != state.KindContainer {
		m.loading = false
		m.err = fmt.Errorf("no VM or Container selected")
		return nil
	}

	node := sel.NodeName
	var vmid int

	if sel.Kind == state.KindVM && sel.VMStatus != nil {
		node = sel.VMStatus.Node
		vmid = sel.VMStatus.VMID
	} else if sel.Kind == state.KindContainer && sel.CTStatus != nil {
		node = sel.CTStatus.Node
		vmid = sel.CTStatus.VMID
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
			backs, err := client.GetBackups(ctx, node, vmid)
			return BackupsLoadedMsg{Backups: backs, Err: err}
		},
	)
}

func (m BackupsModel) Update(msg tea.Msg) (BackupsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case BackupsLoadedMsg:
		m.loading = false
		m.err = msg.Err
		m.backups = msg.Backups
		if m.cursor >= len(m.backups) {
			m.cursor = len(m.backups) - 1
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
			m.st.BackupsVisible = false
		case "j", "down":
			if m.cursor < len(m.backups)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "b", "c", "n": // create new backup
			m.st.ConfirmMsg = "Trigger VZDump Backup (snapshot mode + zstd)?"
			m.st.ConfirmAction = m.actionCreate()
			m.st.ConfirmVisible = true
		}
		// Note: Restore and Delete backups are deliberately not implemented yet
		// as restoring fully replaces VM configurations and requires very careful validation.
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *BackupsModel) actionCreate() func() interface{} {
	node := m.targetNode()
	vmid := m.targetVMID()
	client := m.client

	return func() interface{} {
		m.st.BackupsVisible = false // close overlay synchronously
		var cmd tea.Cmd = func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			// Empty storage triggers auto-detect
			upid, err := client.CreateBackup(ctx, node, vmid, "")
			if err != nil {
				return ActionError{Err: err}
			}
			return TaskStartedMsg{UPID: upid, Node: node, Label: fmt.Sprintf("Backup VM %d", vmid)}
		}
		return cmd
	}
}

func (m *BackupsModel) targetNode() string {
	sel := m.st.Selected
	if sel.VMStatus != nil {
		return sel.VMStatus.Node
	}
	if sel.CTStatus != nil {
		return sel.CTStatus.Node
	}
	return sel.NodeName
}
func (m *BackupsModel) targetVMID() int {
	sel := m.st.Selected
	if sel.VMStatus != nil {
		return sel.VMStatus.VMID
	}
	if sel.CTStatus != nil {
		return sel.CTStatus.VMID
	}
	return 0
}

// View overlays the backups list on the center of the terminal.
func (m BackupsModel) View() string {
	w := OverlayWidth(70, m.width) // Make it fairly wide

	var content string

	if m.loading {
		content = fmt.Sprintf("  %s Loading backups...\n", m.spinner.View())
	} else if m.err != nil {
		content = StyleTaskErr.Render(fmt.Sprintf("  Error loading backups:\n  %v", m.err))
	} else if len(m.backups) == 0 {
		content = "  No backups found for this resource.\n"
	} else {
		for i, back := range m.backups {
			var cursor string
			if m.cursor == i {
				cursor = ">"
			} else {
				cursor = " "
			}

			// Format size
			sizeMB := back.Size / (1024 * 1024)

			// Format time
			t := time.Unix(back.CTime, 0).Format("2006-01-02 15:04")

			// Format row
			row := fmt.Sprintf("%s [%s] %d MB  %s", cursor, t, sizeMB, back.VolID)
			row = truncate(row, w-6)

			if m.cursor == i {
				content += StyleTreeNodeSelected.Render(" "+row) + "\n"
			} else {
				content += StyleTreeItem.Render(" "+row) + "\n"
			}
		}
	}

	footer := StyleHelpDesc.Render("\n\n  [b]: trigger backup   [esc]: close")

	res := StyleConfirmTitle.Render(" VZDump Backups ") + "\n\n" + content + footer
	return StyleConfirm.Width(w).Render(res)
}
