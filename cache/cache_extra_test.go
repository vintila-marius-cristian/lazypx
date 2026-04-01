package cache_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lazypx/api"
	"lazypx/cache"
)

func newErrorMockAPIClient(t *testing.T, nodesStatus int) *api.Client {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			w.WriteHeader(nodesStatus)
			fmt.Fprint(w, `{"errors": "unauthorized"}`)
			return
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	t.Cleanup(srv.Close)
	return api.NewClient(srv.URL, "root@pam!test", "secret", true)
}

func TestIsEmpty(t *testing.T) {
	t.Run("empty snapshot", func(t *testing.T) {
		snap := cache.ClusterSnapshot{}
		if !snap.IsEmpty() {
			t.Error("expected empty snapshot to be IsEmpty() == true")
		}
	})

	t.Run("zero-value snapshot", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			Nodes:     nil,
			VMs:       make(map[string][]api.VMStatus),
			FetchedAt: time.Now(),
		}
		if !snap.IsEmpty() {
			t.Error("expected snapshot with nil Nodes to be empty")
		}
	})

	t.Run("non-empty snapshot", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			Nodes: []api.NodeStatus{{Node: "pve1", Status: "online"}},
		}
		if snap.IsEmpty() {
			t.Error("expected snapshot with one node to not be empty")
		}
	})
}

func TestAllVMs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		snap := cache.ClusterSnapshot{VMs: make(map[string][]api.VMStatus)}
		if got := snap.AllVMs(); len(got) != 0 {
			t.Errorf("expected empty, got %d VMs", len(got))
		}
	})

	t.Run("nil map", func(t *testing.T) {
		snap := cache.ClusterSnapshot{}
		if got := snap.AllVMs(); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("single node", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			VMs: map[string][]api.VMStatus{
				"pve1": {
					{VMID: 100, Name: "web", Node: "pve1"},
					{VMID: 101, Name: "db", Node: "pve1"},
				},
			},
		}
		got := snap.AllVMs()
		if len(got) != 2 {
			t.Fatalf("expected 2 VMs, got %d", len(got))
		}
		ids := map[int]bool{}
		for _, vm := range got {
			ids[vm.VMID] = true
		}
		if !ids[100] || !ids[101] {
			t.Errorf("missing expected VMIDs: %v", ids)
		}
	})

	t.Run("multiple nodes", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			VMs: map[string][]api.VMStatus{
				"pve1": {
					{VMID: 100, Name: "web", Node: "pve1"},
				},
				"pve2": {
					{VMID: 200, Name: "api", Node: "pve2"},
					{VMID: 201, Name: "cache", Node: "pve2"},
				},
			},
		}
		got := snap.AllVMs()
		if len(got) != 3 {
			t.Errorf("expected 3 VMs, got %d", len(got))
		}
	})

	t.Run("node with empty slice", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			VMs: map[string][]api.VMStatus{
				"pve1": nil,
				"pve2": {{VMID: 100, Name: "vm100"}},
			},
		}
		got := snap.AllVMs()
		if len(got) != 1 {
			t.Errorf("expected 1 VM, got %d", len(got))
		}
	})
}

func TestAllContainers(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		snap := cache.ClusterSnapshot{Containers: make(map[string][]api.CTStatus)}
		if got := snap.AllContainers(); len(got) != 0 {
			t.Errorf("expected empty, got %d containers", len(got))
		}
	})

	t.Run("nil map", func(t *testing.T) {
		snap := cache.ClusterSnapshot{}
		if got := snap.AllContainers(); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("single node", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			Containers: map[string][]api.CTStatus{
				"pve1": {
					{VMID: 110, Name: "ct1", Node: "pve1"},
					{VMID: 111, Name: "ct2", Node: "pve1"},
				},
			},
		}
		got := snap.AllContainers()
		if len(got) != 2 {
			t.Fatalf("expected 2 containers, got %d", len(got))
		}
	})

	t.Run("multiple nodes", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			Containers: map[string][]api.CTStatus{
				"pve1": {
					{VMID: 110, Name: "lb", Node: "pve1"},
				},
				"pve2": {
					{VMID: 210, Name: "monitor", Node: "pve2"},
				},
				"pve3": {
					{VMID: 310, Name: "log", Node: "pve3"},
					{VMID: 311, Name: "backup", Node: "pve3"},
				},
			},
		}
		got := snap.AllContainers()
		if len(got) != 4 {
			t.Errorf("expected 4 containers, got %d", len(got))
		}
	})
}

