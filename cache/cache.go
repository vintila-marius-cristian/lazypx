// Package cache provides a TTL-based, goroutine-safe cluster metadata cache.
// It fans out concurrently to all nodes on refresh and merges results.
package cache

import (
	"context"
	"sync"
	"time"

	"lazypx/api"
)

// ClusterSnapshot holds the full cluster state at a point in time.
type ClusterSnapshot struct {
	Nodes      []api.NodeStatus
	VMs        map[string][]api.VMStatus         // node -> VMs
	Containers map[string][]api.CTStatus         // node -> Containers
	Storage    map[string][]api.StorageStatus    // node -> Storage
	Network    map[string][]api.NetworkInterface // node -> Network
	Tasks      []api.Task                        // global cluster tasks
	FetchedAt  time.Time
	Error      error
}

// IsEmpty returns true if no nodes have been loaded.
func (s *ClusterSnapshot) IsEmpty() bool {
	return len(s.Nodes) == 0
}

// AllVMs returns a flat list of all VMs across all nodes.
func (s *ClusterSnapshot) AllVMs() []api.VMStatus {
	var out []api.VMStatus
	for _, vms := range s.VMs {
		out = append(out, vms...)
	}
	return out
}

// AllContainers returns a flat list of all containers across all nodes.
func (s *ClusterSnapshot) AllContainers() []api.CTStatus {
	var out []api.CTStatus
	for _, cts := range s.Containers {
		out = append(out, cts...)
	}
	return out
}

// Cache manages the cluster snapshot with TTL-based refresh.
type Cache struct {
	mu       sync.RWMutex
	snapshot ClusterSnapshot
	ttl      time.Duration
	client   *api.Client
}

// New creates a new Cache with the given TTL.
func New(client *api.Client, ttl time.Duration) *Cache {
	return &Cache{
		client: client,
		ttl:    ttl,
	}
}

// Get returns the current cluster snapshot.
// If the snapshot is stale (older than TTL), it is refreshed first.
func (c *Cache) Get(ctx context.Context) ClusterSnapshot {
	c.mu.RLock()
	age := time.Since(c.snapshot.FetchedAt)
	snap := c.snapshot
	c.mu.RUnlock()

	if snap.IsEmpty() || age > c.ttl {
		return c.Refresh(ctx)
	}
	return snap
}

// Refresh fetches the full cluster state concurrently from all nodes.
func (c *Cache) Refresh(ctx context.Context) ClusterSnapshot {
	nodes, err := c.client.GetNodes(ctx)
	if err != nil {
		snap := ClusterSnapshot{Error: err, FetchedAt: time.Now()}
		c.mu.Lock()
		c.snapshot = snap
		c.mu.Unlock()
		return snap
	}

	// Fan out: fetch VMs, containers, storage from every node in parallel.
	type nodeResult struct {
		node       string
		vms        []api.VMStatus
		containers []api.CTStatus
		storage    []api.StorageStatus
		network    []api.NetworkInterface
	}

	results := make(chan nodeResult, len(nodes))
	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(node api.NodeStatus) {
			defer wg.Done()
			res := nodeResult{node: node.Node}
			// Fetch in parallel within each node too.
			var innerWg sync.WaitGroup
			innerWg.Add(4)
			go func() {
				defer innerWg.Done()
				res.vms, _ = c.client.GetVMs(ctx, node.Node)
			}()
			go func() {
				defer innerWg.Done()
				res.containers, _ = c.client.GetContainers(ctx, node.Node)
			}()
			go func() {
				defer innerWg.Done()
				res.storage, _ = c.client.GetStorage(ctx, node.Node)
			}()
			go func() {
				defer innerWg.Done()
				res.network, _ = c.client.GetNetworkInterfaces(ctx, node.Node)
			}()
			innerWg.Wait()
			results <- res
		}(n)
	}

	// IMPORTANT: globalTasks goroutine must be registered with wg BEFORE
	// the goroutine that calls wg.Wait()/close(results), otherwise
	// wg.Wait() can complete before wg.Add(1) runs → globalTasks is nil.
	var globalTasks []api.Task
	var tasksMu sync.Mutex
	wg.Add(1)
	go func() {
		defer wg.Done()
		if t, err := c.client.GetClusterTasks(ctx); err == nil {
			tasksMu.Lock()
			globalTasks = t
			tasksMu.Unlock()
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	snap := ClusterSnapshot{
		Nodes:      nodes,
		VMs:        make(map[string][]api.VMStatus),
		Containers: make(map[string][]api.CTStatus),
		Storage:    make(map[string][]api.StorageStatus),
		Network:    make(map[string][]api.NetworkInterface),
		FetchedAt:  time.Now(),
	}
	for res := range results {
		snap.VMs[res.node] = res.vms
		snap.Containers[res.node] = res.containers
		snap.Storage[res.node] = res.storage
		snap.Network[res.node] = res.network
	}

	// Safe to read globalTasks now — wg.Wait() completed (close(results) fired),
	// meaning the globalTasks goroutine has finished.
	tasksMu.Lock()
	snap.Tasks = globalTasks
	tasksMu.Unlock()

	c.mu.Lock()
	c.snapshot = snap
	c.mu.Unlock()
	return snap
}

// Invalidate marks the snapshot as stale, forcing the next Get to refresh.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	c.snapshot.FetchedAt = time.Time{}
	c.mu.Unlock()
}
