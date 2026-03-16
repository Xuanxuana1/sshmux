package state

import "time"

// MacSyncMode defines the macOS system proxy sync mode.
type MacSyncMode string

const (
	MacSyncOff       MacSyncMode = "off"
	MacSyncSOCKSOnly MacSyncMode = "socks-only"
	MacSyncHTTPOnly  MacSyncMode = "http-https-only"
	MacSyncFull      MacSyncMode = "full"
)

// HostState holds the persisted state for a single SSH host alias.
type HostState struct {
	HostAlias        string `json:"host_alias"`
	SourceConfigPath string `json:"source_config_path"`
	Hostname         string `json:"hostname"`
	User             string `json:"user"`
	Port             int    `json:"port"`
	IdentityFile     string `json:"identity_file,omitempty"`
	ProxyJump        string `json:"proxy_jump,omitempty"`
	ProxyCommand     string `json:"proxy_command,omitempty"`

	MasterConnected bool `json:"master_connected"`

	SocksEnabled bool `json:"socks_enabled"`
	SocksPort    int  `json:"socks_port"`

	HTTPEnabled bool `json:"http_enabled"`
	HTTPPort    int  `json:"http_port"`

	MacSyncEnabled    bool        `json:"mac_sync_enabled"`
	MacSyncMode       MacSyncMode `json:"mac_sync_mode"`
	MacNetworkService string      `json:"mac_network_service"`

	RemoteProxyEnabled   bool   `json:"remote_proxy_enabled"`
	RemoteProxyHTTPAddr  string `json:"remote_proxy_http_addr,omitempty"`
	RemoteProxySOCKSAddr string `json:"remote_proxy_socks_addr,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
	LastError string    `json:"last_error,omitempty"`
}

// TerminalProxyConfig holds the global terminal-proxy configuration.
type TerminalProxyConfig struct {
	Enabled   bool   `json:"enabled"`
	HTTPAddr  string `json:"http_addr"`
	SOCKSAddr string `json:"socks_addr"`
}
