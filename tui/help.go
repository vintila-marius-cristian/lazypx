package tui

import (
	"lazypx/state"
)

// HelpModel renders the ? help overlay.
type HelpModel struct{}

// NewHelpModel creates a new HelpModel.
func NewHelpModel() HelpModel {
	return HelpModel{}
}

// View renders the help overlay with responsive width.
func (m HelpModel) View(termW int) string {
	type entry struct{ key, desc string }
	nav := []entry{
		{"j / k / ↑↓", "Navigate list"},
		{"enter", "Expand / inspect"},
		{"tab", "Switch panel"},
	}
	actions := []entry{
		{"s", "Start VM / container"},
		{"x", "Stop VM / container"},
		{"r", "Reboot"},
		{"d", "Delete (confirmation)"},
		{"l", "View logs"},
		{"e", "Open shell (SSH)"},
		{"m", "Migrate"},
		{"b", "Backup"},
	}
	misc := []entry{
		{"/", "Fuzzy search"},
		{"f", "Force refresh"},
		{"?", "Toggle this help"},
		{"q / ctrl+c", "Quit"},
	}

	render := func(title string, items []entry) string {
		out := StyleHelpTitle.Render(title) + "\n"
		for _, e := range items {
			out += StyleHelpKey.Render(e.key) + "  " + StyleHelpDesc.Render(e.desc) + "\n"
		}
		return out
	}

	content := "\n" +
		render("Navigation", nav) + "\n" +
		render("Actions", actions) + "\n" +
		render("General", misc)

	w := OverlayWidth(50, termW)
	return StyleHelp.Width(w).Render(StyleHelpTitle.Render("  lazypx Keybindings\n") + content)
}

// ── Confirm modal ──────────────────────────────────────────────────────────────

// ConfirmModel renders a destructive-action confirmation modal.
type ConfirmModel struct {
	st *state.AppState
}

// NewConfirmModel creates a ConfirmModel.
func NewConfirmModel(st *state.AppState) ConfirmModel {
	return ConfirmModel{st: st}
}

// View renders the confirm overlay with responsive width.
func (m ConfirmModel) View(termW int) string {
	msg := StyleConfirmMsg.Render(m.st.ConfirmMsg)
	buttons := "\n\n  " +
		StyleTaskOK.Render("[y] Confirm") + "   " +
		StyleTaskErr.Render("[n] Cancel")

	w := OverlayWidth(44, termW)
	return StyleConfirm.Width(w).Render(
		StyleConfirmTitle.Render("⚠ Confirmation Required\n") + msg + buttons)
}

// ── Search overlay ────────────────────────────────────────────────────────────

// SearchModel renders the fuzzy search overlay.
type SearchModel struct {
	st *state.AppState
}

// NewSearchModel creates a SearchModel.
func NewSearchModel(st *state.AppState) SearchModel {
	return SearchModel{st: st}
}

// View renders the search box with responsive width.
func (m SearchModel) View(termW int) string {
	content := StyleLabel.Render("Search: ") + StyleValue.Render(m.st.SearchQuery+"_")
	w := OverlayWidth(38, termW)
	return StyleSearch.Width(w).Render(content)
}
