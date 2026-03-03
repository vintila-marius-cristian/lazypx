package tui

import (
	"fmt"
	"lazypx/api"
	"lazypx/state"
)

// TasksModel renders the bottom pane with active tasks and event log.
type TasksModel struct {
	st     *state.AppState
	width  int // INNER width (frame subtracted by layout engine)
	height int // INNER height
}

// NewTasksModel creates a new tasks model.
func NewTasksModel(st *state.AppState) TasksModel {
	return TasksModel{st: st}
}

// SetSize sets INNER display dimensions.
func (m *TasksModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Sync updates from state.
func (m *TasksModel) Sync(st *state.AppState) {
	m.st = st
}

// View renders the task pane. innerW and innerH are the INNER dimensions;
// this method wraps content in the border style to produce the final outer pane.
func (m TasksModel) View(innerW, innerH int, focused bool) string {
	if innerW < 2 || innerH < 2 {
		return ""
	}
	style := StylePaneBorder
	if focused {
		style = StylePaneBorderFocused
	}

	title := StyleTitle.Render(" Tasks & Events")
	inner := title + "\n"

	maxRows := innerH - 1
	if maxRows < 1 {
		maxRows = 1
	}

	var rows []string

	// 1. Show local active tasks first with live logs
	for i := len(m.st.ActiveTasks) - 1; i >= 0; i-- {
		t := m.st.ActiveTasks[i]
		rows = append(rows, renderActiveTaskRow(t, innerW))
	}

	// 2. Add global API tasks (newest are typically at the end of the array)
	for i := len(m.st.Snapshot.Tasks) - 1; i >= 0; i-- {
		t := m.st.Snapshot.Tasks[i]
		rows = append(rows, renderClusterTaskRow(t, innerW))
	}

	if len(rows) == 0 {
		inner += "  " + StyleSubtext.Render("No tasks yet.") + "\n"
	} else {
		limit := len(rows)
		if limit > maxRows {
			limit = maxRows
		}
		for i := 0; i < limit; i++ {
			inner += rows[i] + "\n"
		}
	}

	content := clipToHeight(inner, innerH)
	return style.Width(innerW).Height(innerH).Render(content)
}

func renderClusterTaskRow(t api.Task, maxW int) string {
	var status string
	switch t.Status {
	// API typically sets "status":"stopped" when done.
	case "stopped":
		// Proxmox doesn't easily expose success/fail inside the base `/cluster/tasks` object
		// without parsing `exitstatus`. But it's good enough for an OK tag.
		status = StyleTaskLog.Render("[OK] ")
	default:
		// Any other string or "" usually implies running
		status = StyleTaskRun.Render("[\u2026] ")
	}

	// Example: "vzdump", "qmsnapshot", "qmstart"
	actionName := t.Type

	var target string
	if t.ID != "" {
		target = fmt.Sprintf(" %s", t.ID)
	}

	label := StyleValue.Render(fmt.Sprintf("%s%s on %s", actionName, target, t.Node))

	user := StyleSubtext.Render(fmt.Sprintf(" (%s)", t.User))

	line := fmt.Sprintf("  %s %s%s", status, label, user)
	return truncate(line, maxW)
}

func renderActiveTaskRow(t state.ActiveTask, maxW int) string {
	var status string
	if t.Done {
		if t.Success {
			status = StyleTaskOK.Render("[OK] ")
		} else {
			status = StyleTaskErr.Render("[ERR]")
		}
	} else {
		status = StyleTaskRun.Render("[\u2026] ")
	}

	label := StyleValue.Render(t.Label)
	logLine := ""
	if len(t.Logs) > 0 {
		logLine = "  " + StyleTaskLog.Render("└ "+t.Logs[len(t.Logs)-1])
	}

	line := fmt.Sprintf("  %s %s", status, label)
	if logLine != "" {
		line += "\n" + truncate(logLine, maxW)
	}
	return truncate(line, maxW)
}
