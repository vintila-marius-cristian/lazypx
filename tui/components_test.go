package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"lazypx/api"
	"lazypx/cache"
	"lazypx/sessions"
	"lazypx/state"
)

// ── StatusDot ──────────────────────────────────────────────────────────────

func TestStatusDot_Running(t *testing.T) {
	dot := StatusDot("running")
	if dot == "" {
		t.Fatal("StatusDot(running) returned empty")
	}
	if !strings.Contains(dot, "●") {
		t.Error("running dot should contain ●")
	}
}

func TestStatusDot_Stopped(t *testing.T) {
	dot := StatusDot("stopped")
	if dot == "" {
		t.Fatal("StatusDot(stopped) returned empty")
	}
	if !strings.Contains(dot, "○") {
		t.Error("stopped dot should contain ○")
	}
}

func TestStatusDot_Suspended(t *testing.T) {
	dot := StatusDot("suspended")
	if dot == "" {
		t.Fatal("StatusDot(suspended) returned empty")
	}
	if !strings.Contains(dot, "◐") {
		t.Error("suspended dot should contain ◐")
	}
}

func TestStatusDot_Unknown(t *testing.T) {
	dot := StatusDot("unknown")
	if dot == "" {
		t.Fatal("StatusDot(unknown) returned empty")
	}
	if !strings.Contains(dot, "⊘") {
		t.Error("unknown dot should contain ⊘")
	}
}

func TestStatusDot_EmptyString(t *testing.T) {
	dot := StatusDot("")
	if !strings.Contains(dot, "⊘") {
		t.Error("empty status should return the default/error dot")
	}
}

// ── StatusStyle ────────────────────────────────────────────────────────────

func TestStatusStyle_Running(t *testing.T) {
	s := StatusStyle("running")
	if s.GetForeground() != StyleRunning.GetForeground() {
		t.Error("running should use StyleRunning")
	}
}

func TestStatusStyle_Stopped(t *testing.T) {
	s := StatusStyle("stopped")
	if s.GetForeground() != StyleStopped.GetForeground() {
		t.Error("stopped should use StyleStopped")
	}
}

func TestStatusStyle_Suspended(t *testing.T) {
	s := StatusStyle("suspended")
	if s.GetForeground() != StyleSuspended.GetForeground() {
		t.Error("suspended should use StyleSuspended")
	}
}

func TestStatusStyle_Unknown(t *testing.T) {
	s := StatusStyle("bogus")
	if s.GetForeground() != StyleError.GetForeground() {
		t.Error("unknown status should use StyleError")
	}
}

// ── truncate ───────────────────────────────────────────────────────────────

