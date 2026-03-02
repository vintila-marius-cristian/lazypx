package tui

import (
	"testing"
	"time"

	"lazypx/api"
	"lazypx/cache"
	"lazypx/config"
	"lazypx/state"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLowercaseKeybindings(t *testing.T) {
	// We'll construct a mostly empty model and send it keystrokes
	apiClient := api.NewClient("http://localhost", "user", "pass", true)
	c := cache.New(apiClient, time.Second)
	cfg := &config.Profile{Name: "test"}
	m := New(apiClient, c, cfg)

	// Inject a selected VM
	m.state.Selected = state.Selection{
		Kind: state.KindVM,
		VMStatus: &api.VMStatus{
			VMID: 105,
			Name: "test-vm",
			Node: "pve1",
		},
	}

	// Helper to send a key and check confirm state
	testKey := func(key string, expectConfirm bool) {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		newModel, _ := m.Update(msg)
		updated := newModel.(Model)
		if updated.state.ConfirmVisible != expectConfirm {
			t.Errorf("key %q: expected ConfirmVisible=%v, got %v", key, expectConfirm, updated.state.ConfirmVisible)
		}
		// Reset for next
		updated.state.ConfirmVisible = false
		m = updated
	}

	// "x" should trigger stop confirm
	testKey("x", true)

	// "r" should trigger reboot confirm
	testKey("r", true)

	// "d" should trigger delete confirm
	testKey("d", true)

	// "S" (uppercase) should NOT trigger anything (we removed it)
	testKey("S", false)
	testKey("R", false)

	// "e" should trigger actionShell (doesn't set confirm, but we can verify it doesn't crash)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")}
	newModel, cmd := m.Update(msg)
	// cmd might be tea.ExecProcess, which is fine
	_ = newModel
	_ = cmd
}
