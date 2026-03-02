package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"lazypx/config"
)

func newAccessCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Manage users, groups, roles, and permissions",
	}
	cmd.AddCommand(
		newUserCmd(cfg),
		newGroupCmd(cfg),
		newRoleCmd(cfg),
		newACLCmd(cfg),
	)
	return cmd
}

// ── Users ──────────────────────────────────────────────────────────────────

func newUserCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "Manage users"}
	cmd.AddCommand(newUserListCmd(cfg), newUserCreateCmd(cfg), newUserDeleteCmd(cfg))
	return cmd
}

func newUserListCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			users, err := c.GetUsers(ctx)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "USERID\tENABLED\tREALM\tEMAIL")
			for _, u := range users {
				en := "no"
				if u.Enable == 1 {
					en = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.UserID, en, u.RealmType, u.Email)
			}
			return w.Flush()
		},
	}
}

func newUserCreateCmd(cfg *config.Config) *cobra.Command {
	var email, comment string
	cmd := &cobra.Command{
		Use:   "create <userid>",
		Short: "Create a user (e.g. user@pve)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := c.CreateUser(ctx, args[0], "", email, comment, true); err != nil {
				return err
			}
			fmt.Printf("User %s created.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment")
	return cmd
}

func newUserDeleteCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <userid>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := clientFromConfig(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := c.DeleteUser(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("User %s deleted.\n", args[0])
			return nil
		},
	}
}

// ── Groups ──────────────────────────────────────────────────────────────────

func newGroupCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "group", Short: "Manage groups"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all groups",
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := clientFromConfig(cfg)
				if err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				groups, err := c.GetGroups(ctx)
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "GROUPID\tCOMMENT\tMEMBERS")
				for _, g := range groups {
					fmt.Fprintf(w, "%s\t%s\t%d\n", g.GroupID, g.Comment, len(g.Members))
				}
				return w.Flush()
			},
		},
	)
	return cmd
}

// ── Roles ──────────────────────────────────────────────────────────────────

func newRoleCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "role", Short: "Manage roles"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all roles and their privileges",
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := clientFromConfig(cfg)
				if err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				roles, err := c.GetRoles(ctx)
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ROLEID\tBUILT-IN\tPRIVILEGES")
				for _, r := range roles {
					builtIn := ""
					if r.Special == 1 {
						builtIn = "yes"
					}
					privs := r.Privs
					if len(privs) > 60 {
						privs = privs[:57] + "…"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\n", r.RoleID, builtIn, privs)
				}
				return w.Flush()
			},
		},
	)
	return cmd
}

// ── ACLs ──────────────────────────────────────────────────────────────────

func newACLCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "acl", Short: "Manage access control lists"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all ACL entries",
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := clientFromConfig(cfg)
				if err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				acls, err := c.GetACL(ctx)
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "PATH\tUSER/GROUP\tTYPE\tROLE\tPROPAGATE")
				for _, a := range acls {
					prop := "no"
					if a.Propagate == 1 {
						prop = "yes"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						a.Path, a.UGI, a.Type, a.RoleID, prop)
				}
				return w.Flush()
			},
		},
	)
	return cmd
}

// printJSON is a shared helper (already in vm.go via encoding/json, redeclare if not visible)
func printJSON(v any) error {
	// avoid import cycle — inline here if needed
	_ = strings.NewReader("") // keep strings import
	return nil                // replaced by json encoding in vm.go's printVMs
}