func TestTruncate_NoTruncation(t *testing.T) {
	result := truncate("hello", 10)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestTruncate_ExactFit(t *testing.T) {
	result := truncate("hello", 5)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestTruncate_NeedsTruncation(t *testing.T) {
	result := truncate("hello world", 6)
	if !strings.HasSuffix(result, "…") {
		t.Errorf("expected ellipsis suffix, got %q", result)
	}
	if len([]rune(result)) != 6 {
		t.Errorf("expected 6 runes, got %d", len([]rune(result)))
	}
}

func TestTruncate_MaxWOne(t *testing.T) {
	result := truncate("hello", 1)
	if result != "…" {
		t.Errorf("expected '…', got %q", result)
	}
}

func TestTruncate_MaxWZero(t *testing.T) {
	result := truncate("hello", 0)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestTruncate_MaxWNegative(t *testing.T) {
	result := truncate("hello", -1)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestTruncate_EmptyString(t *testing.T) {
	result := truncate("", 5)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestTruncate_Unicode(t *testing.T) {
	result := truncate("日本語テスト", 4)
	if len([]rune(result)) != 4 {
		t.Errorf("expected 4 runes, got %d", len([]rune(result)))
	}
	if !strings.HasSuffix(result, "…") {
		t.Error("expected ellipsis for truncated unicode")
	}
}

// ── SidebarModel ──────────────────────────────────────────────────────────

func TestNewSidebarModel(t *testing.T) {
	st := state.New("test", false)
	m := NewSidebarModel(st)
	if m.st != st {
		t.Error("SidebarModel should hold state reference")
	}
	if m.nodesList == nil {
		t.Error("nodesList should not be nil")
	}
	if m.vmsList == nil {
		t.Error("vmsList should not be nil")
	}
	if m.ctsList == nil {
		t.Error("ctsList should not be nil")
	}
	if m.storageList == nil {
		t.Error("storageList should not be nil")
	}
}

func TestSidebarModel_ActiveList_Nodes(t *testing.T) {
	st := state.New("test", false)
	st.FocusedPanel = state.PanelNodes
	m := NewSidebarModel(st)
	if m.ActiveList() != m.nodesList {
		t.Error("ActiveList should return nodesList when PanelNodes focused")
	}
}

func TestSidebarModel_ActiveList_VMs(t *testing.T) {
	st := state.New("test", false)
	st.FocusedPanel = state.PanelVMs
	m := NewSidebarModel(st)
	if m.ActiveList() != m.vmsList {
		t.Error("ActiveList should return vmsList when PanelVMs focused")
	}
}

func TestSidebarModel_ActiveList_CTs(t *testing.T) {
	st := state.New("test", false)
	st.FocusedPanel = state.PanelCTs
	m := NewSidebarModel(st)
	if m.ActiveList() != m.ctsList {
		t.Error("ActiveList should return ctsList when PanelCTs focused")
	}
}

func TestSidebarModel_ActiveList_Storage(t *testing.T) {
	st := state.New("test", false)
	st.FocusedPanel = state.PanelStorage
	m := NewSidebarModel(st)
	if m.ActiveList() != m.storageList {
		t.Error("ActiveList should return storageList when PanelStorage focused")
	}
}

func TestSidebarModel_ActiveList_Default(t *testing.T) {
	st := state.New("test", false)
	st.FocusedPanel = state.PanelDetail
	m := NewSidebarModel(st)
	if m.ActiveList() != m.nodesList {
		t.Error("ActiveList should return nodesList when non-sidebar panel focused")
	}
}

func TestSidebarModel_Sync_PopulatesNodes(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes: []api.NodeStatus{
			{Node: "pve1", Status: "online"},
			{Node: "pve2", Status: "online"},
		},
		VMs:        make(map[string][]api.VMStatus),
		Containers: make(map[string][]api.CTStatus),
		Storage:    make(map[string][]api.StorageStatus),
		FetchedAt:  time.Now(),
	}
	m := NewSidebarModel(st)
	m.Sync(st)
	if len(m.nodesList.Items) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(m.nodesList.Items))
	}
}

func TestSidebarModel_Sync_PopulatesVMsForSelectedNode(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes: []api.NodeStatus{
			{Node: "pve1", Status: "online"},
		},
		VMs: map[string][]api.VMStatus{
			"pve1": {
				{VMID: 100, Name: "vm1", Status: "running", Node: "pve1"},
				{VMID: 101, Name: "vm2", Status: "stopped", Node: "pve1"},
			},
		},
		Containers: make(map[string][]api.CTStatus),
		Storage:    make(map[string][]api.StorageStatus),
		FetchedAt:  time.Now(),
	}
	m := NewSidebarModel(st)
	m.Sync(st)
	if len(m.vmsList.Items) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(m.vmsList.Items))
	}
}

func TestSidebarModel_Sync_EmptySnapshot(t *testing.T) {
	st := state.New("test", false)
	m := NewSidebarModel(st)
	m.Sync(st)
	if len(m.nodesList.Items) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(m.nodesList.Items))
	}
	if len(m.vmsList.Items) != 0 {
		t.Errorf("expected 0 VMs, got %d", len(m.vmsList.Items))
	}
}

