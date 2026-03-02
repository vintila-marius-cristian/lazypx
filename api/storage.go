package api

import (
	"context"
	"fmt"
)

// GetStorage returns storage pools for a node.
func (c *Client) GetStorage(ctx context.Context, node string) ([]StorageStatus, error) {
	var out APIResponse[[]StorageStatus]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/storage", node), &out); err != nil {
		return nil, fmt.Errorf("get storage: %w", err)
	}
	for i := range out.Data {
		out.Data[i].Node = node
	}
	return out.Data, nil
}
