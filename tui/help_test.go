package tui

import (
	"strings"
	"testing"

	"lazypx/state"
)

func TestNewHelpModel(t *testing.T) {
	m := NewHelpModel()
	_ = m
}

func TestHelpModel_View(t *testing.T) {
	m := NewHelpModel()
	v := m.View(120)
	if v == "" {
		t.Fatal("View returned empty string")
	}
	if !strings.Contains(v, "lazypx Keybindings") {
		t.Error("View should contain title")
	}
	if !strings.Contains(v, "Navigate list") {
		t.Error("View should contain navigation section")
	}
	if !strings.Contains(v, "Start VM") {
		t.Error("View should contain start action")
	}
	if !strings.Contains(v, "Fuzzy search") {
		t.Error("View should contain search keybinding")
	}
}

func TestNewConfirmModel(t *testing.T) {
	st := state.New("test", false)
	m := NewConfirmModel(st)
	if m.st != st {
		t.Error("ConfirmModel should hold the state reference")
	}
}

func TestConfirmModel_View(t *testing.T) {
	st := state.New("test", false)
	st.ConfirmMsg = "Delete VM 100?"
	m := NewConfirmModel(st)
	v := m.View(120)
	if !strings.Contains(v, "Confirmation Required") {
		t.Error("View should contain 'Confirmation Required'")
	}
	if !strings.Contains(v, "Delete VM 100?") {
		t.Error("View should contain the confirm message")
	}
	if !strings.Contains(v, "[y] Confirm") {
		t.Error("View should contain confirm button")
	}
	if !strings.Contains(v, "[n] Cancel") {
		t.Error("View should contain cancel button")
	}
}

func TestNewSearchModel(t *testing.T) {
	st := state.New("test", false)
	m := NewSearchModel(st)
	if m.st != st {
		t.Error("SearchModel should hold the state reference")
	}
}

func TestSearchModel_View(t *testing.T) {
	st := state.New("test", false)
	st.SearchQuery = "pve1"
	m := NewSearchModel(st)
	v := m.View(120)
	if !strings.Contains(v, "Search:") {
		t.Error("View should contain 'Search:'")
	}
	if !strings.Contains(v, "pve1") {
		t.Error("View should contain the search query text")
	}
}

func TestSearchModel_View_EmptyQuery(t *testing.T) {
	st := state.New("test", false)
	st.SearchQuery = ""
	m := NewSearchModel(st)
	v := m.View(120)
	if !strings.Contains(v, "Search:") {
		t.Error("View should contain 'Search:' with empty query")
	}
}
