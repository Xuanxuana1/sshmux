package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/state"
)

func newHostsCmd() *cobra.Command {
	hostsCmd := &cobra.Command{
		Use:   "hosts",
		Short: "Manage SSH hosts",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all imported SSH hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			hosts, err := state.ListHosts()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			if len(hosts) == 0 {
				fmt.Println("No hosts imported. Run: sshmux import-hosts")
				return nil
			}

			fmt.Printf("%-20s %-20s %-10s %-10s %-10s\n", "ALIAS", "HOSTNAME", "USER", "CONNECTED", "SOCKS")
			for _, h := range hosts {
				connected := "no"
				if h.MasterConnected {
					connected = "yes"
				}
				socks := "-"
				if h.SocksEnabled {
					socks = fmt.Sprintf(":%d", h.SocksPort)
				}
				fmt.Printf("%-20s %-20s %-10s %-10s %-10s\n",
					h.HostAlias, h.Hostname, h.User, connected, socks)
			}
			return nil
		},
	}

	hostsCmd.AddCommand(listCmd)
	return hostsCmd
}
