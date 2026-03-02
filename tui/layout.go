package tui

import (
	"lazypx/state"
	"strings"
)

// Layout holds all computed pane dimensions for a given terminal size.
// All values are >= 0. Frame sizes are derived from Lip Gloss styles, not hardcoded.
type Layout struct {
	TermW, TermH int

	// Frame sizes derived from StylePaneBorder
	PaneFrameW int // horizontal frame (left border + right border)
	PaneFrameH int // vertical frame (top border + bottom border)

	// Fixed elements
	HeaderH int // always 1
	KeyBarH int // always 1

	// Sidebar pane (left)
	SidebarOuterW, SidebarOuterH int
	SidebarInnerW, SidebarInnerH int

	// Individual accordion heights (outer, inclusive of borders)
	NodesOuterH   int
	VMsOuterH     int
	CTsOuterH     int
	StorageOuterH int

	// Detail pane (right)
	DetailOuterW, DetailOuterH int
	DetailInnerW, DetailInnerH int

	// Tasks pane (bottom, full width)
	TasksOuterW, TasksOuterH int
	TasksInnerW, TasksInnerH int
}

// Layout constants — tunable knobs.
const (
	minSidebarInnerW = 30 // reduced minimum for narrower layout
	maxSidebarPct    = 32 // reduced maximum percentage
	sidebarPct       = 25 // 25% of terminal for the sidebar lists (was 35%)
	minDetailInner   = 36
	minTasksInnerH   = 3  // minimum content rows for tasks
	maxTasksOuterH   = 14 // tasks pane cap on very tall terminals
	tasksPctH        = 20 // tasks pane as % of terminal height
)

