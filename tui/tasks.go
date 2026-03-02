package tui

import (
	"fmt"

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

	tasks := m.st.ActiveTasks
	maxRows := innerH - 1 // -1 for title line
	if maxRows < 1 {
		maxRows = 1
	}

	if len(tasks) == 0 {
		inner += "  " + StyleSubtext.Render("No tasks yet.") + "\n"
	} else {
		start := len(tasks) - maxRows
		if start < 0 {
			start = 0
		}
		for _, t := range tasks[start:] {
			inner += renderTaskRow(t, innerW) + "\n"
		}
	}

	content := clipToHeight(inner, innerH)
	return style.Width(innerW).Height(innerH).Render(content)
}

func renderTaskRow(t state.ActiveTask, maxW int) string {
	var status string
	if t.Done {
		if t.Success {
			status = StyleTaskOK.Render("[OK] ")
		} else {
			status = StyleTaskErr.Render("[ERR]")
		}
	} else {
		status = StyleTaskRun.Render("[.. ]")
	}

	label := StyleValue.Render(t.Label)
	logLine := ""
	if len(t.Logs) > 0 {
		logLine = "  " + StyleTaskLog.Render("└ "+t.Logs[len(t.Logs)-1])
	}

	line := fmt.Sprintf("  %s  %s", status, label)
	if logLine != "" {
		line += "\n" + logLine
	}
	return line
}
