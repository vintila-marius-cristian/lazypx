package tui

import (
	"lazypx/state"
	"testing"
)

func TestListPane_WrapAround(t *testing.T) {
	pane := NewListPane(state.PanelNodes, "Nodes")
	pane.Items = []ListItem{
		{ID: "node1", Label: "Node 1"},
		{ID: "node2", Label: "Node 2"},
		{ID: "node3", Label: "Node 3"},
	}
	pane.Cursor = 0

	// Test MoveUp wraps to bottom
	pane.MoveUp()
	if pane.Cursor != 2 {
		t.Errorf("expected cursor to wrap to 2, got %d", pane.Cursor)
	}

	// Test MoveDown wraps to top
	pane.MoveDown()
	if pane.Cursor != 0 {
		t.Errorf("expected cursor to wrap to 0, got %d", pane.Cursor)
	}

	// Test MoveDown normal behavior
	pane.MoveDown()
	if pane.Cursor != 1 {
		t.Errorf("expected cursor to move down to 1, got %d", pane.Cursor)
	}

	// Test MoveUp normal behavior
	pane.MoveUp()
	if pane.Cursor != 0 {
		t.Errorf("expected cursor to move up to 0, got %d", pane.Cursor)
	}

	// Test Empty List
	emptyPane := NewListPane(state.PanelNodes, "Empty")
	emptyPane.Cursor = 0
	emptyPane.MoveDown()
	if emptyPane.Cursor != 0 {
		t.Errorf("expected empty pane to not move cursor")
	}
	emptyPane.MoveUp()
	if emptyPane.Cursor != 0 {
		t.Errorf("expected empty pane to not move cursor")
	}
}