// ComputeLayout is a pure function that computes all pane dimensions from
// terminal size and the currently focused panel.
//
// Layout invariant:
//
//	HeaderH + MainOuterH + TasksOuterH + KeyBarH == TermH
//	SidebarOuterW + DetailOuterW == TermW
func ComputeLayout(termW, termH int, focusedPanel state.PanelType) Layout {
	// Derive frame size from the actual border style — never hardcode.
	frameW, frameH := StylePaneBorder.GetFrameSize()

	l := Layout{
		TermW:      termW,
		TermH:      termH,
		PaneFrameW: frameW,
		PaneFrameH: frameH,
		HeaderH:    1,
		KeyBarH:    1,
	}

	// ── Vertical budget ───────────────────────────────────────────────────
	// header(1) + mainOuter + tasksOuter + keybar(1) = termH
	available := termH - l.HeaderH - l.KeyBarH
	if available < 0 {
		available = 0
	}

	// Tasks pane height: scales with terminal, bounded.
	tasksOuter := clamp(termH*tasksPctH/100, minTasksInnerH+frameH, maxTasksOuterH)
	if tasksOuter > available/2 {
		tasksOuter = available / 2 // never take more than half
	}
	if tasksOuter < frameH+1 {
		tasksOuter = frameH + 1
	}

	mainOuter := available - tasksOuter
	if mainOuter < frameH+1 {
		mainOuter = frameH + 1
	}

	// ── Horizontal budget ─────────────────────────────────────────────────
	// sidebarOuter + detailOuter = termW
	sidebarOuter := termW * sidebarPct / 100
	minSidebarOuter := minSidebarInnerW + frameW
	maxSidebarOuter := termW * maxSidebarPct / 100

	sidebarOuter = clamp(sidebarOuter, minSidebarOuter, maxSidebarOuter)
	if termW-sidebarOuter < minDetailInner+frameW {
		sidebarOuter = termW - minDetailInner - frameW
	}
	if sidebarOuter < minSidebarOuter {
		sidebarOuter = minSidebarOuter
	}
	if sidebarOuter > termW {
		sidebarOuter = termW
	}

	detailOuter := termW - sidebarOuter
	if detailOuter < 0 {
		detailOuter = 0
	}

	// ── Assign computed values ────────────────────────────────────────────
	l.SidebarOuterW = sidebarOuter
	l.SidebarOuterH = mainOuter
	l.SidebarInnerW = max0(sidebarOuter - frameW)
	l.SidebarInnerH = max0(mainOuter - frameH)

	l.DetailOuterW = detailOuter
	l.DetailOuterH = mainOuter
	l.DetailInnerW = max0(detailOuter - frameW)
	l.DetailInnerH = max0(mainOuter - frameH)

	l.TasksOuterW = termW
	l.TasksOuterH = tasksOuter
	l.TasksInnerW = max0(termW - frameW)
	l.TasksInnerH = max0(tasksOuter - frameH)

	// ── Sidebar vertical distribution (Accordion) ─────────────────────────
	// Total available for the 4 lists is l.SidebarInnerH (content area of sidebar pane).
	// But we might completely remove the outer sidebar border if we style them as 4 distinct bordered panes.
	// For now, let's assume they are 4 independent borders stacked.
	// Total available = mainOuter.
	// We have 4 panels.
	// An unfocused panel gets exactly `frameH + 1` (title header) or `frameH + 2` (title + summary).
	// Let's give unfocused panels: frameH + 1 rows of height (so they are very thin, just the title).
	const numPanels = 4
	unfocusedOuterH := frameH + 1
	var nodesH, vmsH, ctsH, storageH int

	// Base allocation
	nodesH = unfocusedOuterH
	vmsH = unfocusedOuterH
	ctsH = unfocusedOuterH
	storageH = unfocusedOuterH

	// Determine available height for the focused panel
	sumUnfocused := unfocusedOuterH * (numPanels - 1)
	focusedOuterH := mainOuter - sumUnfocused
	if focusedOuterH < unfocusedOuterH {
		focusedOuterH = unfocusedOuterH
	}

	switch focusedPanel {
	case state.PanelNodes:
		nodesH = focusedOuterH
	case state.PanelVMs:
		vmsH = focusedOuterH
	case state.PanelCTs:
		ctsH = focusedOuterH
	case state.PanelStorage:
		storageH = focusedOuterH
	default:
		nodesH = focusedOuterH
	}

	// Strictly enforce the invariant: sum must equal mainOuter.
	// If the terminal is tiny, some accordions get crushed < unfocusedOuterH.
	// We iteratively shave off 1 row from the largest panels until they fit,
	// or pad them if there's leftover remainder.
	for {
		used := nodesH + vmsH + ctsH + storageH
		if used == mainOuter {
			break
		}
		if used < mainOuter {
			// Pad the focused one (or nodesH if none)
			switch focusedPanel {
			case state.PanelVMs:
				vmsH++
			case state.PanelCTs:
				ctsH++
			case state.PanelStorage:
				storageH++
			default:
				nodesH++
			}
		} else { // used > mainOuter
			// Shrink the largest one
			maxH := nodesH
			var target *int = &nodesH
			if vmsH > maxH {
				maxH = vmsH
				target = &vmsH
			}
			if ctsH > maxH {
				maxH = ctsH
				target = &ctsH
			}
			if storageH > maxH {
				maxH = storageH
				target = &storageH
			}

			if *target > 0 {
				*target--
			} else {
				break // panic safety, shouldn't hit 0 unless mainOuter is 0
			}
		}
	}

	l.NodesOuterH = nodesH
	l.VMsOuterH = vmsH
	l.CTsOuterH = ctsH
	l.StorageOuterH = storageH

	return l
}

// GaugeWidth computes gauge bar width based on available inner width.
// It allocates 40% of the space remaining after labels and padding,
// bounded between 14 and 32 characters.
func GaugeWidth(innerW int) int {
	// Label column is 14 chars wide (StyleLabel.Width), plus 4 chars padding
	remaining := innerW - 18
	if remaining < 0 {
		remaining = 0
	}
	gw := remaining * 40 / 100
	return clamp(gw, 14, 32)
}

// OverlayWidth computes a bounded overlay width that fits the terminal.
func OverlayWidth(maxW, termW int) int {
	w := termW - 6
	if w > maxW {
		w = maxW
	}
	if w < 20 {
		w = 20
	}
	return w
}

// clamp returns v bounded by lo and hi.
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// max0 returns max(v, 0).
func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// clipToHeight ensures a string has NO MORE than maxH lines.
// This prevents Lipgloss containers from expanding beyond their layout budget
// and causing Bubbletea to print more lines than the terminal height.
func clipToHeight(s string, maxH int) string {
	if maxH <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxH {
		// Truncate to maxH lines
		return strings.Join(lines[:maxH], "\n")
	}
	return s
}
