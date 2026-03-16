package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/proxy/httpserver"
)

// newProxyServerCmd returns the hidden internal subcommand that runs the
// HTTP→SOCKS5 proxy server. It is spawned by `proxy.HTTP.Enable` and is not
// intended to be called directly by users.
func newProxyServerCmd() *cobra.Command {
	var httpPort, socksPort int

	cmd := &cobra.Command{
		Use:    "_proxy-server",
		Hidden: true,
		Short:  "Internal: run built-in HTTP→SOCKS5 proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if httpPort <= 0 || socksPort <= 0 {
				return fmt.Errorf("--http-port and --socks-port are required and must be > 0")
			}
			return httpserver.Run(httpPort, socksPort)
		},
	}

	cmd.Flags().IntVar(&httpPort, "http-port", 0, "HTTP proxy listen port")
	cmd.Flags().IntVar(&socksPort, "socks-port", 0, "SOCKS5 upstream port")
	return cmd
}
