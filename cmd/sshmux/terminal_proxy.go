package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/state"
	"github.com/liuxuan/sshmux/internal/termproxy"
)

func newTerminalProxyCmd() *cobra.Command {
	tpCmd := &cobra.Command{
		Use:   "terminal-proxy",
		Short: "Manage terminal proxy environment variables",
	}

	var httpAddr string
	var socksAddr string

	onCmd := &cobra.Command{
		Use:   "on",
		Short: "Enable terminal proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load existing config to preserve previous addresses
			existing, err := state.LoadTerminalProxy()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			// Use new flags if provided, otherwise keep existing
			if cmd.Flags().Changed("http") {
				existing.HTTPAddr = httpAddr
			}
			if cmd.Flags().Changed("socks") {
				existing.SOCKSAddr = socksAddr
			}

			if existing.HTTPAddr == "" && existing.SOCKSAddr == "" {
				fmt.Fprintf(os.Stderr, "Error: at least one of --http or --socks must be specified\n")
				return fmt.Errorf("no proxy address specified")
			}

			if err := termproxy.Enable(existing); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			fmt.Println("Terminal proxy enabled")
			if existing.HTTPAddr != "" {
				fmt.Printf("  HTTP:  %s\n", existing.HTTPAddr)
			}
			if existing.SOCKSAddr != "" {
				fmt.Printf("  SOCKS: %s\n", existing.SOCKSAddr)
			}
			fmt.Println("New terminal sessions will inherit proxy environment variables.")
			return nil
		},
	}
	onCmd.Flags().StringVar(&httpAddr, "http", "", "HTTP proxy address (e.g. 127.0.0.1:7897)")
	onCmd.Flags().StringVar(&socksAddr, "socks", "", "SOCKS proxy address (e.g. 127.0.0.1:7897)")

	offCmd := &cobra.Command{
		Use:   "off",
		Short: "Disable terminal proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := termproxy.Disable(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			fmt.Println("Terminal proxy disabled. New terminal sessions will not have proxy env vars.")
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show terminal proxy status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := state.LoadTerminalProxy()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			if cfg.Enabled {
				fmt.Println("Terminal proxy: enabled")
			} else {
				fmt.Println("Terminal proxy: disabled")
			}
			if cfg.HTTPAddr != "" {
				fmt.Printf("  HTTP:  %s\n", cfg.HTTPAddr)
			}
			if cfg.SOCKSAddr != "" {
				fmt.Printf("  SOCKS: %s\n", cfg.SOCKSAddr)
			}
			return nil
		},
	}

	tpCmd.AddCommand(onCmd, offCmd, statusCmd)
	return tpCmd
}
