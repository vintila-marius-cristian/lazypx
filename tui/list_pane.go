package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazypx/state"
)

// ListItem represents a single row in a ListPane.
type ListItem struct {
	ID      string      // unique identifier (NodeName, VMID as string, etc)
	Label   string      // e.g "VM 100"
	Name    string      // e.g "ubuntu-docker"
	Status  string      // e.g "running"
	RawData interface{} // Pointers to api.NodeStatus, api.VMStatus, etc
}

// ListPane represents one of the accordion panels in the sidebar.
type ListPane struct {
	PanelType state.PanelType
	Title     string
	Items     []ListItem
	Cursor    int

	width  int
	height int
}

func NewListPane(ptype state.PanelType, title string) *ListPane {
	return &ListPane{
		PanelType: ptype,
		Title:     title,
	}
}

func (m *ListPane) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.clampCursor()
}

func (m *ListPane) clampCursor() {
	if len(m.Items) == 0 {
		m.Cursor = 0
		return
	}
	if m.Cursor >= len(m.Items) {
		m.Cursor = len(m.Items) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

func (m *ListPane) MoveUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

func (m *ListPane) MoveDown() {
	if m.Cursor < len(m.Items)-1 {
		m.Cursor++
	}
}

func (m *ListPane) SelectedItem() *ListItem {
	if len(m.Items) == 0 || m.Cursor >= len(m.Items) {
		return nil
	}
	return &m.Items[m.Cursor]
}

// View renders the list pane. It handles its own border based on whether it is focused.
func (m *ListPane) View(focused bool) string {
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	// 1. Determine borders
	style := StylePaneBorder
	if focused {
		style = StylePaneBorderFocused
	}

	// We are given OUTER height constraints (to stack correctly in lipgloss).
	// We need to calculate inner dimensions for our content.
	frameW, frameH := style.GetFrameSize()
	innerW := m.width - frameW
	innerH := m.height - frameH
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	// 2. Render Title
	title := StyleTitle.Render(" " + m.Title)

	// 3. Render Items
	var rows []string
	if innerH > 1 {
		// Calculate virtual scrolling bounds
		maxRows := innerH - 1 // space left after title
		if maxRows < 0 {
			maxRows = 0
		}

		start := m.Cursor - maxRows/2
		if start < 0 {
			start = 0
		}
		end := start + maxRows
		if end > len(m.Items) {
			end = len(m.Items)
			start = end - maxRows
			if start < 0 {
				start = 0
			}
		}

		for i := start; i < end && i < len(m.Items); i++ {
			item := m.Items[i]
			selected := focused && (i == m.Cursor)
			// Secondary selection (grayed out) if panel isn't focused but item is selected
			secondary := !focused && (i == m.Cursor)

			rows = append(rows, m.renderRow(item, selected, secondary, innerW))
		}
	}

	// Join title and rows, and strictly clip to inner height to prevent vertical bounds overflow
	content := title
	if len(rows) > 0 {
		content += "\n" + strings.Join(rows, "\n")
	}

	content = clipToHeight(content, innerH)
	return style.Width(innerW).Height(innerH).Render(content)
}

func (m *ListPane) renderRow(item ListItem, selected, secondary bool, innerW int) string {
	dot := StatusDot(item.Status)

	// Available label width
	maxTextW := innerW - 1 // PaddingLeft(1)
	if maxTextW < 4 {
		maxTextW = 4
	}

	plainText := fmt.Sprintf("%s  %s", item.Label, item.Name)
	plainText = truncate(plainText, maxTextW-2) // -2 for dot + space

	styled := dot + " " + plainText

	if selected {
		return StyleTreeNodeSelected.Render(styled)
	}
	if secondary {
		// A slightly highlighted but unfocused look
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			PaddingLeft(1).
			Render(styled)
	}
	return StyleTreeItem.Render(styled)
}

// truncate clips a string to maxW runes, adding … if needed (working on raw text).
func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return string(runes[:maxW-1]) + "…"
}
