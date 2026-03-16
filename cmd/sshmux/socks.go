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

func newSocksCmd() *cobra.Command {
	socksCmd := &cobra.Command{
		Use:   "socks",
		Short: "Manage SOCKS proxy via SSH DynamicForward",
	}

	var port int

	onCmd := &cobra.Command{
		Use:   "on <host>",
		Short: "Enable SOCKS proxy on host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			socks := proxy.NewSOCKS(ssh.ExecRunner{})
			if err := socks.Enable(ctx, host, port); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s, _ := state.Load(host)
			if s == nil {
				s = &state.HostState{HostAlias: host}
			}
			s.SocksEnabled = true
			s.SocksPort = port
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("SOCKS proxy enabled on 127.0.0.1:%d for %s\n", port, host)
			return nil
		},
	}
	onCmd.Flags().IntVar(&port, "port", 1087, "SOCKS proxy port")

	offCmd := &cobra.Command{
		Use:   "off <host>",
		Short: "Disable SOCKS proxy on host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)
			if s == nil || !s.SocksEnabled {
				fmt.Printf("SOCKS proxy is not enabled for %s\n", host)
				return nil
			}

			socks := proxy.NewSOCKS(ssh.ExecRunner{})
			if err := socks.Disable(ctx, host, s.SocksPort); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s.SocksEnabled = false
			s.LastError = ""
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("SOCKS proxy disabled for %s\n", host)
			return nil
		},
	}

	var newPort int

	setPortCmd := &cobra.Command{
		Use:   "set-port <host>",
		Short: "Change SOCKS proxy port",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			ctx := context.Background()

			s, _ := state.Load(host)
			if s == nil || !s.SocksEnabled {
				fmt.Fprintf(os.Stderr, "Error: SOCKS proxy is not enabled for %s\n", host)
				return fmt.Errorf("SOCKS proxy not enabled for %s", host)
			}

			oldPort := s.SocksPort
			socks := proxy.NewSOCKS(ssh.ExecRunner{})
			if err := socks.SetPort(ctx, host, oldPort, newPort); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			s.SocksPort = newPort
			if err := state.Save(s); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
			}

			fmt.Printf("SOCKS proxy port changed from %d to %d for %s\n", oldPort, newPort, host)
			return nil
		},
	}
	setPortCmd.Flags().IntVar(&newPort, "port", 0, "New SOCKS proxy port")
	setPortCmd.MarkFlagRequired("port")

	socksCmd.AddCommand(onCmd, offCmd, setPortCmd)
	return socksCmd
}
