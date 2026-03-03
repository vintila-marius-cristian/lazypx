package tui

// shell_pane.go — ShellPane: embedded interactive SSH terminal in the right pane.
// Connects a sessions.Manager PTY session to a Terminal emulator and renders it
// inside a Lip Gloss border with the same frame dimensions as the detail pane.

import (
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazypx/sessions"
	"lazypx/state"
)

// ── Messages ──────────────────────────────────────────────────────────────────

// ShellOutputMsg carries raw bytes read from a PTY session.
type ShellOutputMsg struct {
	Key  string
	Data []byte
}

// ShellExitedMsg signals that a PTY process has exited or the PTY closed.
type ShellExitedMsg struct {
	Key string
	Err error
}

// ── ShellPane ─────────────────────────────────────────────────────────────────

// ShellPane is the embedded terminal model shown in the right-side detail area.
type ShellPane struct {
	key    string // sessions.Manager session key
	vmid   int
	vmName string
	kind   state.ResourceKind

	mgr  *sessions.Manager
	term *Terminal

	width, height int

	ended     bool
	exitErr   error
	startedAt time.Time
}

// shellTitleRows is the number of rows reserved inside the pane for the
// title / status line at the top (does not include the border frame).
const shellTitleRows = 1

// NewShellPane creates a ShellPane for the given session.
// w and h are INNER pane dimensions (border already subtracted by layout engine).
func NewShellPane(
	key string, vmid int, vmName string, kind state.ResourceKind,
	mgr *sessions.Manager,
	w, h int,
) *ShellPane {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	termH := h - shellTitleRows
	if termH < 1 {
		termH = 1
	}
	return &ShellPane{
		key:       key,
		vmid:      vmid,
		vmName:    vmName,
		kind:      kind,
		mgr:       mgr,
		width:     w,
		height:    h,
		term:      NewTerminal(w, termH),
		startedAt: time.Now(),
	}
}

// SetSize updates pane and terminal dimensions.
func (p *ShellPane) SetSize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	p.width = w
	p.height = h
	termH := h - shellTitleRows
	if termH < 1 {
		termH = 1
	}
	p.term.Resize(w, termH)
}

// Feed writes raw PTY output into the terminal emulator.
func (p *ShellPane) Feed(data []byte) { p.term.Feed(data) }

// ScrollUp scrolls the view into scrollback history.
func (p *ShellPane) ScrollUp(n int) { p.term.ScrollViewUp(n) }

// ScrollDown scrolls the view toward the live screen.
func (p *ShellPane) ScrollDown(n int) { p.term.ScrollViewDown(n) }

// ScrollReset returns to the live view.
func (p *ShellPane) ScrollReset() { p.term.ScrollViewReset() }

// IsScrolled reports whether the view is scrolled into history.
func (p *ShellPane) IsScrolled() bool { return p.term.IsScrolled() }

// StartReadCmd returns a tea.Cmd that reads the next chunk from the PTY.
// Returns nil if the session PTY is not available.
func (p *ShellPane) StartReadCmd() tea.Cmd {
	ptm := p.mgr.GetPTY(p.key)
	if ptm == nil {
		return nil
	}
	return shellReadCmd(p.key, ptm)
}

// WriteToShell sends raw bytes to the PTY input.
func (p *ShellPane) WriteToShell(data []byte) {
	if ptm := p.mgr.GetPTY(p.key); ptm != nil {
		ptm.Write(data) //nolint:errcheck
	}
}

// kindLabel returns "VM" or "CT".
func (p *ShellPane) kindLabel() string {
	if p.kind == state.KindContainer {
		return "CT"
	}
	return "VM"
}

// View renders the shell pane inside a border.
// width and height are INNER dimensions (border subtracted by layout engine).
func (p *ShellPane) View(focused bool) string {
	if p.width < 4 || p.height < 2 {
		return ""
	}
	borderStyle := StylePaneBorder
	if focused {
		borderStyle = StylePaneBorderFocused
	}

	// ── Title line (rendered inside the content area) ─────────────────────
	var statusMark string
	if p.ended {
		statusMark = StyleError2.Render("○")
	} else if p.IsScrolled() {
		statusMark = StyleSuspended.Render("◎")
	} else {
		statusMark = StyleRunning.Render("●")
	}

	titleColor := lipgloss.NewStyle().Foreground(colorCyan)
	if focused {
		titleColor = lipgloss.NewStyle().Foreground(colorBorderFocus)
	}
	titleText := titleColor.Render(
		fmt.Sprintf("%s %s %d — %s", statusMark, p.kindLabel(), p.vmid, p.vmName),
	)
	scrollHint := ""
	if p.IsScrolled() {
		scrollHint = "  " + StyleSubtext.Render("[scrollback]")
	}
	focusHint := ""
	if !focused && !p.ended {
		focusHint = "  " + StyleSubtext.Render("[e] focus")
	}
	titleLine := titleText + scrollHint + focusHint

	// ── Content area ──────────────────────────────────────────────────────
	var termContent string
	if p.ended {
		elapsed := time.Since(p.startedAt).Truncate(time.Second)
		errDetail := ""
		if p.exitErr != nil {
			msg := p.exitErr.Error()
			if msg != "EOF" && msg != "read /dev/ptmx: input/output error" {
				errDetail = "\n  " + StyleSubtext.Render(msg)
			}
		}
		termH := p.height - shellTitleRows
		if termH < 1 {
			termH = 1
		}
		body := "\n\n  " + StyleError2.Render("● Session ended") +
			StyleSubtext.Render(fmt.Sprintf("  (ran for %s)", elapsed)) +
			errDetail +
			"\n\n  " + StyleSubtext.Render("[e] restart  [ctrl+w] close view")
		termContent = lipgloss.NewStyle().Width(p.width).Height(termH).Render(body)
	} else {
		termContent = p.term.Render()
	}

	content := titleLine + "\n" + termContent
	return borderStyle.Width(p.width).Height(p.height).Render(content)
}

