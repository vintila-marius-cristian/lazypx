package api

import (
	"context"
	"fmt"
)

// ── Cluster ──────────────────────────────────────────────────────────────────

// ClusterMember is an entry in /cluster/status.
type ClusterMember struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`   // node | cluster
	Online  int    `json:"online"` // 1=online
	Local   int    `json:"local"`
	NodeID  int    `json:"nodeid"`
	IP      string `json:"ip"`
	Level   string `json:"level"`
	Quorate int    `json:"quorate,omitempty"`
}

// ClusterResource is an entry in /cluster/resources.
type ClusterResource struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"` // vm|lxc|storage|node|pool|sdn
	Node     string  `json:"node"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	VMID     int     `json:"vmid,omitempty"`
	CPU      float64 `json:"cpu,omitempty"`
	MaxCPU   int     `json:"maxcpu,omitempty"`
	Mem      int64   `json:"mem,omitempty"`
	MaxMem   int64   `json:"maxmem,omitempty"`
	Disk     int64   `json:"disk,omitempty"`
	MaxDisk  int64   `json:"maxdisk,omitempty"`
	Uptime   int64   `json:"uptime,omitempty"`
	HAState  string  `json:"hastate,omitempty"`
	Pool     string  `json:"pool,omitempty"`
	Tags     string  `json:"tags,omitempty"`
	Template int     `json:"template,omitempty"`
}

// GetClusterStatus returns the cluster membership and quorum info.
func (c *Client) GetClusterStatus(ctx context.Context) ([]ClusterMember, error) {
	var out APIResponse[[]ClusterMember]
	if err := c.get(ctx, "/cluster/status", &out); err != nil {
		return nil, fmt.Errorf("cluster status: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// GetClusterResources returns all resources across the cluster.
// rtype can be "" (all), "vm", "lxc", "storage", "node", "pool", "sdn".
func (c *Client) GetClusterResources(ctx context.Context, rtype string) ([]ClusterResource, error) {
	path := "/cluster/resources"
	if rtype != "" {
		path += "?type=" + rtype
	}
	var out APIResponse[[]ClusterResource]
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("cluster resources: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// ── Pools ──────────────────────────────────────────────────────────────────

// Pool represents a resource pool.
type Pool struct {
	PoolID  string            `json:"poolid"`
	Comment string            `json:"comment,omitempty"`
	Members []ClusterResource `json:"members,omitempty"`
}

// GetPools returns all defined pools.
func (c *Client) GetPools(ctx context.Context) ([]Pool, error) {
	var out APIResponse[[]Pool]
	if err := c.get(ctx, "/pools", &out); err != nil {
		return nil, fmt.Errorf("get pools: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// GetPool returns a single pool with its members.
func (c *Client) GetPool(ctx context.Context, poolid string) (*Pool, error) {
	var out APIResponse[Pool]
	if err := c.get(ctx, fmt.Sprintf("/pools/%s", poolid), &out); err != nil {
		return nil, fmt.Errorf("get pool %s: %w", poolid, ClassifyError(err))
	}
	return &out.Data, nil
}

// CreatePool creates a new resource pool.
func (c *Client) CreatePool(ctx context.Context, poolid, comment string) error {
	body := map[string]string{"poolid": poolid}
	if comment != "" {
		body["comment"] = comment
	}
	var out APIResponse[any]
	if err := c.post(ctx, "/pools", body, &out); err != nil {
		return fmt.Errorf("create pool: %w", ClassifyError(err))
	}
	return nil
}

// DeletePool deletes a pool.
func (c *Client) DeletePool(ctx context.Context, poolid string) error {
	var out APIResponse[any]
	if err := c.del(ctx, fmt.Sprintf("/pools/%s", poolid), &out); err != nil {
		return fmt.Errorf("delete pool: %w", ClassifyError(err))
	}
	return nil
}