func TestSidebarModel_ApplyFilter(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes: []api.NodeStatus{
			{Node: "pve1", Status: "online"},
			{Node: "pve2", Status: "online"},
			{Node: "storage1", Status: "online"},
		},
		VMs:        make(map[string][]api.VMStatus),
		Containers: make(map[string][]api.CTStatus),
		Storage:    make(map[string][]api.StorageStatus),
		FetchedAt:  time.Now(),
	}
	m := NewSidebarModel(st)
	m.Sync(st)

	m.ApplyFilter("pve", st)
	if len(m.nodesList.Items) != 2 {
		t.Errorf("expected 2 nodes matching 'pve', got %d", len(m.nodesList.Items))
	}
}

func TestSidebarModel_ApplyFilter_CaseInsensitive(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes: []api.NodeStatus{
			{Node: "PVE1", Status: "online"},
			{Node: "pve2", Status: "online"},
		},
		VMs:        make(map[string][]api.VMStatus),
		Containers: make(map[string][]api.CTStatus),
		Storage:    make(map[string][]api.StorageStatus),
		FetchedAt:  time.Now(),
	}
	m := NewSidebarModel(st)
	m.Sync(st)

	m.ApplyFilter("pve", st)
	if len(m.nodesList.Items) != 2 {
		t.Errorf("expected 2 nodes matching 'pve' case-insensitive, got %d", len(m.nodesList.Items))
	}
}

func TestSidebarModel_ApplyFilter_EmptyQuery(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes: []api.NodeStatus{
			{Node: "pve1", Status: "online"},
			{Node: "pve2", Status: "online"},
		},
		VMs:        make(map[string][]api.VMStatus),
		Containers: make(map[string][]api.CTStatus),
		Storage:    make(map[string][]api.StorageStatus),
		FetchedAt:  time.Now(),
	}
	m := NewSidebarModel(st)
	m.Sync(st)

	m.ApplyFilter("", st)
	if len(m.nodesList.Items) != 2 {
		t.Errorf("empty query should keep all items, got %d", len(m.nodesList.Items))
	}
}

func TestSidebarModel_ApplyFilter_NoMatch(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes: []api.NodeStatus{
			{Node: "pve1", Status: "online"},
		},
		VMs:        make(map[string][]api.VMStatus),
		Containers: make(map[string][]api.CTStatus),
		Storage:    make(map[string][]api.StorageStatus),
		FetchedAt:  time.Now(),
	}
	m := NewSidebarModel(st)
	m.Sync(st)

	m.ApplyFilter("zzzzz", st)
	if len(m.nodesList.Items) != 0 {
		t.Errorf("expected 0 nodes for non-matching query, got %d", len(m.nodesList.Items))
	}
}

// ── DetailModel ───────────────────────────────────────────────────────────

func TestNewDetailModel(t *testing.T) {
	st := state.New("test", false)
	m := NewDetailModel(st)
	if m.st != st {
		t.Error("DetailModel should hold state reference")
	}
}

func TestDetailModel_SetSize(t *testing.T) {
	st := state.New("test", false)
	m := NewDetailModel(st)
	m.SetSize(80, 40)
	if m.width != 80 || m.height != 40 {
		t.Errorf("expected 80x40, got %dx%d", m.width, m.height)
	}
}

func TestDetailModel_Sync(t *testing.T) {
	st1 := state.New("test", false)
	st2 := state.New("test2", false)
	m := NewDetailModel(st1)
	m.Sync(st2)
	if m.st != st2 {
		t.Error("Sync should update state reference")
	}
}

func TestDetailModel_View_TooSmall(t *testing.T) {
	st := state.New("test", false)
	m := NewDetailModel(st)
	m.SetSize(1, 1)
	v := m.View(false)
	if v != "" {
		t.Error("View should return empty for size < 2")
	}
}

