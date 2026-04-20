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
	var bindAddress string
	var dockerGateway bool
	var loopbackOnly bool

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

			opts := remoteproxy.Options{
				HTTPAddr:      effectiveHTTP,
				SOCKSAddr:     effectiveSOCKS,
				BindAddress:   bindAddress,
				DockerGateway: dockerGateway,
				LoopbackOnly:  loopbackOnly,
			}

			// Load state to check for existing remote-proxy and handle port switch
			s, _ := state.Load(host)
			if s == nil {
				s = &state.HostState{HostAlias: host}
			}

			rp := remoteproxy.NewRemoteProxy(ssh.ExecRunner{})
			var activation remoteproxy.Activation

			if s.RemoteProxyEnabled {
				var err error
				activation, err = rp.SetPort(ctx, host, activationFromState(s), opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
			} else {
				var err error
				activation, err = rp.Enable(ctx, host, opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
			}

			s.RemoteProxyEnabled = true
			applyActivationToState(s, activation)
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("Remote proxy enabled for %s\n", host)
			if activation.HTTPAddr != "" {
				fmt.Printf("  Source HTTP:     %s\n", activation.HTTPAddr)
			}
			if activation.SOCKSAddr != "" {
				fmt.Printf("  Source SOCKS:    %s\n", activation.SOCKSAddr)
			}
			if activation.ShellHTTPAddr() != "" {
				fmt.Printf("  Remote shell:    %s\n", activation.ShellHTTPAddr())
			}
			if activation.BindAddress != "" {
				fmt.Printf("  Container bind:  %s\n", activation.BindAddress)
				if activation.ExposedHTTPAddr != "" {
					fmt.Printf("  Container HTTP:  %s\n", activation.ExposedHTTPAddr)
				}
				if activation.ExposedSOCKSAddr != "" {
					fmt.Printf("  Container SOCKS: %s\n", activation.ExposedSOCKSAddr)
				}
			} else {
				fmt.Printf("  Container mode:  loopback-only\n")
			}
			return nil
		},
	}
	onCmd.Flags().StringVar(&httpAddr, "http", "", "HTTP proxy address (e.g. 127.0.0.1:7897)")
	onCmd.Flags().StringVar(&socksAddr, "socks", "", "SOCKS proxy address (e.g. 127.0.0.1:7897)")
	onCmd.Flags().StringVar(&bindAddress, "bind-address", "", "Bind relay on the remote host for Docker/container access (e.g. 172.17.0.1)")
	onCmd.Flags().BoolVar(&dockerGateway, "docker-gateway", true, "Auto-detect the remote Docker bridge gateway and expose the proxy there")
	onCmd.Flags().BoolVar(&loopbackOnly, "loopback-only", false, "Keep remote-proxy bound to 127.0.0.1 only")

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
			if err := rp.Disable(ctx, host, activationFromState(s)); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s.RemoteProxyEnabled = false
			clearActivationState(s)
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
				fmt.Printf("  Source HTTP:     %s\n", s.RemoteProxyHTTPAddr)
			}
			if s.RemoteProxySOCKSAddr != "" {
				fmt.Printf("  Source SOCKS:    %s\n", s.RemoteProxySOCKSAddr)
			}
			activation := activationFromState(s)
			if activation.ShellHTTPAddr() != "" {
				fmt.Printf("  Remote shell:    %s\n", activation.ShellHTTPAddr())
			}
			if s.RemoteProxyBindAddr != "" {
				fmt.Printf("  Container bind:  %s\n", s.RemoteProxyBindAddr)
			}
			if s.RemoteProxyExposedHTTPAddr != "" {
				fmt.Printf("  Container HTTP:  %s\n", s.RemoteProxyExposedHTTPAddr)
			}
			if s.RemoteProxyExposedSOCKSAddr != "" {
				fmt.Printf("  Container SOCKS: %s\n", s.RemoteProxyExposedSOCKSAddr)
			}
			return nil
		},
	}

	rpCmd.AddCommand(onCmd, offCmd, statusCmd)
	return rpCmd
}

func activationFromState(s *state.HostState) remoteproxy.Activation {
	if s == nil {
		return remoteproxy.Activation{}
	}
	return remoteproxy.Activation{
		HTTPAddr:         s.RemoteProxyHTTPAddr,
		SOCKSAddr:        s.RemoteProxySOCKSAddr,
		BindAddress:      s.RemoteProxyBindAddr,
		ExposedHTTPAddr:  s.RemoteProxyExposedHTTPAddr,
		ExposedSOCKSAddr: s.RemoteProxyExposedSOCKSAddr,
	}
}

func applyActivationToState(s *state.HostState, activation remoteproxy.Activation) {
	s.RemoteProxyHTTPAddr = activation.HTTPAddr
	s.RemoteProxySOCKSAddr = activation.SOCKSAddr
	s.RemoteProxyBindAddr = activation.BindAddress
	s.RemoteProxyExposedHTTPAddr = activation.ExposedHTTPAddr
	s.RemoteProxyExposedSOCKSAddr = activation.ExposedSOCKSAddr
}

func clearActivationState(s *state.HostState) {
	s.RemoteProxyHTTPAddr = ""
	s.RemoteProxySOCKSAddr = ""
	s.RemoteProxyBindAddr = ""
	s.RemoteProxyExposedHTTPAddr = ""
	s.RemoteProxyExposedSOCKSAddr = ""
}
