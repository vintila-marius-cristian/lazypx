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

func newClusterCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster-wide operations",
	}
	cmd.AddCommand(newClusterStatusCmd(cfg), newClusterResourcesCmd(cfg))
	return cmd
}

func newClusterStatusCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cluster membership and quorum",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			members, err := c.GetClusterStatus(ctx)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tIP\tONLINE\tLOCAL")
			for _, m := range members {
				online := "no"
				if m.Online == 1 {
					online = "yes"
				}
				local := ""
				if m.Local == 1 {
					local = "✓"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", m.Name, m.Type, m.IP, online, local)
			}
			return w.Flush()
		},
	}
}

func newClusterResourcesCmd(cfg *config.Config) *cobra.Command {
	var rtype, output string
	cmd := &cobra.Command{
		Use:   "resources",
		Short: "List all cluster resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			resources, err := c.GetClusterResources(ctx, rtype)
			if err != nil {
				return err
			}
			switch output {
			case "json":
				return printJSON(resources)
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tTYPE\tNODE\tNAME\tSTATUS\tCPU\tRAM")
				for _, r := range resources {
					cpu := ""
					if r.CPU > 0 {
						cpu = fmt.Sprintf("%.1f%%", r.CPU*100)
					}
					ram := ""
					if r.MaxMem > 0 {
						ram = fmt.Sprintf("%s/%s", formatBytesCmd(r.Mem), formatBytesCmd(r.MaxMem))
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						r.ID, r.Type, r.Node, r.Name, r.Status, cpu, ram)
				}
				return w.Flush()
			}
		},
	}
	cmd.Flags().StringVarP(&rtype, "type", "t", "", "Filter by type: vm|lxc|storage|node|pool")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json")
	return cmd
}
