package api

import (
	"context"
	"fmt"
)

// HAResource is an entry in /cluster/ha/resources.
type HAResource struct {
	SID         string `json:"sid"`   // e.g. "vm:100"
	Type        string `json:"type"`  // vm|ct
	State       string `json:"state"` // started|stopped|enabled|disabled|ignored
	Max         int    `json:"max_restart"`
	MaxRelocate int    `json:"max_relocate"`
	Group       string `json:"group,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// HAGroup is an entry in /cluster/ha/groups.
type HAGroup struct {
	Group      string `json:"group"`
	Nodes      string `json:"nodes"`
	Restricted int    `json:"restricted,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// HAStatus represents the HA manager status from /cluster/ha/status/current.
type HAStatus struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Node    string `json:"node,omitempty"`
	Comment string `json:"crm_state,omitempty"`
}

// GetHAResources returns all HA-managed resources.
func (c *Client) GetHAResources(ctx context.Context) ([]HAResource, error) {
	var out APIResponse[[]HAResource]
	if err := c.get(ctx, "/cluster/ha/resources", &out); err != nil {
		return nil, fmt.Errorf("ha resources: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// GetHAGroups returns all HA groups.
func (c *Client) GetHAGroups(ctx context.Context) ([]HAGroup, error) {
	var out APIResponse[[]HAGroup]
	if err := c.get(ctx, "/cluster/ha/groups", &out); err != nil {
		return nil, fmt.Errorf("ha groups: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// GetHAStatus returns the current HA manager status entries.
func (c *Client) GetHAStatus(ctx context.Context) ([]HAStatus, error) {
	var out APIResponse[[]HAStatus]
	if err := c.get(ctx, "/cluster/ha/status/current", &out); err != nil {
		return nil, fmt.Errorf("ha status: %w", ClassifyError(err))
	}
	return out.Data, nil
}
