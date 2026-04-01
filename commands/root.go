// Package commands provides the Cobra CLI command tree.
// clientFromConfig builds an API client from the active profile.
package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"lazypx/api"
	"lazypx/cache"
	"lazypx/config"
	"lazypx/tui"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	profileFlag string
	cfgGlobal   *config.Config
)

// Root builds and returns the root Cobra command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "lazypx",
		Short: "Blazing-fast Proxmox TUI — powered by Bubble Tea",
		Long: `lazypx — A lazygit-style terminal UI for Proxmox VE clusters.

Run without subcommands to launch the interactive TUI.
Use subcommands for direct CLI access.

Configuration: ~/.config/lazypx/config.yaml`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// PersistentPreRunE already loaded config into cfgGlobal.
			if cfgGlobal == nil {
				cfg, err := config.Load(profileFlag)
				if err != nil {
					return err
				}
				cfgGlobal = cfg
			}
			return runTUI(cfgGlobal)
		},
	}

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(profileFlag)
		if err != nil {
			return err
		}
		cfgGlobal = cfg
		return nil
	}

	root.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "Configuration profile to use")

	// Sub-commands
	root.AddCommand(
		newVMCmd(nil), // cfg injected in PersistentPreRunE
		newNodeCmd(nil),
		newClusterCmd(nil),
		newSnapshotCmd(nil),
		newAccessCmd(nil),
		newInitConfigCmd(),
		newVersionCmd(),
		newSSHCmd(),
	)

	// Inject global config into sub-commands at run time
	cobra.OnInitialize(func() {
		for _, sub := range root.Commands() {
			injectConfig(sub)
		}
	})

	return root
}

// injectConfig walks sub-command tree and injects cfgGlobal.
// (Simple approach: re-bind via closure; cobra doesn't support dynamic injection cleanly)
// For v0.1 we call Load() again inside each command instead.
func injectConfig(_ *cobra.Command) {}

// clientFromConfig creates an API client from the active profile.
func clientFromConfig(cfg *config.Config) (*api.Client, error) {
	if cfgGlobal != nil && cfgGlobal.ActiveProfile != nil {
		cfg = cfgGlobal
	}
	if cfg == nil || cfg.ActiveProfile == nil {
		return nil, fmt.Errorf("no profile configured\n\nRun: lazypx init-config")
	}
	p := cfg.ActiveProfile
	return api.NewClient(p.Host, p.TokenID, p.TokenSecret, p.TLSInsecure), nil
}

func runTUI(cfg *config.Config) error {
	if cfg.ActiveProfile == nil {
		fmt.Println("No profile configured. Create ~/.config/lazypx/config.yaml")
		fmt.Println("\nExample config:")
		fmt.Println(config.ExampleConfig())
		fmt.Println("Then run: lazypx")
		return nil
	}
	p := cfg.ActiveProfile
	client := api.NewClient(p.Host, p.TokenID, p.TokenSecret, p.TLSInsecure)

	refreshSecs := p.RefreshInterval
	if refreshSecs == 0 {
		refreshSecs = 30
	}
	clusterCache := cache.New(client, time.Duration(refreshSecs)*time.Second)

	model := tui.New(client, clusterCache, p)
	prog := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := prog.Run()
	return err
}

func newInitConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init-config",
		Short: "Show example configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("# Save this to ~/.config/lazypx/config.yaml")
			fmt.Println(config.ExampleConfig())
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("lazypx v0.1.0")
		},
	}
}
