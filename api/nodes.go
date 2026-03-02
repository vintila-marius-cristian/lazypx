package api

import (
	"context"
	"fmt"
)

// GetNodes returns all nodes in the cluster.
func (c *Client) GetNodes(ctx context.Context) ([]NodeStatus, error) {
	var out APIResponse[[]NodeStatus]
	if err := c.get(ctx, "/nodes", &out); err != nil {
		return nil, fmt.Errorf("get nodes: %w", err)
	}
	return out.Data, nil
}

// nodeStatusRaw is the real shape of /nodes/{node}/status — nested sub-objects.
type nodeStatusRaw struct {
	CPU     float64    `json:"cpu"`
	Uptime  int64      `json:"uptime"`
	PVEVer  string     `json:"pveversion"`
	KVer    string     `json:"kversion"`
	Memory  memInfo    `json:"memory"`
	Swap    memInfo    `json:"swap"`
	RootFS  fsInfo     `json:"rootfs"`
	CPUInfo cpuInfo    `json:"cpuinfo"`
	Kernel  kernelInfo `json:"current-kernel"`
	Wait    float64    `json:"wait"`
}

type memInfo struct {
	Used  int64 `json:"used"`
	Total int64 `json:"total"`
	Free  int64 `json:"free"`
}

type fsInfo struct {
	Used  int64 `json:"used"`
	Total int64 `json:"total"`
	Avail int64 `json:"avail"`
}

type cpuInfo struct {
	Model   string `json:"model"`
	CPUs    int    `json:"cpus"`
	Cores   int    `json:"cores"`
	Sockets int    `json:"sockets"`
	MHz     string `json:"mhz"`
}

type kernelInfo struct {
	Release string `json:"release"`
	SysName string `json:"sysname"`
	Machine string `json:"machine"`
}

// GetNodeStatus returns detailed status for a single node, with nested fields mapped
// into the flat NodeStatus struct used everywhere.
func (c *Client) GetNodeStatus(ctx context.Context, node string) (*NodeStatus, error) {
	var raw APIResponse[nodeStatusRaw]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/status", node), &raw); err != nil {
		return nil, fmt.Errorf("get node status %s: %w", node, ClassifyError(err))
	}
	r := raw.Data

	// Map the nested /status response back to the flat NodeStatus we use everywhere.
	ns := &NodeStatus{
		Node:      node,
		Status:    "online",
		CPUUsage:  r.CPU,
		MaxCPU:    r.CPUInfo.CPUs,
		MemUsed:   r.Memory.Used,
		MemTotal:  r.Memory.Total,
		DiskUsed:  r.RootFS.Used,
		DiskTotal: r.RootFS.Total,
		Uptime:    r.Uptime,
		// Extra fields carried in the extended struct below
	}

	// Attach extended info for the detail pane
	ns.Extended = &NodeStatusExtended{
		PVEVersion:  r.PVEVer,
		KernelVer:   r.Kernel.Release,
		CPUModel:    r.CPUInfo.Model,
		CPUCores:    r.CPUInfo.Cores,
		CPUSockets:  r.CPUInfo.Sockets,
		CPUMHz:      r.CPUInfo.MHz,
		SwapUsed:    r.Swap.Used,
		SwapTotal:   r.Swap.Total,
		RootFSAvail: r.RootFS.Avail,
	}
	return ns, nil
}
