package api

import (
	"context"
	"fmt"
	"strings"
)

// GetBackups returns all backups for a specific VM/CT across all storages on a node.
func (c *Client) GetBackups(ctx context.Context, node string, vmid int) ([]BackupVolume, error) {
	// 1. Get all storages for the node
	storages, err := c.GetStorage(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("failed to list storages for backups: %w", err)
	}

	var allBackups []BackupVolume

	// 2. Iterate storages that support backups
	for _, st := range storages {
		if !strings.Contains(st.Content, "backup") {
			continue
		}

		var out APIResponse[[]BackupVolume]
		path := fmt.Sprintf("/nodes/%s/storage/%s/content?content=backup&vmid=%d", node, st.Storage, vmid)
		if err := c.get(ctx, path, &out); err != nil {
			// Some storages might return 500 if offline, skip errors and continue
			continue
		}

		for _, b := range out.Data {
			allBackups = append(allBackups, b)
		}
	}

	return allBackups, nil
}

// CreateBackup initiates a VZDump backup for a VM/CT and returns the task UPID.
// It automatically attempts to find a valid backup storage if storage is "".
func (c *Client) CreateBackup(ctx context.Context, node string, vmid int, storage string) (string, error) {
	if storage == "" {
		storages, err := c.GetStorage(ctx, node)
		if err == nil {
			for _, st := range storages {
				if strings.Contains(st.Content, "backup") && st.Active == 1 {
					storage = st.Storage
					break
				}
			}
		}
	}

	if storage == "" {
		return "", fmt.Errorf("no active storage supporting backups found on node %s", node)
	}

	body := map[string]string{
		"vmid":     fmt.Sprintf("%d", vmid),
		"storage":  storage,
		"mode":     "snapshot", // snapshot mode is standard minimal downtime
		"compress": "zstd",
	}

	var out APIResponse[string]
	path := fmt.Sprintf("/nodes/%s/vzdump", node)
	if err := c.post(ctx, path, body, &out); err != nil {
		return "", fmt.Errorf("create backup on %d failed: %w", vmid, ClassifyError(err))
	}

	return out.Data, nil
}
