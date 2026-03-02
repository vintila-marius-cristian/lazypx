package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"lazypx/api"
	"lazypx/config"
)

func newVMCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vm",
		Short: "Manage virtual machines",
	}
	cmd.AddCommand(
		newVMListCmd(cfg),
		newVMStartCmd(cfg),
		newVMStopCmd(cfg),
		newVMRebootCmd(cfg),
	)
	return cmd
}

func newVMListCmd(cfg *config.Config) *cobra.Command {
	var node, output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all VMs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Determine nodes to query
			var nodes []string
			if node != "" {
				nodes = []string{node}
			} else {
				ns, err := c.GetNodes(ctx)
				if err != nil {
					return err
				}
				for _, n := range ns {
					nodes = append(nodes, n.Node)
				}
			}

			var allVMs []api.VMStatus
			for _, n := range nodes {
				vms, err := c.GetVMs(ctx, n)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: node %s: %v\n", n, err)
					continue
				}
				allVMs = append(allVMs, vms...)
			}

			return printVMs(allVMs, output)
		},
	}
	cmd.Flags().StringVarP(&node, "node", "n", "", "Filter to specific node")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json|yaml")
	return cmd
}

func newVMStartCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "start <vmid>",
		Short: "Start a VM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmid, node, c, resolvedKind, err := resolveVMWithKind(cfg, args[0], "auto")
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var upid string
			if resolvedKind == "qemu" {
				upid, err = c.StartVM(ctx, node, vmid)
			} else {
				upid, err = c.StartCT(ctx, node, vmid)
			}

			if err != nil {
				return err
			}
			fmt.Printf("Task started: %s\n", upid)
			return nil
		},
	}
}

func newVMStopCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <vmid>",
		Short: "Stop a VM (graceful shutdown)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmid, node, c, resolvedKind, err := resolveVMWithKind(cfg, args[0], "auto")
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var upid string
			if resolvedKind == "qemu" {
				upid, err = c.StopVM(ctx, node, vmid)
			} else {
				upid, err = c.StopCT(ctx, node, vmid)
			}

			if err != nil {
				return err
			}
			fmt.Printf("Task started: %s\n", upid)
			return nil
		},
	}
}

func newVMRebootCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "reboot <vmid>",
		Short: "Reboot a VM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmid, node, c, resolvedKind, err := resolveVMWithKind(cfg, args[0], "auto")
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var upid string
			if resolvedKind == "qemu" {
				upid, err = c.RebootVM(ctx, node, vmid)
			} else {
				upid, err = c.RebootCT(ctx, node, vmid)
			}

			if err != nil {
				return err
			}
			fmt.Printf("Task started: %s\n", upid)
			return nil
		},
	}
}

// Removed resolveVM since we now use resolveVMWithKind from snapshot.go

func printVMs(vms []api.VMStatus, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(vms)
	default: // table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "VMID\tNAME\tSTATUS\tNODE\tCPU\tRAM")
		for _, vm := range vms {
			cpuStr := fmt.Sprintf("%.1f%%", vm.CPU*100)
			ramStr := fmt.Sprintf("%s/%s",
				formatBytesCmd(vm.MemUsed), formatBytesCmd(vm.MemTotal))
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
				vm.VMID, vm.Name, vm.Status, vm.Node, cpuStr, ramStr)
		}
		return w.Flush()
	}
}

func formatBytesCmd(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}
