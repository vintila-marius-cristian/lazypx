package api

import (
	"context"
	"fmt"
	"strconv"
)

// VMConfig holds the full configuration of a QEMU VM.
// Only commonly needed fields are typed; the rest fold into Extra.
type VMConfig struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OSType      string `json:"ostype,omitempty"`
	Cores       int    `json:"cores,omitempty"`
	Sockets     int    `json:"sockets,omitempty"`
	CPU         string `json:"cpu,omitempty"`
	Memory      int    `json:"memory,omitempty"`
	BIOS        string `json:"bios,omitempty"`
	Boot        string `json:"boot,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Tags        string `json:"tags,omitempty"`
	Protection  int    `json:"protection,omitempty"`
	Lock        string `json:"lock,omitempty"`
	Digest      string `json:"digest,omitempty"`
	// Network interfaces: net0, net1, …
	Net0 string `json:"net0,omitempty"`
	Net1 string `json:"net1,omitempty"`
	// Disks: scsi0, virtio0, ide0, …
	Scsi0   string `json:"scsi0,omitempty"`
	Scsi1   string `json:"scsi1,omitempty"`
	Virtio0 string `json:"virtio0,omitempty"`
}

// CTConfig holds the configuration of an LXC container.
type CTConfig struct {
	Hostname    string `json:"hostname"`
	Description string `json:"description,omitempty"`
	Arch        string `json:"arch,omitempty"`
	OSType      string `json:"ostype,omitempty"`
	Cores       int    `json:"cores,omitempty"`
	Memory      int    `json:"memory,omitempty"`
	Swap        int    `json:"swap,omitempty"`
	Tags        string `json:"tags,omitempty"`
	Protection  int    `json:"protection,omitempty"`
	Lock        string `json:"lock,omitempty"`
	Net0        string `json:"net0,omitempty"`
	Rootfs      string `json:"rootfs,omitempty"`
}

// GetVMConfig fetches the full configuration of a QEMU VM.
func (c *Client) GetVMConfig(ctx context.Context, node string, vmid int) (*VMConfig, error) {
	var out APIResponse[VMConfig]
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("vm config %d: %w", vmid, ClassifyError(err))
	}
	return &out.Data, nil
}

// GetCTConfig fetches the full configuration of an LXC container.
func (c *Client) GetCTConfig(ctx context.Context, node string, vmid int) (*CTConfig, error) {
	var out APIResponse[CTConfig]
	path := fmt.Sprintf("/nodes/%s/lxc/%d/config", node, vmid)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("ct config %d: %w", vmid, ClassifyError(err))
	}
	return &out.Data, nil
}

// CloneVM clones a VM to a new VMID. Returns the task UPID.
// target: target node (empty = same node). newid: new VMID.
func (c *Client) CloneVM(ctx context.Context, node string, vmid, newid int, name, target string) (string, error) {
	body := map[string]string{
		"newid": strconv.Itoa(newid),
		"full":  "1",
	}
	if name != "" {
		body["name"] = name
	}
	if target != "" {
		body["target"] = target
	}
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", node, vmid)
	if err := c.post(ctx, path, body, &out); err != nil {
		return "", fmt.Errorf("clone vm %d → %d: %w", vmid, newid, ClassifyError(err))
	}
	return out.Data, nil
}

// CloneCT clones an LXC container. Returns the task UPID.
func (c *Client) CloneCT(ctx context.Context, node string, vmid, newid int, hostname string) (string, error) {
	body := map[string]string{
		"newid": strconv.Itoa(newid),
	}
	if hostname != "" {
		body["hostname"] = hostname
	}
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/lxc/%d/clone", node, vmid)
	if err := c.post(ctx, path, body, &out); err != nil {
		return "", fmt.Errorf("clone ct %d → %d: %w", vmid, newid, ClassifyError(err))
	}
	return out.Data, nil
}