func TestDetailModel_View_EmptyState(t *testing.T) {
	st := state.New("test", false)
	m := NewDetailModel(st)
	m.SetSize(80, 40)
	v := m.View(false)
	if v == "" {
		t.Fatal("View should not be empty for valid size")
	}
	if !strings.Contains(v, "connecting") {
		t.Error("empty state should show connecting message")
	}
}

func TestDetailModel_View_Focused(t *testing.T) {
	st := state.New("test", false)
	m := NewDetailModel(st)
	m.SetSize(80, 40)
	v := m.View(true)
	if v == "" {
		t.Fatal("View should not be empty when focused")
	}
}

func TestDetailModel_View_WithVM(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes:     []api.NodeStatus{{Node: "pve1"}},
		FetchedAt: time.Now(),
	}
	st.Selected = state.Selection{
		Kind: state.KindVM,
		VMStatus: &api.VMStatus{
			VMID:     100,
			Name:     "test-vm",
			Node:     "pve1",
			Status:   "running",
			CPU:      0.05,
			MaxCPU:   2,
			MemUsed:  1073741824,
			MemTotal: 4294967296,
			MaxDisk:  34359738368,
		},
	}
	m := NewDetailModel(st)
	m.SetSize(80, 40)
	v := m.View(false)
	if !strings.Contains(v, "VM 100") {
		t.Error("View should contain VM ID")
	}
	if !strings.Contains(v, "test-vm") {
		t.Error("View should contain VM name")
	}
	if !strings.Contains(v, "running") {
		t.Error("View should contain status")
	}
}

func TestDetailModel_View_WithNode(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Nodes:     []api.NodeStatus{{Node: "pve1"}},
		FetchedAt: time.Now(),
	}
	st.Selected = state.Selection{
		Kind:     state.KindNode,
		NodeName: "pve1",
		NodeStatus: &api.NodeStatus{
			Node:      "pve1",
			Status:    "online",
			CPUUsage:  0.25,
			MaxCPU:    8,
			MemUsed:   8589934592,
			MemTotal:  17179869184,
			DiskUsed:  53687091200,
			DiskTotal: 107374182400,
		},
	}
	m := NewDetailModel(st)
	m.SetSize(80, 40)
	v := m.View(false)
	if !strings.Contains(v, "pve1") {
		t.Error("View should contain node name")
	}
}

// ── TasksModel ────────────────────────────────────────────────────────────

func TestNewTasksModel(t *testing.T) {
	st := state.New("test", false)
	m := NewTasksModel(st)
	if m.st != st {
		t.Error("TasksModel should hold state reference")
	}
}

func TestTasksModel_SetSize(t *testing.T) {
	st := state.New("test", false)
	m := NewTasksModel(st)
	m.SetSize(100, 10)
	if m.width != 100 || m.height != 10 {
		t.Errorf("expected 100x10, got %dx%d", m.width, m.height)
	}
}

func TestTasksModel_View_TooSmall(t *testing.T) {
	st := state.New("test", false)
	m := NewTasksModel(st)
	v := m.View(1, 1, false)
	if v != "" {
		t.Error("View should return empty for size < 2")
	}
}

func TestTasksModel_View_EmptyTasks(t *testing.T) {
	st := state.New("test", false)
	m := NewTasksModel(st)
	v := m.View(80, 10, false)
	if v == "" {
		t.Fatal("View should not be empty")
	}
	if !strings.Contains(v, "No tasks yet") {
		t.Error("View should show 'No tasks yet' when empty")
	}
}

func TestTasksModel_View_WithLocalEvents(t *testing.T) {
	st := state.New("test", false)
	st.AddLocalEvent("shell opened", "info")
	m := NewTasksModel(st)
	v := m.View(80, 10, false)
	if !strings.Contains(v, "shell opened") {
		t.Error("View should contain local event label")
	}
}

func TestTasksModel_View_WithActiveTasks(t *testing.T) {
	st := state.New("test", false)
	st.AddTask("UPID1", "pve1", "Starting VM 100")
	m := NewTasksModel(st)
	v := m.View(80, 10, false)
	if !strings.Contains(v, "Starting VM 100") {
		t.Error("View should contain active task label")
	}
}

