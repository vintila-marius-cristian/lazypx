package commands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"lazypx/config"
)

func newNodeCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage cluster nodes",
	}
	cmd.AddCommand(newNodeListCmd(cfg), newNodeStatusCmd(cfg))
	return cmd
}

func newNodeListCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			nodes, err := c.GetNodes(ctx)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NODE\tSTATUS\tCPU\tRAM\tUPTIME")
			for _, n := range nodes {
				cpuStr := fmt.Sprintf("%.1f%%", n.CPUUsage*100)
				ramStr := fmt.Sprintf("%s/%s", formatBytesCmd(n.MemUsed), formatBytesCmd(n.MemTotal))
				upStr := fmt.Sprintf("%ds", n.Uptime)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", n.Node, n.Status, cpuStr, ramStr, upStr)
			}
			return w.Flush()
		},
	}
}

func newNodeStatusCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status <node>",
		Short: "Get status for a specific node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			n, err := c.GetNodeStatus(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Node:    %s\n", args[0])
			fmt.Printf("Status:  %s\n", n.Status)
			fmt.Printf("CPU:     %.1f%% (%d cores)\n", n.CPUUsage*100, n.MaxCPU)
			fmt.Printf("RAM:     %s / %s\n", formatBytesCmd(n.MemUsed), formatBytesCmd(n.MemTotal))
			fmt.Printf("Disk:    %s / %s\n", formatBytesCmd(n.DiskUsed), formatBytesCmd(n.DiskTotal))
			fmt.Printf("Uptime:  %ds\n", n.Uptime)
			return nil
		},
	}
}
