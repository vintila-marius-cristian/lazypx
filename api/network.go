package api

import (
	"context"
	"fmt"
)

// NetworkInterface represents a network interface on a Proxmox node.
type NetworkInterface struct {
	Iface       string `json:"iface"`
	Type        string `json:"type"`   // bridge|eth|bond|vlan
	Method      string `json:"method"` // static|dhcp|manual
	Address     string `json:"address,omitempty"`
	Netmask     string `json:"netmask,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	BridgePorts string `json:"bridge_ports,omitempty"`
	Active      int    `json:"active"`
	Autostart   int    `json:"autostart"`
	Comments    string `json:"comments,omitempty"`
}

// GetNetworkInterfaces returns the network interfaces configured on a node.
func (c *Client) GetNetworkInterfaces(ctx context.Context, node string) ([]NetworkInterface, error) {
	var out APIResponse[[]NetworkInterface]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/network", node), &out); err != nil {
		return nil, fmt.Errorf("get network %s: %w", node, ClassifyError(err))
	}
	return out.Data, nil
}

// GuestNetworkInterface represents an interface returned by QEMU guest agent.
type GuestNetworkInterface struct {
	Name            string `json:"name"`
	HardwareAddress string `json:"hardware-address"`
	IPAddresses     []struct {
		IPAddress     string `json:"ip-address"`
		IPAddressType string `json:"ip-address-type"`
		Prefix        int    `json:"prefix"`
	} `json:"ip-addresses"`
	Statistics struct {
		RxBytes int64 `json:"rx-bytes"`
		TxBytes int64 `json:"tx-bytes"`
	} `json:"statistics"`
}

type GuestAgentResponse struct {
	Result []GuestNetworkInterface `json:"result"`
}

// GetGuestAgentNetworkInterfaces fetches IPs via the QEMU Guest Agent.
// Fails gracefully if the agent is disabled or unresponsive.
func (c *Client) GetGuestAgentNetworkInterfaces(ctx context.Context, node string, vmid int) ([]GuestNetworkInterface, error) {
	var out APIResponse[GuestAgentResponse]
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, vmid)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("guest agent network: %w", err)
	}
	return out.Data.Result, nil
}

// ── Firewall (read-only) ──────────────────────────────────────────────────

// FirewallRule represents a firewall rule.
type FirewallRule struct {
	Pos     int    `json:"pos"`
	Type    string `json:"type"`   // in|out|group
	Action  string `json:"action"` // ACCEPT|DROP|REJECT
	Enable  int    `json:"enable"`
	Source  string `json:"source,omitempty"`
	Dest    string `json:"dest,omitempty"`
	Proto   string `json:"proto,omitempty"`
	DPort   string `json:"dport,omitempty"`
	Sport   string `json:"sport,omitempty"`
	Comment string `json:"comment,omitempty"`
	Iface   string `json:"iface,omitempty"`
	Log     string `json:"log,omitempty"`
}

// GetClusterFirewallRules returns cluster-level firewall rules.
func (c *Client) GetClusterFirewallRules(ctx context.Context) ([]FirewallRule, error) {
	var out APIResponse[[]FirewallRule]
	if err := c.get(ctx, "/cluster/firewall/rules", &out); err != nil {
		return nil, fmt.Errorf("cluster firewall rules: %w", ClassifyError(err))
	}
	return out.Data, nil
}

// GetNodeFirewallRules returns node-level firewall rules.
func (c *Client) GetNodeFirewallRules(ctx context.Context, node string) ([]FirewallRule, error) {
	var out APIResponse[[]FirewallRule]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/firewall/rules", node), &out); err != nil {
		return nil, fmt.Errorf("node firewall rules %s: %w", node, ClassifyError(err))
	}
	return out.Data, nil
}

// GetVMFirewallRules returns VM-level firewall rules.
func (c *Client) GetVMFirewallRules(ctx context.Context, node string, vmid int) ([]FirewallRule, error) {
	var out APIResponse[[]FirewallRule]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/firewall/rules", node, vmid), &out); err != nil {
		return nil, fmt.Errorf("vm firewall rules %d: %w", vmid, ClassifyError(err))
	}
	return out.Data, nil
}