// ── PTY reader command ────────────────────────────────────────────────────────

// shellReadCmd reads one chunk from r and returns it as ShellOutputMsg or
// ShellExitedMsg. The caller must schedule the returned Next cmd to continue
// reading, creating a self-chaining read pipeline.
func shellReadCmd(key string, r io.Reader) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := r.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			return ShellOutputMsg{Key: key, Data: data}
		}
		if err != nil {
			return ShellExitedMsg{Key: key, Err: err}
		}
		// Zero-byte non-error read: return empty output so the pipeline continues.
		return ShellOutputMsg{Key: key, Data: nil}
	}
}

// ── Key → PTY byte translation ────────────────────────────────────────────────

// KeyToShellBytes translates a BubbleTea KeyMsg to the byte sequence that
// should be forwarded to the PTY. Returns nil for unrecognised keys.
func KeyToShellBytes(msg tea.KeyMsg) []byte {
	s := msg.String()

	// Single printable rune (includes letters, digits, punctuation).
	runes := []rune(s)
	if len(runes) == 1 && runes[0] >= 0x20 && runes[0] != 0x7f {
		return []byte(string(runes[0]))
	}

	switch s {
	case "enter":
		return []byte{'\r'}
	case "backspace":
		return []byte{0x7f}
	case "tab":
		return []byte{'\t'}
	case "space":
		return []byte{' '}
	case "esc":
		return []byte{0x1b}
	case "delete":
		return []byte{0x1b, '[', '3', '~'}
	case "insert":
		return []byte{0x1b, '[', '2', '~'}
	case "home":
		return []byte{0x1b, '[', 'H'}
	case "end":
		return []byte{0x1b, '[', 'F'}
	case "pgup":
		return []byte{0x1b, '[', '5', '~'}
	case "pgdown":
		return []byte{0x1b, '[', '6', '~'}
	case "up":
		return []byte{0x1b, '[', 'A'}
	case "down":
		return []byte{0x1b, '[', 'B'}
	case "right":
		return []byte{0x1b, '[', 'C'}
	case "left":
		return []byte{0x1b, '[', 'D'}
	case "f1":
		return []byte{0x1b, 'O', 'P'}
	case "f2":
		return []byte{0x1b, 'O', 'Q'}
	case "f3":
		return []byte{0x1b, 'O', 'R'}
	case "f4":
		return []byte{0x1b, 'O', 'S'}
	case "f5":
		return []byte{0x1b, '[', '1', '5', '~'}
	case "f6":
		return []byte{0x1b, '[', '1', '7', '~'}
	case "f7":
		return []byte{0x1b, '[', '1', '8', '~'}
	case "f8":
		return []byte{0x1b, '[', '1', '9', '~'}
	case "f9":
		return []byte{0x1b, '[', '2', '0', '~'}
	case "f10":
		return []byte{0x1b, '[', '2', '1', '~'}
	case "f11":
		return []byte{0x1b, '[', '2', '3', '~'}
	case "f12":
		return []byte{0x1b, '[', '2', '4', '~'}
	}

	// ctrl+a … ctrl+z → bytes 1–26.
	if strings.HasPrefix(s, "ctrl+") {
		char := s[5:]
		if len(char) == 1 {
			ch := char[0]
			if ch >= 'a' && ch <= 'z' {
				return []byte{ch - 'a' + 1}
			}
			switch ch {
			case '[':
				return []byte{0x1b}
			case '\\':
				return []byte{0x1c}
			case ']':
				return []byte{0x1d}
			case '^':
				return []byte{0x1e}
			case '_':
				return []byte{0x1f}
			}
		}
	}

	// alt+X → ESC X.
	if strings.HasPrefix(s, "alt+") {
		rest := s[4:]
		if len(rest) == 1 {
			return []byte{0x1b, rest[0]}
		}
	}

	return nil
}
