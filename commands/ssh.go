package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"lazypx/config"

	"github.com/spf13/cobra"
)

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh <vmid|name>",
		Short: "Open an SSH shell to a VM or Container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetInput := args[0]
			var vmid int
			var err error

			// 1. Try to parse as integer VMID first
			vmid, err = strconv.Atoi(targetInput)
			if err != nil {
				// 2. If not an int, resolve the name via API
				vmid, err = resolveVMName(cfgGlobal, targetInput)
				if err != nil {
					return err
				}
			}

			sshHosts, err := config.LoadSSH()
			if err != nil {
				return fmt.Errorf("failed to load ssh config: %w", err)
			}

			host, ok := sshHosts[vmid]
			if !ok {
				return fmt.Errorf("no SSH mapping for VMID %d in ~/.config/lazypx/ssh.yaml", vmid)
			}

			target := host.Host
			if host.User != "" {
				target = host.User + "@" + host.Host
			}

			sshArgs := []string{}
			if host.IdentityFile != "" {
				sshArgs = append(sshArgs, "-i", host.IdentityFile)
			}
			if host.Port != 0 {
				sshArgs = append(sshArgs, "-p", strconv.Itoa(host.Port))
			}
			sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
			sshArgs = append(sshArgs, target)

			var c *exec.Cmd
			if host.Password != "" {
				if _, err := exec.LookPath("sshpass"); err == nil {
					sshArgs = append([]string{"-e", "ssh"}, sshArgs...)
					c = exec.Command("sshpass", sshArgs...)
					c.Env = append(os.Environ(), "SSHPASS="+host.Password)
				} else {
					c = exec.Command("ssh", sshArgs...)
				}
			} else {
				c = exec.Command("ssh", sshArgs...)
			}

			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			return c.Run()
		},
	}
}

// resolveVMName searches all nodes for a VM or CT matching the exact name.
func resolveVMName(cfg *config.Config, name string) (int, error) {
	c, err := clientFromConfig(cfg)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get nodes: %w", err)
	}

	for _, n := range nodes {
		// Check VMs
		vms, err := c.GetVMs(ctx, n.Node)
		if err == nil {
			for _, vm := range vms {
				if vm.Name == name {
					return vm.VMID, nil
				}
			}
		}

		// Check CTs
		cts, err := c.GetContainers(ctx, n.Node)
		if err == nil {
			for _, ct := range cts {
				if ct.Name == name {
					return ct.VMID, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("could not find a VM or CT with the name: %s", name)
}
