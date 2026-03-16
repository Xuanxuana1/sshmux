package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/proxy"
	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
)

func newHTTPCmd() *cobra.Command {
	httpCmd := &cobra.Command{
		Use:   "http",
		Short: "Manage HTTP/HTTPS proxy adapter (Privoxy)",
	}

	var port int

	onCmd := &cobra.Command{
		Use:   "on <host>",
		Short: "Enable HTTP proxy on host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)
			if s == nil {
				s = &state.HostState{HostAlias: host}
			}

			if !s.SocksEnabled || s.SocksPort == 0 {
				fmt.Fprintf(os.Stderr, "Error: SOCKS proxy must be enabled first. Run: sshmux socks on %s\n", host)
				return fmt.Errorf("SOCKS proxy not enabled for %s", host)
			}

			http := proxy.NewHTTP(ssh.ExecRunner{})
			if err := http.Enable(ctx, host, port, s.SocksPort); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s.HTTPEnabled = true
			s.HTTPPort = port
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("HTTP proxy enabled on 127.0.0.1:%d for %s (forwarding to SOCKS :%d)\n", port, host, s.SocksPort)
			return nil
		},
	}
	onCmd.Flags().IntVar(&port, "port", 7897, "HTTP proxy port")

	offCmd := &cobra.Command{
		Use:   "off <host>",
		Short: "Disable HTTP proxy on host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)
			if s == nil || !s.HTTPEnabled {
				fmt.Printf("HTTP proxy is not enabled for %s\n", host)
				return nil
			}

			http := proxy.NewHTTP(ssh.ExecRunner{})
			if err := http.Disable(ctx, host); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s.HTTPEnabled = false
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("HTTP proxy disabled for %s\n", host)
			return nil
		},
	}

	var newPort int

	setPortCmd := &cobra.Command{
		Use:   "set-port <host>",
		Short: "Change HTTP proxy port",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)
			if s == nil || !s.HTTPEnabled {
				fmt.Fprintf(os.Stderr, "Error: HTTP proxy is not enabled for %s\n", host)
				return fmt.Errorf("HTTP proxy not enabled for %s", host)
			}

			oldPort := s.HTTPPort
			http := proxy.NewHTTP(ssh.ExecRunner{})
			if err := http.SetPort(ctx, host, oldPort, newPort, s.SocksPort); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s.HTTPPort = newPort
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("HTTP proxy port changed from %d to %d for %s\n", oldPort, newPort, host)
			return nil
		},
	}
	setPortCmd.Flags().IntVar(&newPort, "port", 0, "New HTTP proxy port")
	setPortCmd.MarkFlagRequired("port")

	httpCmd.AddCommand(onCmd, offCmd, setPortCmd)
	return httpCmd
}
