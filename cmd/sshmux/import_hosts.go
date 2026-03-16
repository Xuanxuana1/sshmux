package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/config"
	"github.com/liuxuan/sshmux/internal/state"
)

func newImportHostsCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "import-hosts",
		Short: "Import SSH hosts from OpenSSH config",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configPath
			if path == "" {
				defaultPath, err := config.DefaultConfigPath()
				if err != nil {
					return err
				}
				path = defaultPath
			}

			entries, err := config.ParseFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			imported := 0
			for _, e := range entries {
				port := 0
				if e.Port != "" {
					p, err := strconv.Atoi(e.Port)
					if err == nil {
						port = p
					}
				}

				// Check if already exists
				existing, err := state.Load(e.Alias)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not load state for %s: %v\n", e.Alias, err)
					continue
				}

				if existing != nil {
					// Update fields from config but preserve runtime state
					existing.Hostname = e.Hostname
					existing.User = e.User
					existing.Port = port
					existing.IdentityFile = e.IdentityFile
					existing.ProxyJump = e.ProxyJump
					existing.ProxyCommand = e.ProxyCommand
					existing.SourceConfigPath = e.SourceFile
					if err := state.Save(existing); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not save state for %s: %v\n", e.Alias, err)
						continue
					}
				} else {
					s := &state.HostState{
						HostAlias:        e.Alias,
						SourceConfigPath: e.SourceFile,
						Hostname:         e.Hostname,
						User:             e.User,
						Port:             port,
						IdentityFile:     e.IdentityFile,
						ProxyJump:        e.ProxyJump,
						ProxyCommand:     e.ProxyCommand,
					}
					if err := state.Save(s); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not save state for %s: %v\n", e.Alias, err)
						continue
					}
				}
				imported++
			}

			fmt.Printf("Imported %d hosts from %s\n", imported, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to SSH config file (default: ~/.ssh/config)")
	return cmd
}
