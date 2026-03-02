package api

import (
	"context"
	"fmt"
)

// GetContainers returns all LXC containers on a node.
func (c *Client) GetContainers(ctx context.Context, node string) ([]CTStatus, error) {
	var out APIResponse[[]CTStatus]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc", node), &out); err != nil {
		return nil, fmt.Errorf("get containers: %w", err)
	}
	for i := range out.Data {
		out.Data[i].Node = node
		out.Data[i].Type = "ct"
	}
	return out.Data, nil
}

// StartCT starts a container. Returns task UPID.
func (c *Client) StartCT(ctx context.Context, node string, vmid int) (string, error) {
	return c.ctAction(ctx, node, vmid, "start")
}

// StopCT stops a container. Returns task UPID.
func (c *Client) StopCT(ctx context.Context, node string, vmid int) (string, error) {
	return c.ctAction(ctx, node, vmid, "stop")
}

// RebootCT reboots a container. Returns task UPID.
func (c *Client) RebootCT(ctx context.Context, node string, vmid int) (string, error) {
	return c.ctAction(ctx, node, vmid, "reboot")
}

// ShutdownCT gracefully shuts down a container. Returns task UPID.
func (c *Client) ShutdownCT(ctx context.Context, node string, vmid int) (string, error) {
	return c.ctAction(ctx, node, vmid, "shutdown")
}

func (c *Client) ctAction(ctx context.Context, node string, vmid int, action string) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/lxc/%d/status/%s", node, vmid, action)
	if err := c.post(ctx, path, nil, &out); err != nil {
		return "", fmt.Errorf("ct %s %d %s: %w", node, vmid, action, err)
	}
	return out.Data, nil
}

// DeleteCT deletes a container. Returns task UPID.
func (c *Client) DeleteCT(ctx context.Context, node string, vmid int) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/lxc/%d", node, vmid)
	if err := c.del(ctx, path, &out); err != nil {
		return "", fmt.Errorf("delete ct %d: %w", vmid, err)
	}
	return out.Data, nil
}
