package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
)

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <host>",
		Short: "Establish SSH master connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			master := ssh.NewMaster(ssh.ExecRunner{})
			if err := master.Connect(ctx, host); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			// Update state
			s, err := state.Load(host)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not load state: %v\n", err)
			}
			if s == nil {
				s = &state.HostState{HostAlias: host}
			}
			s.MasterConnected = true
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("Connected to %s\n", host)
			return nil
		},
	}
}

func newDisconnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disconnect <host>",
		Short: "Close SSH master connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			master := ssh.NewMaster(ssh.ExecRunner{})
			if err := master.Disconnect(ctx, host); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			// Update state
			s, err := state.Load(host)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not load state: %v\n", err)
			}
			if s != nil {
				s.MasterConnected = false
				s.SocksEnabled = false
				s.HTTPEnabled = false
				if err := state.Save(s); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
				}
			}

			fmt.Printf("Disconnected from %s\n", host)
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <host>",
		Short: "Show host connection and proxy status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			// Check live connection status
			master := ssh.NewMaster(ssh.ExecRunner{})
			connected, err := master.IsConnected(ctx, host)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not check connection: %v\n", err)
			}

			// Load persisted state
			s, err := state.Load(host)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			if s == nil {
				s = &state.HostState{HostAlias: host}
			}

			// Refresh connection state from live check
			s.MasterConnected = connected

			fmt.Printf("Host:             %s\n", s.HostAlias)
			fmt.Printf("Hostname:         %s\n", s.Hostname)
			fmt.Printf("User:             %s\n", s.User)
			fmt.Printf("Connected:        %v\n", s.MasterConnected)
			fmt.Printf("SOCKS:            enabled=%v port=%d\n", s.SocksEnabled, s.SocksPort)
			fmt.Printf("HTTP:             enabled=%v port=%d\n", s.HTTPEnabled, s.HTTPPort)
			fmt.Printf("Mac Sync:         enabled=%v mode=%s service=%s\n", s.MacSyncEnabled, s.MacSyncMode, s.MacNetworkService)
			fmt.Printf(
				"Remote Proxy:     enabled=%v http=%s socks=%s bind=%s container_http=%s container_socks=%s\n",
				s.RemoteProxyEnabled,
				s.RemoteProxyHTTPAddr,
				s.RemoteProxySOCKSAddr,
				s.RemoteProxyBindAddr,
				s.RemoteProxyExposedHTTPAddr,
				s.RemoteProxyExposedSOCKSAddr,
			)
			if s.LastError != "" {
				fmt.Printf("Last Error:       %s\n", s.LastError)
			}
			return nil
		},
	}
}
