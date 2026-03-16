package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/remoteproxy"
	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
)

func newRemoteProxyCmd() *cobra.Command {
	rpCmd := &cobra.Command{
		Use:   "remote-proxy",
		Short: "Manage remote server proxy forwarding via SSH reverse port forward",
	}

	var httpAddr string
	var socksAddr string

	onCmd := &cobra.Command{
		Use:   "on <host>",
		Short: "Enable remote proxy forwarding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			// Check SSH connection
			master := ssh.NewMaster(ssh.ExecRunner{})
			connected, err := master.IsConnected(ctx, host)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not check connection: %v\n", err)
			}
			if !connected {
				fmt.Fprintf(os.Stderr, "Error: not connected to %s. Run: sshmux connect %s\n", host, host)
				return fmt.Errorf("SSH master not connected to %s", host)
			}

			// Resolve addresses (fall back to terminal-proxy config)
			effectiveHTTP := httpAddr
			effectiveSOCKS := socksAddr
			if !cmd.Flags().Changed("http") && !cmd.Flags().Changed("socks") {
				effectiveHTTP, effectiveSOCKS = remoteproxy.ResolveAddrs(effectiveHTTP, effectiveSOCKS)
			} else {
				if cmd.Flags().Changed("http") {
					effectiveHTTP = httpAddr
				}
				if cmd.Flags().Changed("socks") {
					effectiveSOCKS = socksAddr
				}
			}

			if effectiveHTTP == "" && effectiveSOCKS == "" {
				fmt.Fprintf(os.Stderr, "Error: at least one of --http or --socks must be specified\n")
				return fmt.Errorf("no proxy address specified")
			}

			// Load state to check for existing remote-proxy and handle port switch
			s, _ := state.Load(host)
			if s == nil {
				s = &state.HostState{HostAlias: host}
			}

			rp := remoteproxy.NewRemoteProxy(ssh.ExecRunner{})

			if s.RemoteProxyEnabled {
				// Port switch: disable old, enable new
				if err := rp.SetPort(ctx, host,
					s.RemoteProxyHTTPAddr, s.RemoteProxySOCKSAddr,
					effectiveHTTP, effectiveSOCKS,
				); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
			} else {
				if err := rp.Enable(ctx, host, effectiveHTTP, effectiveSOCKS); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
			}

			s.RemoteProxyEnabled = true
			s.RemoteProxyHTTPAddr = effectiveHTTP
			s.RemoteProxySOCKSAddr = effectiveSOCKS
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("Remote proxy enabled for %s\n", host)
			if effectiveHTTP != "" {
				fmt.Printf("  HTTP:  %s\n", effectiveHTTP)
			}
			if effectiveSOCKS != "" {
				fmt.Printf("  SOCKS: %s\n", effectiveSOCKS)
			}
			return nil
		},
	}
	onCmd.Flags().StringVar(&httpAddr, "http", "", "HTTP proxy address (e.g. 127.0.0.1:7897)")
	onCmd.Flags().StringVar(&socksAddr, "socks", "", "SOCKS proxy address (e.g. 127.0.0.1:7897)")

	offCmd := &cobra.Command{
		Use:   "off <host>",
		Short: "Disable remote proxy forwarding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)
			if s == nil || !s.RemoteProxyEnabled {
				fmt.Printf("Remote proxy is not enabled for %s\n", host)
				return nil
			}

			rp := remoteproxy.NewRemoteProxy(ssh.ExecRunner{})
			if err := rp.Disable(ctx, host, s.RemoteProxyHTTPAddr, s.RemoteProxySOCKSAddr); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s.RemoteProxyEnabled = false
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("Remote proxy disabled for %s\n", host)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status <host>",
		Short: "Show remote proxy status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]

			s, err := state.Load(host)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			if s == nil {
				fmt.Printf("No state found for %s\n", host)
				return nil
			}

			if s.RemoteProxyEnabled {
				fmt.Printf("Remote proxy for %s: enabled\n", host)
			} else {
				fmt.Printf("Remote proxy for %s: disabled\n", host)
			}
			if s.RemoteProxyHTTPAddr != "" {
				fmt.Printf("  HTTP:  %s\n", s.RemoteProxyHTTPAddr)
			}
			if s.RemoteProxySOCKSAddr != "" {
				fmt.Printf("  SOCKS: %s\n", s.RemoteProxySOCKSAddr)
			}
			return nil
		},
	}

	rpCmd.AddCommand(onCmd, offCmd, statusCmd)
	return rpCmd
}
