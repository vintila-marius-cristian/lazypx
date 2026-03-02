package commands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"lazypx/api"
	"lazypx/config"
)

func newSnapshotCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage VM and container snapshots",
	}
	cmd.AddCommand(
		newSnapshotListCmd(cfg),
		newSnapshotCreateCmd(cfg),
		newSnapshotDeleteCmd(cfg),
		newSnapshotRollbackCmd(cfg),
	)
	return cmd
}

// resolveVMWithKind finds a VMID's node, searching both QEMU and LXC if kind=="auto".
func resolveVMWithKind(cfg *config.Config, vmidStr, kind string) (int, string, *api.Client, string, error) {
	var vmid int
	if _, err := fmt.Sscanf(vmidStr, "%d", &vmid); err != nil {
		return 0, "", nil, "", fmt.Errorf("invalid vmid: %s", vmidStr)
	}
	c, err := clientFromConfig(cfg)
	if err != nil {
		return 0, "", nil, "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return 0, "", nil, "", err
	}
	for _, n := range nodes {
		if kind == "qemu" || kind == "auto" {
			vms, _ := c.GetVMs(ctx, n.Node)
			for _, vm := range vms {
				if vm.VMID == vmid {
					return vmid, n.Node, c, "qemu", nil
				}
			}
		}
		if kind == "lxc" || kind == "auto" {
			cts, _ := c.GetContainers(ctx, n.Node)
			for _, ct := range cts {
				if ct.VMID == vmid {
					return vmid, n.Node, c, "lxc", nil
				}
			}
		}
	}
	return 0, "", nil, "", fmt.Errorf("vmid %d not found", vmid)
}

func newSnapshotListCmd(cfg *config.Config) *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "list <vmid>",
		Short: "List snapshots for a VM or container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmid, node, c, resolvedKind, err := resolveVMWithKind(cfg, args[0], kind)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			snaps, err := c.GetSnapshots(ctx, node, vmid, resolvedKind)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tDESCRIPTION\tRUNNING")
			for _, s := range snaps {
				running := ""
				if s.Running == 1 {
					running = "yes (current)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Description, running)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVarP(&kind, "kind", "k", "auto", "Resource kind: auto|qemu|lxc")
	return cmd
}

func newSnapshotCreateCmd(cfg *config.Config) *cobra.Command {
	var kind, desc string
	cmd := &cobra.Command{
		Use:   "create <vmid> <snapname>",
		Short: "Create a snapshot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmid, node, c, resolvedKind, err := resolveVMWithKind(cfg, args[0], kind)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			upid, err := c.CreateSnapshot(ctx, node, vmid, resolvedKind, args[1], desc)
			if err != nil {
				return err
			}
			fmt.Printf("Snapshot task: %s\n", upid)
			return nil
		},
	}
	cmd.Flags().StringVarP(&kind, "kind", "k", "auto", "Resource kind: auto|qemu|lxc")
	cmd.Flags().StringVarP(&desc, "description", "d", "", "Snapshot description")
	return cmd
}

func newSnapshotDeleteCmd(cfg *config.Config) *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "delete <vmid> <snapname>",
		Short: "Delete a snapshot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmid, node, c, resolvedKind, err := resolveVMWithKind(cfg, args[0], kind)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			upid, err := c.DeleteSnapshot(ctx, node, vmid, resolvedKind, args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Delete snapshot task: %s\n", upid)
			return nil
		},
	}
	cmd.Flags().StringVarP(&kind, "kind", "k", "auto", "Resource kind: auto|qemu|lxc")
	return cmd
}

func newSnapshotRollbackCmd(cfg *config.Config) *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "rollback <vmid> <snapname>",
		Short: "Rollback VM to a snapshot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vmid, node, c, resolvedKind, err := resolveVMWithKind(cfg, args[0], kind)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			upid, err := c.RollbackSnapshot(ctx, node, vmid, resolvedKind, args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Rollback task: %s\n", upid)
			return nil
		},
	}
	cmd.Flags().StringVarP(&kind, "kind", "k", "auto", "Resource kind: auto|qemu|lxc")
	return cmd
}
