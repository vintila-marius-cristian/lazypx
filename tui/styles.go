package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Base colors
	colorBgAlt       = lipgloss.Color("#16161e")
	colorBorder      = lipgloss.Color("#3b4261")
	colorBorderFocus = lipgloss.Color("#7aa2f7")
	colorText        = lipgloss.Color("#c0caf5")
	colorSubtext     = lipgloss.Color("#565f89")
	colorHeader      = lipgloss.Color("#7aa2f7")
	colorGreen       = lipgloss.Color("#9ece6a")
	colorRed         = lipgloss.Color("#f7768e")
	colorYellow      = lipgloss.Color("#e0af68")
	colorMagenta     = lipgloss.Color("#bb9af7")
	colorCyan        = lipgloss.Color("#7dcfff")
	colorSelected    = lipgloss.Color("#364a82")
	colorKeyBg       = lipgloss.Color("#1f2335")

	// Status badges
	StyleRunning   = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	StyleStopped   = lipgloss.NewStyle().Foreground(colorRed)
	StyleSuspended = lipgloss.NewStyle().Foreground(colorYellow)
	StyleError     = lipgloss.NewStyle().Foreground(colorMagenta).Bold(true)

	// Pane borders
	StylePaneBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	StylePaneBorderFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorderFocus)

	// Title bar
	StyleTitle = lipgloss.NewStyle().
			Foreground(colorHeader).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)

	StyleTitleProd = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)

	// Tree items
	StyleTreeItem = lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(1)

	StyleTreeItemSelected = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorCyan).
				Bold(true).
				PaddingLeft(1)

	StyleTreeNode = lipgloss.NewStyle().
			Foreground(colorHeader).
			Bold(true).
			PaddingLeft(1)

	StyleTreeNodeSelected = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorCyan).
				Bold(true).
				PaddingLeft(1)

	// Detail pane
	StyleLabel = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Width(14)

	StyleValue = lipgloss.NewStyle().
			Foreground(colorText)

	StyleGaugeBar = lipgloss.NewStyle().
			Foreground(colorGreen)

	StyleGaugeFill = lipgloss.NewStyle().
			Foreground(colorBorder)

	// Key hint bar
	StyleKeyHint = lipgloss.NewStyle().
			Background(colorKeyBg).
			Foreground(colorSubtext).
			PaddingLeft(1).
			PaddingRight(1)

	StyleKeyBracket = lipgloss.NewStyle().
			Background(colorKeyBg).
			Foreground(colorCyan).
			Bold(true)

	// Help overlay
	StyleHelp = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderFocus).
			Background(colorBgAlt).
			Padding(1, 2)

	StyleHelpTitle = lipgloss.NewStyle().
			Foreground(colorHeader).
			Bold(true).
			MarginBottom(1)

	StyleHelpKey = lipgloss.NewStyle().
			Foreground(colorCyan).
			Width(14)

	StyleHelpDesc = lipgloss.NewStyle().
			Foreground(colorText)

	// Confirm modal
	StyleConfirm = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Background(colorBgAlt).
			Padding(1, 3)

	StyleConfirmTitle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true).
				MarginBottom(1)

	StyleConfirmMsg = lipgloss.NewStyle().
			Foreground(colorText)

	// Task log
	StyleTaskOK  = lipgloss.NewStyle().Foreground(colorGreen)
	StyleTaskErr = lipgloss.NewStyle().Foreground(colorRed)
	StyleTaskRun = lipgloss.NewStyle().Foreground(colorYellow)
	StyleTaskLog = lipgloss.NewStyle().Foreground(colorSubtext)

	// Loading
	StyleSpinner = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)

	// Error
	StyleError2 = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			PaddingLeft(2)

	// Search
	StyleSearch = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Background(colorBgAlt).
			Padding(0, 1)

	// Subtext (secondary info, dim labels)
	StyleSubtext = lipgloss.NewStyle().Foreground(colorSubtext)

	// Overlay box (sessions picker, etc.)
	StyleOverlayBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Background(colorBgAlt).
			Padding(1, 2)
)

// GaugeBar renders a percentage bar like ████░░░░ 78%
func GaugeBar(width int, pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(float64(width) * pct)
	empty := width - filled
	if empty < 0 {
		empty = 0
	}
	bar := StyleGaugeBar.Render(repeatRune('█', filled)) +
		StyleGaugeFill.Render(repeatRune('░', empty))
	return bar
}

func repeatRune(r rune, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}

// StatusDot returns a colored status indicator.
func StatusDot(status string) string {
	switch status {
	case "running":
		return StyleRunning.Render("●")
	case "stopped":
		return StyleStopped.Render("○")
	case "suspended":
		return StyleSuspended.Render("◐")
	default:
		return StyleError.Render("⊘")
	}
}

// StatusStyle returns the style for a status string.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return StyleRunning
	case "stopped":
		return StyleStopped
	case "suspended":
		return StyleSuspended
	default:
		return StyleError
	}
}
