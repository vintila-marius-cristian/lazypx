package tui

import (
	"fmt"
	"strings"
	"time"

	"lazypx/sessions"
	"lazypx/state"
)

// SessionsModel is the TUI overlay for listing and managing tmux-backed SSH sessions.
type SessionsModel struct {
	st       *state.AppState
	manager  *sessions.Manager
	sessions []sessions.SessionInfo
	cursor   int
	width    int
	height   int
	err      string
}

// NewSessionsModel creates a new sessions overlay model.
func NewSessionsModel(st *state.AppState, mgr *sessions.Manager) SessionsModel {
	return SessionsModel{st: st, manager: mgr}
}

// Refresh re-polls tmux for the current session list.
func (m *SessionsModel) Refresh() {
	m.sessions = m.manager.ListSessions()
	m.err = ""
	if m.cursor >= len(m.sessions) {
		m.cursor = len(m.sessions) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// MoveUp moves the cursor up in the sessions list.
func (m *SessionsModel) MoveUp() {
	if m.cursor > 0 {
		m.cursor--
	} else if len(m.sessions) > 0 {
		m.cursor = len(m.sessions) - 1
	}
}

// MoveDown moves the cursor down in the sessions list.
func (m *SessionsModel) MoveDown() {
	if m.cursor < len(m.sessions)-1 {
		m.cursor++
	} else if len(m.sessions) > 0 {
		m.cursor = 0
	}
}

// Selected returns the currently selected session, or nil.
func (m *SessionsModel) Selected() *sessions.SessionInfo {
	if len(m.sessions) == 0 || m.cursor >= len(m.sessions) {
		return nil
	}
	return &m.sessions[m.cursor]
}

// View renders the sessions overlay.
func (m SessionsModel) View(outerW, outerH int) string {
	w := outerW - 6
	if w > 80 {
		w = 80
	}
	if w < 30 {
		w = 30
	}

	var sb strings.Builder
	sb.WriteString(StyleTitle.Render(" 🖥  Shell Sessions") + "\n\n")

	if m.err != "" {
		sb.WriteString("  " + StyleError2.Render(m.err) + "\n\n")
	}

	if !sessions.HasTmux() {
		sb.WriteString("  " + StyleError2.Render("tmux is not installed.") + "\n")
		sb.WriteString("  " + StyleSubtext.Render("Install tmux for persistent shell sessions.") + "\n")
		sb.WriteString("  " + StyleSubtext.Render("brew install tmux  /  apt install tmux") + "\n\n")
		sb.WriteString("  " + StyleSubtext.Render("[esc] close") + "\n")
		content := sb.String()
		return StyleOverlayBox.Width(w).Render(content)
	}

	if len(m.sessions) == 0 {
		sb.WriteString("  " + StyleSubtext.Render("No active sessions.") + "\n")
		sb.WriteString("  " + StyleSubtext.Render("Press [e] on a VM/CT to start one.") + "\n\n")
	} else {
		for i, s := range m.sessions {
			cursor := "  "
			nameStyle := StyleValue
			if i == m.cursor {
				cursor = StyleRunning.Render("▸ ")
				nameStyle = StyleTitle
			}

			elapsed := time.Since(s.StartedAt).Truncate(time.Second)
			statusIcon := StyleRunning.Render("●")
			if s.Status != "running" {
				statusIcon = StyleError2.Render("○")
			}
			attachedFlag := ""
			if s.Attached {
				attachedFlag = StyleMagenta.Render(" [attached]")
			}

			line := fmt.Sprintf("%s%s VM %d  %s  %s%s",
				cursor,
				statusIcon,
				s.VMID,
				nameStyle.Render(s.Key),
				StyleSubtext.Render(formatElapsed(elapsed)),
				attachedFlag,
			)
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  " + StyleSubtext.Render("[enter] attach  [d] close  [esc] back") + "\n")

	content := sb.String()
	return StyleOverlayBox.Width(w).Render(content)
}

func formatElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