func TestSnapshotWithErrors(t *testing.T) {
	snap := cache.ClusterSnapshot{
		Nodes: []api.NodeStatus{
			{Node: "pve1", Status: "online"},
			{Node: "pve2", Status: "offline"},
		},
		VMs: map[string][]api.VMStatus{
			"pve1": {{VMID: 100, Name: "web", Node: "pve1"}},
		},
		Containers: map[string][]api.CTStatus{},
		Storage:    map[string][]api.StorageStatus{},
		Network:    map[string][]api.NetworkInterface{},
		Errors:     []string{"node pve2 vms: connection refused", "node pve2 storage: timeout"},
		FetchedAt:  time.Now(),
	}

	if snap.IsEmpty() {
		t.Error("snapshot with nodes should not be empty")
	}
	if len(snap.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(snap.Errors))
	}
	if len(snap.AllVMs()) != 1 {
		t.Errorf("expected 1 VM from healthy node, got %d", len(snap.AllVMs()))
	}
}

func TestSnapshotEdgeCases(t *testing.T) {
	t.Run("all maps empty", func(t *testing.T) {
		snap := cache.ClusterSnapshot{
			Nodes:      []api.NodeStatus{{Node: "pve1"}},
			VMs:        make(map[string][]api.VMStatus),
			Containers: make(map[string][]api.CTStatus),
			Storage:    make(map[string][]api.StorageStatus),
			Network:    make(map[string][]api.NetworkInterface),
			Tasks:      []api.Task{},
			FetchedAt:  time.Now(),
		}

		if snap.IsEmpty() {
			t.Error("snapshot with one node should not be empty")
		}
		if got := snap.AllVMs(); len(got) != 0 {
			t.Errorf("expected 0 VMs, got %d", len(got))
		}
		if got := snap.AllContainers(); len(got) != 0 {
			t.Errorf("expected 0 containers, got %d", len(got))
		}
	})

	t.Run("empty Errors slice", func(t *testing.T) {
		snap := cache.ClusterSnapshot{Errors: []string{}}
		if len(snap.Errors) != 0 {
			t.Errorf("expected 0 errors, got %d", len(snap.Errors))
		}
	})

	t.Run("nil Errors", func(t *testing.T) {
		snap := cache.ClusterSnapshot{}
		if snap.Errors != nil {
			t.Errorf("expected nil errors, got %v", snap.Errors)
		}
	})
}

func TestNew(t *testing.T) {
	c := newMockAPIClient(t)
	ca := cache.New(c, 5*time.Second)
	// New returns a non-nil cache; initial snapshot is zero-valued (stale)
	snap := ca.Get(t.Context())
	if snap.Error != nil {
		t.Fatalf("unexpected error from fresh cache: %v", snap.Error)
	}
}

func TestGetReturnsCachedSnapshot(t *testing.T) {
	c := newMockAPIClient(t)
	ca := cache.New(c, 5*time.Second)

	// First Get populates cache
	snap1 := ca.Get(t.Context())
	if snap1.Error != nil {
		t.Fatalf("first get: %v", snap1.Error)
	}

	// Second Get should return cached snapshot (within TTL, not re-fetch)
	snap2 := ca.Get(t.Context())
	if snap2.Error != nil {
		t.Fatalf("second get: %v", snap2.Error)
	}
	if !snap2.FetchedAt.Equal(snap1.FetchedAt) {
		t.Error("expected second Get to return cached snapshot with same FetchedAt")
	}
}

func TestRefreshGetNodesError(t *testing.T) {
	c := newErrorMockAPIClient(t, 401)
	ca := cache.New(c, 1*time.Second)

	ctx := context.Background()
	snap := ca.Refresh(ctx)
	if snap.Error == nil {
		t.Fatal("expected error from Refresh when GetNodes fails, got nil")
	}
}

func TestGetRefreshesOnGetNodesError(t *testing.T) {
	c := newErrorMockAPIClient(t, 500)
	ca := cache.New(c, 1*time.Second)

	ctx := context.Background()
	snap := ca.Get(ctx)
	if snap.Error == nil {
		t.Fatal("expected error from Get when API returns 500 for nodes")
	}
}