func TestTasksModel_View_WithClusterTasks(t *testing.T) {
	st := state.New("test", false)
	st.Snapshot = cache.ClusterSnapshot{
		Tasks: []api.Task{
			{
				UPID:   "UPID:pve1:00001",
				Node:   "pve1",
				Type:   "vzdump",
				ID:     "vm/100",
				User:   "root@pam",
				Status: "stopped",
			},
		},
		FetchedAt: time.Now(),
	}
	m := NewTasksModel(st)
	v := m.View(80, 10, false)
	if !strings.Contains(v, "vzdump") {
		t.Error("View should contain task type")
	}
}

func TestTasksModel_View_Focused(t *testing.T) {
	st := state.New("test", false)
	m := NewTasksModel(st)
	v := m.View(80, 10, true)
	if v == "" {
		t.Fatal("View should not be empty when focused")
	}
}

// ── SnapshotsModel ────────────────────────────────────────────────────────

func TestNewSnapshotsModel(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewSnapshotsModel(st, client)
	if m.st != st {
		t.Error("SnapshotsModel should hold state reference")
	}
	if m.client != client {
		t.Error("SnapshotsModel should hold client reference")
	}
}

func TestSnapshotsModel_Init(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewSnapshotsModel(st, client)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil cmd")
	}
}

func TestSnapshotsModel_View_Loading(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewSnapshotsModel(st, client)
	m.loading = true
	m.width = 80
	v := m.View()
	if !strings.Contains(v, "Loading snapshots") {
		t.Error("View should show loading message")
	}
}

func TestSnapshotsModel_View_Error(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewSnapshotsModel(st, client)
	m.err = fmt.Errorf("connection refused")
	m.width = 80
	v := m.View()
	if !strings.Contains(v, "Error loading snapshots") {
		t.Error("View should show error message")
	}
	if !strings.Contains(v, "connection refused") {
		t.Error("View should contain the error text")
	}
}

func TestSnapshotsModel_View_Empty(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewSnapshotsModel(st, client)
	m.width = 80
	v := m.View()
	if !strings.Contains(v, "No snapshots found") {
		t.Error("View should show empty message")
	}
}

func TestSnapshotsModel_View_WithSnapshots(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewSnapshotsModel(st, client)
	m.width = 80
	m.snapshots = []api.Snapshot{
		{Name: "current", Description: "running state"},
		{Name: "backup-01", Description: "before update"},
	}
	v := m.View()
	if !strings.Contains(v, "backup-01") {
		t.Error("View should contain snapshot name")
	}
	if !strings.Contains(v, "before update") {
		t.Error("View should contain snapshot description")
	}
}

// ── BackupsModel ──────────────────────────────────────────────────────────

func TestNewBackupsModel(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewBackupsModel(st, client)
	if m.st != st {
		t.Error("BackupsModel should hold state reference")
	}
	if m.client != client {
		t.Error("BackupsModel should hold client reference")
	}
}

func TestBackupsModel_Init(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewBackupsModel(st, client)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil cmd")
	}
}

func TestBackupsModel_View_Loading(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewBackupsModel(st, client)
	m.loading = true
	m.width = 80
	v := m.View()
	if !strings.Contains(v, "Loading backups") {
		t.Error("View should show loading message")
	}
}

func TestBackupsModel_View_Error(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewBackupsModel(st, client)
	m.err = fmt.Errorf("timeout")
	m.width = 80
	v := m.View()
	if !strings.Contains(v, "Error loading backups") {
		t.Error("View should show error message")
	}
	if !strings.Contains(v, "timeout") {
		t.Error("View should contain the error text")
	}
}

func TestBackupsModel_View_Empty(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewBackupsModel(st, client)
	m.width = 80
	v := m.View()
	if !strings.Contains(v, "No backups found") {
		t.Error("View should show empty message")
	}
}

