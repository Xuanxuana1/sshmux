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

	RemoteProxyEnabled          bool   `json:"remote_proxy_enabled"`
	RemoteProxyHTTPAddr         string `json:"remote_proxy_http_addr,omitempty"`
	RemoteProxySOCKSAddr        string `json:"remote_proxy_socks_addr,omitempty"`
	RemoteProxyBindAddr         string `json:"remote_proxy_bind_addr,omitempty"`
	RemoteProxyExposedHTTPAddr  string `json:"remote_proxy_exposed_http_addr,omitempty"`
	RemoteProxyExposedSOCKSAddr string `json:"remote_proxy_exposed_socks_addr,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
	LastError string    `json:"last_error,omitempty"`
}

// TerminalProxyConfig holds the global terminal-proxy configuration.
type TerminalProxyConfig struct {
	Enabled   bool   `json:"enabled"`
	HTTPAddr  string `json:"http_addr"`
	SOCKSAddr string `json:"socks_addr"`
}

// RemoteSourceMode defines which local proxy source remote-proxy should use.
type RemoteSourceMode string

const (
	RemoteSourceSSHMux   RemoteSourceMode = "sshmux"
	RemoteSourceExternal RemoteSourceMode = "external"
)

// GlobalConfig holds settings shared across all hosts.
type GlobalConfig struct {
	SocksPort         int              `json:"socks_port"`
	HTTPPort          int              `json:"http_port"`
	RemoteSource      RemoteSourceMode `json:"remote_source"`
	ExternalHTTPAddr  string           `json:"external_http_addr,omitempty"`
	ExternalSOCKSAddr string           `json:"external_socks_addr,omitempty"`
}

// DefaultGlobalConfig returns sensible defaults.
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		SocksPort:         7897,
		HTTPPort:          7897,
		RemoteSource:      RemoteSourceSSHMux,
		ExternalHTTPAddr:  "127.0.0.1:7897",
		ExternalSOCKSAddr: "127.0.0.1:7897",
	}
}
