package api

import (
	"context"
	"fmt"
	"strconv"
)

// GetVMs returns all VMs on a node.
func (c *Client) GetVMs(ctx context.Context, node string) ([]VMStatus, error) {
	var out APIResponse[[]VMStatus]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu", node), &out); err != nil {
		return nil, fmt.Errorf("get vms: %w", err)
	}
	for i := range out.Data {
		out.Data[i].Node = node
	}
	return out.Data, nil
}

// StartVM starts a VM. Returns the task UPID.
func (c *Client) StartVM(ctx context.Context, node string, vmid int) (string, error) {
	return c.vmAction(ctx, node, vmid, "start")
}

// StopVM stops a VM (immediate). Returns the task UPID.
func (c *Client) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	return c.vmAction(ctx, node, vmid, "stop")
}

// ShutdownVM gracefully shuts down a VM. Returns the task UPID.
func (c *Client) ShutdownVM(ctx context.Context, node string, vmid int) (string, error) {
	return c.vmAction(ctx, node, vmid, "shutdown")
}

// RebootVM reboots a VM. Returns the task UPID.
func (c *Client) RebootVM(ctx context.Context, node string, vmid int) (string, error) {
	return c.vmAction(ctx, node, vmid, "reboot")
}

func (c *Client) vmAction(ctx context.Context, node string, vmid int, action string) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/%s", node, vmid, action)
	if err := c.post(ctx, path, nil, &out); err != nil {
		return "", fmt.Errorf("vm %s %d %s: %w", node, vmid, action, err)
	}
	return out.Data, nil
}

// DeleteVM deletes a VM. Returns the task UPID.
func (c *Client) DeleteVM(ctx context.Context, node string, vmid int) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/qemu/%d", node, vmid)
	if err := c.del(ctx, path, &out); err != nil {
		return "", fmt.Errorf("delete vm %d: %w", vmid, err)
	}
	return out.Data, nil
}

// MigrateVM migrates a VM to a target node. Returns the task UPID.
func (c *Client) MigrateVM(ctx context.Context, node string, vmid int, target string, online bool) (string, error) {
	onlineVal := "0"
	if online {
		onlineVal = "1"
	}
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/qemu/%d/migrate", node, vmid)
	if err := c.post(ctx, path, map[string]string{
		"target": target,
		"online": onlineVal,
	}, &out); err != nil {
		return "", fmt.Errorf("migrate vm %d: %w", vmid, err)
	}
	return out.Data, nil
}

// BackupVM creates a backup of a VM. Returns the task UPID.
func (c *Client) BackupVM(ctx context.Context, node string, vmid int, storage string) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/vzdump", node)
	if err := c.post(ctx, path, map[string]string{
		"vmid":    strconv.Itoa(vmid),
		"storage": storage,
		"mode":    "snapshot",
	}, &out); err != nil {
		return "", fmt.Errorf("backup vm %d: %w", vmid, err)
	}
	return out.Data, nil
}
