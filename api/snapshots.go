package api

import (
	"context"
	"fmt"
)

// Snapshot represents a VM or CT snapshot.
type Snapshot struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	SnapTime    float64 `json:"snaptime,omitempty"`
	Running     int     `json:"running,omitempty"`
	Parent      string  `json:"parent,omitempty"`
}

// snapshotBase returns the API path prefix for VM or CT snapshots.
func snapshotBase(node string, vmid int, kind string) string {
	if kind == "lxc" {
		return fmt.Sprintf("/nodes/%s/lxc/%d/snapshot", node, vmid)
	}
	return fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
}

// GetSnapshots returns all snapshots for a VM (kind="qemu") or CT (kind="lxc").
func (c *Client) GetSnapshots(ctx context.Context, node string, vmid int, kind string) ([]Snapshot, error) {
	var out APIResponse[[]Snapshot]
	if err := c.get(ctx, snapshotBase(node, vmid, kind), &out); err != nil {
		return nil, fmt.Errorf("get snapshots %d: %w", vmid, ClassifyError(err))
	}
	return out.Data, nil
}

// CreateSnapshot creates a new snapshot. Returns the task UPID.
func (c *Client) CreateSnapshot(ctx context.Context, node string, vmid int, kind, snapname, description string) (string, error) {
	body := map[string]string{"snapname": snapname}
	if description != "" {
		body["description"] = description
	}
	var out APIResponse[string]
	if err := c.post(ctx, snapshotBase(node, vmid, kind), body, &out); err != nil {
		return "", fmt.Errorf("create snapshot %s on %d: %w", snapname, vmid, ClassifyError(err))
	}
	return out.Data, nil
}

// DeleteSnapshot deletes a snapshot by name. Returns the task UPID.
func (c *Client) DeleteSnapshot(ctx context.Context, node string, vmid int, kind, snapname string) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("%s/%s", snapshotBase(node, vmid, kind), snapname)
	if err := c.del(ctx, path, &out); err != nil {
		return "", fmt.Errorf("delete snapshot %s on %d: %w", snapname, vmid, ClassifyError(err))
	}
	return out.Data, nil
}

// RollbackSnapshot rolls back a VM/CT to a snapshot. Returns the task UPID.
func (c *Client) RollbackSnapshot(ctx context.Context, node string, vmid int, kind, snapname string) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("%s/%s/rollback", snapshotBase(node, vmid, kind), snapname)
	if err := c.post(ctx, path, nil, &out); err != nil {
		return "", fmt.Errorf("rollback %s on %d: %w", snapname, vmid, ClassifyError(err))
	}
	return out.Data, nil
}
