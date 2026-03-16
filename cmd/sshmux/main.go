package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/tui"
)

var debug bool

func initLogger(debug bool) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{AddSource: debug}
	if debug {
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		opts.Level = slog.LevelWarn
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "sshmux",
		Short: "macOS local SSH proxy manager",
		Long:  "sshmux manages persistent SSH connections with SOCKS/HTTP proxy, macOS system proxy sync, terminal proxy, and remote proxy forwarding.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			initLogger(debug)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(ssh.ExecRunner{})
		},
	}

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")

	// Register all subcommands
	rootCmd.AddCommand(
		newImportHostsCmd(),
		newHostsCmd(),
		newConnectCmd(),
		newDisconnectCmd(),
		newStatusCmd(),
		newSocksCmd(),
		newHTTPCmd(),
		newSyncCmd(),
		newTerminalProxyCmd(),
		newRemoteProxyCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