func TestBackupsModel_View_WithBackups(t *testing.T) {
	st := state.New("test", false)
	client := api.NewClient("http://localhost", "user", "pass", true)
	m := NewBackupsModel(st, client)
	m.width = 80
	m.backups = []api.BackupVolume{
		{
			VolID:  "local:backup/vzdump-qemu-100-2024.vma.zst",
			Format: "vma.zst",
			Size:   1073741824,
			CTime:  1700000000,
		},
	}
	v := m.View()
	if !strings.Contains(v, "vzdump-qemu-100") {
		t.Error("View should contain backup volume ID")
	}
	if !strings.Contains(v, "MB") {
		t.Error("View should contain size in MB")
	}
}

// ── SessionsModel ─────────────────────────────────────────────────────────

func TestNewSessionsModel(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	if m.st != st {
		t.Error("SessionsModel should hold state reference")
	}
	if m.manager != mgr {
		t.Error("SessionsModel should hold manager reference")
	}
}

func TestSessionsModel_MoveUp_EmptyList(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.MoveUp()
	if m.cursor != 0 {
		t.Errorf("cursor should stay 0 on empty list, got %d", m.cursor)
	}
}

func TestSessionsModel_MoveDown_EmptyList(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.MoveDown()
	if m.cursor != 0 {
		t.Errorf("cursor should stay 0 on empty list, got %d", m.cursor)
	}
}

func TestSessionsModel_Selected_EmptyList(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	sel := m.Selected()
	if sel != nil {
		t.Error("Selected should return nil on empty list")
	}
}

func TestSessionsModel_Selected_WithSessions(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.sessions = []sessions.SessionInfo{
		{Key: "s1", VMID: 100, Status: "running", StartedAt: time.Now()},
		{Key: "s2", VMID: 101, Status: "running", StartedAt: time.Now()},
	}
	sel := m.Selected()
	if sel == nil {
		t.Fatal("Selected should not return nil")
	}
	if sel.Key != "s1" {
		t.Errorf("expected key 's1', got %q", sel.Key)
	}
}

func TestSessionsModel_MoveDown_Wrap(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.sessions = []sessions.SessionInfo{
		{Key: "s1", VMID: 100},
		{Key: "s2", VMID: 101},
		{Key: "s3", VMID: 102},
	}
	m.cursor = 0
	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}
	m.MoveDown()
	if m.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", m.cursor)
	}
	m.MoveDown() // should wrap
	if m.cursor != 0 {
		t.Errorf("expected cursor to wrap to 0, got %d", m.cursor)
	}
}

func TestSessionsModel_MoveUp_Wrap(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.sessions = []sessions.SessionInfo{
		{Key: "s1", VMID: 100},
		{Key: "s2", VMID: 101},
	}
	m.cursor = 0
	m.MoveUp() // should wrap to bottom
	if m.cursor != 1 {
		t.Errorf("expected cursor to wrap to 1, got %d", m.cursor)
	}
}

func TestSessionsModel_Selected_OutOfBounds(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.sessions = []sessions.SessionInfo{
		{Key: "s1", VMID: 100},
	}
	m.cursor = 99
	sel := m.Selected()
	if sel != nil {
		t.Error("Selected should return nil when cursor is out of bounds")
	}
}

func TestSessionsModel_View_Empty(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	v := m.View(80, 40)
	if !strings.Contains(v, "No active sessions") {
		t.Error("View should show 'No active sessions' when empty")
	}
}

func TestSessionsModel_View_WithSessions(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.sessions = []sessions.SessionInfo{
		{Key: "lazypx-default-100", VMID: 100, Status: "running", StartedAt: time.Now()},
	}
	v := m.View(80, 40)
	if !strings.Contains(v, "100") {
		t.Error("View should contain VMID")
	}
	if !strings.Contains(v, "attach") {
		t.Error("View should contain help text")
	}
}
