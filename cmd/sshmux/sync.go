package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/macos"
	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
)

func newSyncCmd() *cobra.Command {
	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Manage macOS system proxy sync",
	}

	var service string
	var mode string

	onCmd := &cobra.Command{
		Use:   "on <host>",
		Short: "Enable macOS system proxy sync",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)
			if s == nil {
				s = &state.HostState{HostAlias: host}
			}

			syncMode := state.MacSyncMode(mode)
			macProxy := macos.NewProxy(ssh.ExecRunner{})

			switch syncMode {
			case state.MacSyncSOCKSOnly, state.MacSyncFull:
				if !s.SocksEnabled {
					fmt.Fprintf(os.Stderr, "Error: SOCKS proxy must be enabled first\n")
					return fmt.Errorf("SOCKS proxy not enabled for %s", host)
				}
				if err := macProxy.SetSOCKS(ctx, service, "127.0.0.1", s.SocksPort); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
			}

			switch syncMode {
			case state.MacSyncHTTPOnly, state.MacSyncFull:
				if !s.HTTPEnabled {
					fmt.Fprintf(os.Stderr, "Error: HTTP proxy must be enabled first\n")
					return fmt.Errorf("HTTP proxy not enabled for %s", host)
				}
				if err := macProxy.SetWebProxy(ctx, service, "127.0.0.1", s.HTTPPort); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				if err := macProxy.SetSecureWebProxy(ctx, service, "127.0.0.1", s.HTTPPort); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
			}

			s.MacSyncEnabled = true
			s.MacSyncMode = syncMode
			s.MacNetworkService = service
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("macOS proxy sync enabled for %s (service=%s, mode=%s)\n", host, service, mode)
			return nil
		},
	}
	onCmd.Flags().StringVar(&service, "service", "Wi-Fi", "macOS network service name")
	onCmd.Flags().StringVar(&mode, "mode", "full", "Sync mode: socks-only, http-https-only, full")

	offCmd := &cobra.Command{
		Use:   "off <host>",
		Short: "Disable macOS system proxy sync",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)

			svc := "Wi-Fi"
			if s != nil && s.MacNetworkService != "" {
				svc = s.MacNetworkService
			}

			macProxy := macos.NewProxy(ssh.ExecRunner{})
			if err := macProxy.ClearAll(ctx, svc); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			if s != nil {
				s.MacSyncEnabled = false
				s.MacSyncMode = state.MacSyncOff
				s.LastError = ""
				if err := state.Save(s); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
				}
			}

			fmt.Printf("macOS proxy sync disabled for %s\n", host)
			return nil
		},
	}

	syncCmd.AddCommand(onCmd, offCmd)
	return syncCmd
}
