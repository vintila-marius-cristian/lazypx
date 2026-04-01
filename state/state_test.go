package state_test

import (
	"testing"
	"time"

	"lazypx/api"
	"lazypx/cache"
	"lazypx/state"
)

func TestNew(t *testing.T) {
	s := state.New("prod", true)
	if s.ProfileName != "prod" {
		t.Fatalf("ProfileName = %q, want %q", s.ProfileName, "prod")
	}
	if !s.Production {
		t.Fatal("Production should be true")
	}
	if s.FocusedPanel != state.PanelNodes {
		t.Fatalf("FocusedPanel = %d, want %d", s.FocusedPanel, state.PanelNodes)
	}
}

func TestAddTask(t *testing.T) {
	s := state.New("test", false)
	idx := s.AddTask("UPID:node:0", "node1", "Starting VM 100")
	if idx != 0 {
		t.Fatalf("first AddTask index = %d, want 0", idx)
	}
	if len(s.ActiveTasks) != 1 {
		t.Fatalf("ActiveTasks len = %d, want 1", len(s.ActiveTasks))
	}
	if s.ActiveTasks[0].UPID != "UPID:node:0" {
		t.Errorf("UPID = %q", s.ActiveTasks[0].UPID)
	}
	if s.ActiveTasks[0].Node != "node1" {
		t.Errorf("Node = %q", s.ActiveTasks[0].Node)
	}
	if s.ActiveTasks[0].Label != "Starting VM 100" {
		t.Errorf("Label = %q", s.ActiveTasks[0].Label)
	}

	idx2 := s.AddTask("UPID2", "node2", "Stopping CT 200")
	if idx2 != 1 {
		t.Fatalf("second AddTask index = %d, want 1", idx2)
	}
}

func TestAppendTaskLog(t *testing.T) {
	s := state.New("test", false)
	s.AddTask("UPID1", "node1", "Task 1")
	s.AddTask("UPID2", "node2", "Task 2")

	s.AppendTaskLog(0, "line 1")
	s.AppendTaskLog(0, "line 2")
	s.AppendTaskLog(1, "other line")

	if len(s.ActiveTasks[0].Logs) != 2 {
		t.Fatalf("task 0 logs = %d, want 2", len(s.ActiveTasks[0].Logs))
	}
	if s.ActiveTasks[0].Logs[0] != "line 1" {
		t.Errorf("task 0 log[0] = %q", s.ActiveTasks[0].Logs[0])
	}
	if len(s.ActiveTasks[1].Logs) != 1 {
		t.Fatalf("task 1 logs = %d, want 1", len(s.ActiveTasks[1].Logs))
	}
}

func TestAppendTaskLog_OutOfBounds(t *testing.T) {
	s := state.New("test", false)
	// Should not panic
	s.AppendTaskLog(99, "line")
	s.AppendTaskLog(-1, "line")
	if len(s.ActiveTasks) != 0 {
		t.Fatal("should not create tasks")
	}
}

func TestMarkTaskDone(t *testing.T) {
	s := state.New("test", false)
	s.AddTask("UPID1", "node1", "Task 1")

	s.MarkTaskDone(0, true)
	if !s.ActiveTasks[0].Done {
		t.Error("task should be Done")
	}
	if !s.ActiveTasks[0].Success {
		t.Error("task should be Success")
	}

	s.MarkTaskDone(0, false)
	if s.ActiveTasks[0].Success {
		t.Error("task should not be Success after marking false")
	}
}

func TestMarkTaskDone_OutOfBounds(t *testing.T) {
	s := state.New("test", false)
	// Should not panic
	s.MarkTaskDone(99, true)
	s.MarkTaskDone(-1, true)
}

func TestAddLocalEvent(t *testing.T) {
	s := state.New("test", false)
	s.AddLocalEvent("shell opened", "info")
	s.AddLocalEvent("warning", "warn")

	if len(s.LocalEvents) != 2 {
		t.Fatalf("LocalEvents len = %d, want 2", len(s.LocalEvents))
	}
	if s.LocalEvents[0].Label != "shell opened" {
		t.Errorf("event[0].Label = %q", s.LocalEvents[0].Label)
	}
	if s.LocalEvents[0].Level != "info" {
		t.Errorf("event[0].Level = %q", s.LocalEvents[0].Level)
	}
	if s.LocalEvents[1].Level != "warn" {
		t.Errorf("event[1].Level = %q", s.LocalEvents[1].Level)
	}
	if s.LocalEvents[0].At.IsZero() {
		t.Error("event[0].At should be set")
	}
}

func TestAddLocalEvent_KeepsFifty(t *testing.T) {
	s := state.New("test", false)
	for i := 0; i < 60; i++ {
		s.AddLocalEvent("event", "info")
	}
	if len(s.LocalEvents) != 50 {
		t.Fatalf("LocalEvents len = %d, want 50", len(s.LocalEvents))
	}
}

func TestPanelTypeConstants(t *testing.T) {
	// Verify they are distinct
	panels := []state.PanelType{
		state.PanelNodes, state.PanelVMs, state.PanelCTs,
		state.PanelStorage, state.PanelDetail, state.PanelTasks,
	}
	seen := map[state.PanelType]bool{}
	for _, p := range panels {
		if seen[p] {
			t.Errorf("duplicate PanelType value: %d", p)
		}
		seen[p] = true
	}
}

func TestResourceKindConstants(t *testing.T) {
	kinds := []state.ResourceKind{
		state.KindNone, state.KindNode, state.KindVM,
		state.KindContainer, state.KindStorage,
	}
	seen := map[state.ResourceKind]bool{}
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("duplicate ResourceKind value: %d", k)
		}
		seen[k] = true
	}
}

func TestSelectionStruct(t *testing.T) {
	sel := state.Selection{
		Kind:     state.KindVM,
		NodeName: "pve1",
		VMID:     100,
		VMStatus: &api.VMStatus{VMID: 100, Name: "test"},
	}
	if sel.Kind != state.KindVM {
		t.Errorf("Kind = %d", sel.Kind)
	}
	if sel.VMStatus == nil || sel.VMStatus.Name != "test" {
		t.Error("VMStatus not set correctly")
	}
}

func TestAppState_SnapshotField(t *testing.T) {
	s := state.New("test", false)
	if !s.Snapshot.IsEmpty() {
		t.Error("initial snapshot should be empty")
	}
	s.Snapshot = cache.ClusterSnapshot{
		Nodes:     []api.NodeStatus{{Node: "pve1"}},
		FetchedAt: time.Now(),
	}
	if s.Snapshot.IsEmpty() {
		t.Error("snapshot should not be empty after assignment")
	}
}
