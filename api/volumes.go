package api

import (
	"context"
	"fmt"
)

// Volume is a storage volume from /nodes/{node}/storage/{storage}/content.
type Volume struct {
	Volid  string  `json:"volid"`
	Name   string  `json:"name,omitempty"`
	Format string  `json:"format"`
	Size   int64   `json:"size"`
	Used   int64   `json:"used,omitempty"`
	VMID   string  `json:"vmid,omitempty"`
	CTime  float64 `json:"ctime,omitempty"`
	Notes  string  `json:"notes,omitempty"`
	CT     int     `json:"content,omitempty"`
}

// GetVolumes lists volumes in a specific storage on a node.
// content can be "" (all) or "images", "iso", "vztmpl", "backup", etc.
func (c *Client) GetVolumes(ctx context.Context, node, storage, content string) ([]Volume, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
	if content != "" {
		path += "?content=" + content
	}
	var out APIResponse[[]Volume]
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get volumes %s/%s: %w", node, storage, ClassifyError(err))
	}
	return out.Data, nil
}

// DeleteVolume deletes a volume. Returns the task UPID.
func (c *Client) DeleteVolume(ctx context.Context, node, storage, volid string) (string, error) {
	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", node, storage, volid)
	if err := c.del(ctx, path, &out); err != nil {
		return "", fmt.Errorf("delete volume %s: %w", volid, ClassifyError(err))
	}
	return out.Data, nil
}

// ResizeDisk resizes a disk attached to a VM or CT.
// kind: "qemu" or "lxc". size: e.g. "+10G" or "50G".
func (c *Client) ResizeDisk(ctx context.Context, node string, vmid int, kind, disk, size string) (string, error) {
	var path string
	if kind == "lxc" {
		path = fmt.Sprintf("/nodes/%s/lxc/%d/resize", node, vmid)
	} else {
		path = fmt.Sprintf("/nodes/%s/qemu/%d/resize", node, vmid)
	}
	var out APIResponse[string]
	if err := c.put(ctx, path, map[string]string{"disk": disk, "size": size}, &out); err != nil {
		return "", fmt.Errorf("resize disk %s on vm %d: %w", disk, vmid, ClassifyError(err))
	}
	return out.Data, nil
}

// MoveDisk moves a VM disk to a different storage. Returns the task UPID.
// storage: target storage ID, e.g. "local-lvm".
func (c *Client) MoveDisk(ctx context.Context, node string, vmid int, disk, storage string, deleteSource bool) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/move_disk", node, vmid)
	del := "0"
	if deleteSource {
		del = "1"
	}
	var out APIResponse[string]
	if err := c.post(ctx, path, map[string]string{
		"disk":    disk,
		"storage": storage,
		"delete":  del,
	}, &out); err != nil {
		return "", fmt.Errorf("move disk %s on vm %d: %w", disk, vmid, ClassifyError(err))
	}
	return out.Data, nil
}
