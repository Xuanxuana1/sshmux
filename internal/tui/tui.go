package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/liuxuan/sshmux/internal/config"
	"github.com/liuxuan/sshmux/internal/macos"
	"github.com/liuxuan/sshmux/internal/proxy"
	"github.com/liuxuan/sshmux/internal/remoteproxy"
	"github.com/liuxuan/sshmux/internal/ssh"
	"github.com/liuxuan/sshmux/internal/state"
	"github.com/liuxuan/sshmux/internal/termproxy"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))

	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	selectedStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	onlineStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	offlineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	disabledStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusOKStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	statusErrStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	helpDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	borderStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// editMode tracks whether the user is in port-editing mode.
type editMode int

const (
	editNone  editMode = iota
	editPorts          // 'p' key — edit global SOCKS + HTTP ports
)

// model holds all TUI state.
type model struct {
	hosts      []state.HostState
	termProxy  *state.TerminalProxyConfig
	globalCfg  *state.GlobalConfig
	cursor     int
	statusMsg  string
	statusErr  bool
	runner     ssh.Runner
	width      int
	height     int
	editing    editMode
	portField  int // 0 = SOCKS, 1 = HTTP
	portInputs [2]string
}

// Run starts the interactive TUI.
func Run(runner ssh.Runner) error {
	hosts, err := loadHosts()
	if err != nil {
		hosts = nil
	}

	tp, err := state.LoadTerminalProxy()
	if err != nil {
		tp = &state.TerminalProxyConfig{}
	}

	gcfg, err := state.LoadGlobalConfig()
	if err != nil {
		gcfg = state.DefaultGlobalConfig()
	}

	m := model{
		hosts:     hosts,
		termProxy: tp,
		globalCfg: gcfg,
		cursor:    0,
		runner:    runner,
	}

	// Redirect slog to a log file so WARN/ERROR output doesn't leak into the
	// TUI. Alt screen isolates the display buffer, but stderr still bleeds
	// through without this redirect.
	if logFile, logErr := openLogFile(); logErr == nil {
		defer logFile.Close()
		slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// openLogFile opens (or creates) the sshmux TUI log file.
func openLogFile() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".sshmux")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "tui.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
}

func loadHosts() ([]state.HostState, error) {
	ptrs, err := state.ListHosts()
	if err != nil {
		return nil, err
	}
	hosts := make([]state.HostState, 0, len(ptrs))
	for _, h := range ptrs {
		if h != nil {
			hosts = append(hosts, *h)
		}
	}
	return hosts, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// In edit mode, all keys are handled by the port editor.
		if m.editing != editNone {
			return m.handleEditKey(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.hosts)-1 {
				m.cursor++
			}
			return m, nil

		case "c":
			return m.toggleSSH()

		case "p":
			return m.startEditPorts()

		case "m":
			return m.toggleMacSync()

		case "r":
			return m.toggleRemoteProxy()

		case "i":
			return m.importHosts()

		case "t":
			return m.toggleTerminalProxy()
		}
	}

	return m, nil
}

func (m model) startEditPorts() (tea.Model, tea.Cmd) {
	m.editing = editPorts
	m.portField = 0
	m.portInputs[0] = fmt.Sprintf("%d", m.globalCfg.SocksPort)
	m.portInputs[1] = fmt.Sprintf("%d", m.globalCfg.HTTPPort)
	m.statusMsg = ""
	return m, nil
}

func (m model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = editNone
		return m, nil
	case "enter":
		return m.confirmPorts()
	case "tab":
		m.portField = 1 - m.portField // toggle between 0 and 1
		return m, nil
	case "backspace":
		if len(m.portInputs[m.portField]) > 0 {
			m.portInputs[m.portField] = m.portInputs[m.portField][:len(m.portInputs[m.portField])-1]
		}
		return m, nil
	default:
		s := msg.String()
		if len(s) == 1 && s >= "0" && s <= "9" && len(m.portInputs[m.portField]) < 5 {
			m.portInputs[m.portField] += s
		}
		return m, nil
	}
}

func (m model) confirmPorts() (tea.Model, tea.Cmd) {
	m.editing = editNone
	ctx := context.Background()
	h := m.selectedHost()

	socksPort, err := strconv.Atoi(m.portInputs[0])
	if err != nil || socksPort <= 0 || socksPort > 65535 {
		m.statusMsg = fmt.Sprintf("invalid SOCKS port: %q", m.portInputs[0])
		m.statusErr = true
		return m, nil
	}
	httpPort, err := strconv.Atoi(m.portInputs[1])
	if err != nil || httpPort <= 0 || httpPort > 65535 {
		m.statusMsg = fmt.Sprintf("invalid HTTP port: %q", m.portInputs[1])
		m.statusErr = true
		return m, nil
	}

	oldSocks := m.globalCfg.SocksPort
	oldHTTP := m.globalCfg.HTTPPort
	m.globalCfg.SocksPort = socksPort
	m.globalCfg.HTTPPort = httpPort
	if err := state.SaveGlobalConfig(m.globalCfg); err != nil {
		m.statusMsg = fmt.Sprintf("save global config failed: %v", err)
		m.statusErr = true
		return m, nil
	}

	// Hot-switch selected host's running proxies if ports changed.
	if h != nil {
		if h.SocksEnabled && oldSocks != socksPort {
			socks := proxy.NewSOCKS(m.runner)
			if err := socks.SetPort(ctx, h.HostAlias, oldSocks, socksPort); err != nil {
				m.statusMsg = fmt.Sprintf("SOCKS port switch failed: %v", err)
				m.statusErr = true
				return m, nil
			}
			h.SocksPort = socksPort
			_ = state.Save(h)
			m.hosts[m.cursor] = *h
		}
		if h.HTTPEnabled && oldHTTP != httpPort {
			httpProxy := proxy.NewHTTP(m.runner)
			if err := httpProxy.SetPort(ctx, h.HostAlias, oldHTTP, httpPort, socksPort); err != nil {
				m.statusMsg = fmt.Sprintf("HTTP port switch failed: %v", err)
				m.statusErr = true
				return m, nil
			}
			h.HTTPPort = httpPort
			_ = state.Save(h)
			m.hosts[m.cursor] = *h
		}
	}

	m.statusMsg = fmt.Sprintf("Ports updated — SOCKS :%d  HTTP :%d", socksPort, httpPort)
	m.statusErr = false
	return m, nil
}

func (m model) selectedHost() *state.HostState {
	if len(m.hosts) == 0 || m.cursor < 0 || m.cursor >= len(m.hosts) {
		return nil
	}
	return &m.hosts[m.cursor]
}

func (m model) toggleSSH() (tea.Model, tea.Cmd) {
	h := m.selectedHost()
	if h == nil {
		return m, nil
	}
	ctx := context.Background()
	master := ssh.NewMaster(m.runner)

	if h.MasterConnected {
		if err := m.disableLocalProxies(ctx, h); err != nil {
			m.statusMsg = fmt.Sprintf("disable local proxies failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		if err := master.Disconnect(ctx, h.HostAlias); err != nil {
			m.statusMsg = fmt.Sprintf("disconnect failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.MasterConnected = false
		h.SocksEnabled = false
		h.HTTPEnabled = false
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("SSH disconnected from %s", h.HostAlias)
		m.statusErr = false
	} else {
		if err := master.Connect(ctx, h.HostAlias); err != nil {
			m.statusMsg = fmt.Sprintf("connect failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.MasterConnected = true
		if err := m.enableLocalProxies(ctx, h); err != nil {
			h.LastError = err.Error()
			_ = state.Save(h)
			m.hosts[m.cursor] = *h
			m.statusMsg = fmt.Sprintf("SSH connected but local proxies failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("SSH connected to %s", h.HostAlias)
		m.statusErr = false
	}
	return m, nil
}

func (m model) enableLocalProxies(ctx context.Context, h *state.HostState) error {
	socksPort := m.globalCfg.SocksPort
	httpPort := m.globalCfg.HTTPPort

	if h.SocksEnabled && h.SocksPort != 0 && h.SocksPort != socksPort {
		socks := proxy.NewSOCKS(m.runner)
		if err := socks.SetPort(ctx, h.HostAlias, h.SocksPort, socksPort); err != nil {
			return fmt.Errorf("switch SOCKS port: %w", err)
		}
	}
	if !h.SocksEnabled || h.SocksPort != socksPort {
		socks := proxy.NewSOCKS(m.runner)
		if err := socks.Enable(ctx, h.HostAlias, socksPort); err != nil {
			return fmt.Errorf("enable SOCKS: %w", err)
		}
	}
	h.SocksEnabled = true
	h.SocksPort = socksPort

	if h.HTTPEnabled && h.HTTPPort != 0 && h.HTTPPort != httpPort {
		httpProxy := proxy.NewHTTP(m.runner)
		if err := httpProxy.SetPort(ctx, h.HostAlias, h.HTTPPort, httpPort, socksPort); err != nil {
			return fmt.Errorf("switch HTTP port: %w", err)
		}
	}
	if !h.HTTPEnabled || h.HTTPPort != httpPort {
		httpProxy := proxy.NewHTTP(m.runner)
		if err := httpProxy.Enable(ctx, h.HostAlias, httpPort, socksPort); err != nil {
			return fmt.Errorf("enable HTTP: %w", err)
		}
	}
	h.HTTPEnabled = true
	h.HTTPPort = httpPort
	return nil
}

func (m model) disableLocalProxies(ctx context.Context, h *state.HostState) error {
	if h.HTTPEnabled {
		httpProxy := proxy.NewHTTP(m.runner)
		if err := httpProxy.Disable(ctx, h.HostAlias); err != nil {
			return fmt.Errorf("disable HTTP: %w", err)
		}
	}
	if h.SocksEnabled {
		socks := proxy.NewSOCKS(m.runner)
		if err := socks.Disable(ctx, h.HostAlias, h.SocksPort); err != nil {
			return fmt.Errorf("disable SOCKS: %w", err)
		}
	}
	h.SocksEnabled = false
	h.SocksPort = 0
	h.HTTPEnabled = false
	h.HTTPPort = 0
	return nil
}

func (m model) toggleSOCKS() (tea.Model, tea.Cmd) {
	h := m.selectedHost()
	if h == nil {
		return m, nil
	}
	ctx := context.Background()
	socks := proxy.NewSOCKS(m.runner)

	port := m.globalCfg.SocksPort

	if h.SocksEnabled {
		if err := socks.Disable(ctx, h.HostAlias, h.SocksPort); err != nil {
			m.statusMsg = fmt.Sprintf("SOCKS disable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.SocksEnabled = false
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("SOCKS disabled on %s", h.HostAlias)
		m.statusErr = false
	} else {
		if err := socks.Enable(ctx, h.HostAlias, port); err != nil {
			m.statusMsg = fmt.Sprintf("SOCKS enable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.SocksEnabled = true
		h.SocksPort = port
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("SOCKS enabled on %s  :%d", h.HostAlias, port)
		m.statusErr = false
	}
	return m, nil
}

func (m model) toggleHTTP() (tea.Model, tea.Cmd) {
	h := m.selectedHost()
	if h == nil {
		return m, nil
	}
	ctx := context.Background()
	httpProxy := proxy.NewHTTP(m.runner)

	port := m.globalCfg.HTTPPort

	if h.HTTPEnabled {
		if err := httpProxy.Disable(ctx, h.HostAlias); err != nil {
			m.statusMsg = fmt.Sprintf("HTTP disable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.HTTPEnabled = false
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("HTTP disabled on %s", h.HostAlias)
		m.statusErr = false
	} else {
		if !h.SocksEnabled || h.SocksPort == 0 {
			m.statusMsg = fmt.Sprintf("SOCKS must be enabled first on %s", h.HostAlias)
			m.statusErr = true
			return m, nil
		}
		if err := httpProxy.Enable(ctx, h.HostAlias, port, h.SocksPort); err != nil {
			m.statusMsg = fmt.Sprintf("HTTP enable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.HTTPEnabled = true
		h.HTTPPort = port
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("HTTP enabled on %s  :%d", h.HostAlias, port)
		m.statusErr = false
	}
	return m, nil
}

func (m model) toggleMacSync() (tea.Model, tea.Cmd) {
	h := m.selectedHost()
	if h == nil {
		return m, nil
	}
	ctx := context.Background()
	macProxy := macos.NewProxy(m.runner)

	if h.MacSyncEnabled {
		svc := h.MacNetworkService
		if svc == "" {
			svc = "Wi-Fi"
		}
		if err := macProxy.ClearAll(ctx, svc); err != nil {
			m.statusMsg = fmt.Sprintf("macOS sync disable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.MacSyncEnabled = false
		h.MacSyncMode = state.MacSyncOff
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("macOS sync disabled on %s", h.HostAlias)
		m.statusErr = false
	} else {
		if !h.MasterConnected {
			m.statusMsg = fmt.Sprintf("SSH not connected — press [c] to connect %s first", h.HostAlias)
			m.statusErr = true
			return m, nil
		}
		if err := m.enableLocalProxies(ctx, h); err != nil {
			m.statusMsg = fmt.Sprintf("enable local proxies failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		svc := h.MacNetworkService
		if svc == "" {
			svc = "Wi-Fi"
		}
		if err := macProxy.SetSOCKS(ctx, svc, "127.0.0.1", h.SocksPort); err != nil {
			m.statusMsg = fmt.Sprintf("macOS sync enable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.MacSyncEnabled = true
		h.MacSyncMode = state.MacSyncSOCKSOnly
		h.MacNetworkService = svc
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("macOS sync enabled on %s", h.HostAlias)
		m.statusErr = false
	}
	return m, nil
}

func (m model) toggleRemoteProxy() (tea.Model, tea.Cmd) {
	h := m.selectedHost()
	if h == nil {
		return m, nil
	}
	ctx := context.Background()
	rp := remoteproxy.NewRemoteProxy(m.runner)

	if h.RemoteProxyEnabled {
		if err := rp.Disable(ctx, h.HostAlias, h.RemoteProxyHTTPAddr, h.RemoteProxySOCKSAddr); err != nil {
			m.statusMsg = fmt.Sprintf("remote-proxy disable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.RemoteProxyEnabled = false
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("remote-proxy disabled on %s", h.HostAlias)
		m.statusErr = false
	} else {
		if !h.MasterConnected {
			m.statusMsg = fmt.Sprintf("SSH not connected — press [c] to connect %s first", h.HostAlias)
			m.statusErr = true
			return m, nil
		}
		httpAddr, socksAddr := remoteproxy.ResolveAddrs("", "")
		if httpAddr == "" && socksAddr == "" {
			m.statusMsg = "no proxy addresses configured; enable terminal-proxy first"
			m.statusErr = true
			return m, nil
		}
		if err := rp.Enable(ctx, h.HostAlias, httpAddr, socksAddr); err != nil {
			m.statusMsg = fmt.Sprintf("remote-proxy enable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		h.RemoteProxyEnabled = true
		h.RemoteProxyHTTPAddr = httpAddr
		h.RemoteProxySOCKSAddr = socksAddr
		h.LastError = ""
		_ = state.Save(h)
		m.hosts[m.cursor] = *h
		m.statusMsg = fmt.Sprintf("remote-proxy enabled on %s", h.HostAlias)
		m.statusErr = false
	}
	return m, nil
}

func (m model) importHosts() (tea.Model, tea.Cmd) {
	path, err := config.DefaultConfigPath()
	if err != nil {
		m.statusMsg = fmt.Sprintf("import failed: %v", err)
		m.statusErr = true
		return m, nil
	}

	entries, err := config.ParseFile(path)
	if err != nil {
		m.statusMsg = fmt.Sprintf("import failed: %v", err)
		m.statusErr = true
		return m, nil
	}

	imported := 0
	for _, e := range entries {
		port := 0
		if e.Port != "" {
			p, parseErr := strconv.Atoi(e.Port)
			if parseErr == nil {
				port = p
			}
		}

		existing, loadErr := state.Load(e.Alias)
		if loadErr != nil {
			continue
		}

		if existing != nil {
			existing.Hostname = e.Hostname
			existing.User = e.User
			existing.Port = port
			existing.IdentityFile = e.IdentityFile
			existing.ProxyJump = e.ProxyJump
			existing.ProxyCommand = e.ProxyCommand
			existing.SourceConfigPath = e.SourceFile
			_ = state.Save(existing)
		} else {
			s := &state.HostState{
				HostAlias:        e.Alias,
				SourceConfigPath: e.SourceFile,
				Hostname:         e.Hostname,
				User:             e.User,
				Port:             port,
				IdentityFile:     e.IdentityFile,
				ProxyJump:        e.ProxyJump,
				ProxyCommand:     e.ProxyCommand,
			}
			_ = state.Save(s)
		}
		imported++
	}

	// Reload hosts from disk
	hosts, err := loadHosts()
	if err != nil {
		m.statusMsg = fmt.Sprintf("import succeeded but reload failed: %v", err)
		m.statusErr = true
		return m, nil
	}
	m.hosts = hosts
	if m.cursor >= len(m.hosts) && len(m.hosts) > 0 {
		m.cursor = len(m.hosts) - 1
	}
	m.statusMsg = fmt.Sprintf("Imported %d hosts from %s", imported, path)
	m.statusErr = false
	return m, nil
}

func (m model) toggleTerminalProxy() (tea.Model, tea.Cmd) {
	tp, err := state.LoadTerminalProxy()
	if err != nil {
		m.statusMsg = fmt.Sprintf("terminal-proxy toggle failed: %v", err)
		m.statusErr = true
		return m, nil
	}

	if tp.Enabled {
		if err := termproxy.Disable(); err != nil {
			m.statusMsg = fmt.Sprintf("terminal-proxy disable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		tp.Enabled = false
		m.termProxy = tp
		m.statusMsg = "Terminal proxy disabled"
		m.statusErr = false
	} else {
		if tp.HTTPAddr == "" && tp.SOCKSAddr == "" {
			m.statusMsg = "terminal-proxy has no addresses configured; use CLI to set --http/--socks first"
			m.statusErr = true
			return m, nil
		}
		if err := termproxy.Enable(tp); err != nil {
			m.statusMsg = fmt.Sprintf("terminal-proxy enable failed: %v", err)
			m.statusErr = true
			return m, nil
		}
		m.termProxy = tp
		m.statusMsg = "Terminal proxy enabled"
		m.statusErr = false
	}
	return m, nil
}

// View renders the TUI.
func (m model) View() string {
	var b strings.Builder

	// Title
	b.WriteString("\n  ")
	b.WriteString(titleStyle.Render("sshmux -- SSH Proxy Manager"))
	b.WriteString("\n\n")

	if len(m.hosts) == 0 {
		b.WriteString("  No hosts found. Press [i] to import from ~/.ssh/config\n")
	} else {
		b.WriteString(m.renderTable())
	}

	b.WriteString("\n")

	// Shortcuts
	b.WriteString("  ")
	b.WriteString(renderHelp("[c]", "SSH"))
	b.WriteString("  ")
	b.WriteString(renderHelp("[m]", "macOS sync"))
	b.WriteString("  ")
	b.WriteString(renderHelp("[r]", "remote-proxy"))
	b.WriteString("  ")
	b.WriteString(renderHelp("[p]", "ports"))
	b.WriteString("  ")
	b.WriteString(renderHelp("[i]", "import"))
	b.WriteString("\n")

	// Separator
	b.WriteString("  ")
	b.WriteString(dimStyle.Render(strings.Repeat("-", 72)))
	b.WriteString("\n")

	// Global proxy ports line
	b.WriteString("  Proxy Ports  ")
	b.WriteString(dimStyle.Render("SOCKS"))
	b.WriteString(" " + helpKeyStyle.Render(fmt.Sprintf(":%d", m.globalCfg.SocksPort)))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("HTTP"))
	b.WriteString(" " + helpKeyStyle.Render(fmt.Sprintf(":%d", m.globalCfg.HTTPPort)))
	b.WriteString("   ")
	b.WriteString(renderHelp("[p]", "edit ports"))
	b.WriteString("\n")

	// Terminal proxy status line
	b.WriteString("  Terminal Proxy  ")
	if m.termProxy != nil && m.termProxy.Enabled {
		b.WriteString(onlineStyle.Render("ON"))
		if m.termProxy.HTTPAddr != "" {
			b.WriteString("  http=" + m.termProxy.HTTPAddr)
		}
		if m.termProxy.SOCKSAddr != "" {
			b.WriteString("  socks=" + m.termProxy.SOCKSAddr)
		}
	} else {
		b.WriteString(offlineStyle.Render("OFF"))
	}
	b.WriteString("   ")
	b.WriteString(renderHelp("[t]", "toggle"))
	b.WriteString("\n")

	// Navigation help
	b.WriteString("  ")
	b.WriteString(renderHelp("[↑/↓] or [j/k]", "to navigate"))
	b.WriteString("   ")
	b.WriteString(renderHelp("[q]", "quit"))
	b.WriteString("\n")

	// Port editor panel (p key)
	if m.editing == editPorts {
		b.WriteString("\n")
		for i, label := range []string{"SOCKS", "HTTP "} {
			b.WriteString("  ")
			b.WriteString(dimStyle.Render(label + ":"))
			b.WriteString("  ")
			if m.portField == i {
				b.WriteString(helpKeyStyle.Render(m.portInputs[i]))
				b.WriteString(cursorStyle.Render("_"))
			} else {
				b.WriteString(m.portInputs[i])
			}
			b.WriteString("\n")
		}
		b.WriteString("  ")
		b.WriteString(dimStyle.Render("[tab] switch field  [enter] confirm  [esc] cancel"))
		b.WriteString("\n")
	}

	// Status message (word-wrapped to terminal width)
	if m.statusMsg != "" {
		maxW := m.width - 4
		if maxW < 40 {
			maxW = 40
		}
		b.WriteString("\n  ")
		if m.statusErr {
			b.WriteString(statusErrStyle.Width(maxW).Render(m.statusMsg))
		} else {
			b.WriteString(statusOKStyle.Width(maxW).Render(m.statusMsg))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderHelp(key, desc string) string {
	return helpKeyStyle.Render(key) + " " + helpDescStyle.Render(desc)
}

// Column widths for the table.
const (
	colHost = 18
	colSSH  = 12
	colSync = 8
	colRPx  = 7
)

func (m model) renderTable() string {
	var b strings.Builder

	// Header separator (top border)
	b.WriteString("  ")
	b.WriteString(borderStyle.Render(
		"+" + strings.Repeat("-", colHost+1) +
			"+" + strings.Repeat("-", colSSH+1) +
			"+" + strings.Repeat("-", colSync+1) +
			"+" + strings.Repeat("-", colRPx+1) + "+"))
	b.WriteString("\n")

	// Header row
	b.WriteString("  ")
	b.WriteString(borderStyle.Render("|"))
	b.WriteString(" ")
	b.WriteString(headerStyle.Render(pad("Host", colHost)))
	b.WriteString(borderStyle.Render("|"))
	b.WriteString(" ")
	b.WriteString(headerStyle.Render(pad("SSH", colSSH)))
	b.WriteString(borderStyle.Render("|"))
	b.WriteString(" ")
	b.WriteString(headerStyle.Render(pad("Sync", colSync)))
	b.WriteString(borderStyle.Render("|"))
	b.WriteString(" ")
	b.WriteString(headerStyle.Render(pad("RPx", colRPx)))
	b.WriteString(borderStyle.Render("|"))
	b.WriteString("\n")

	// Header separator (middle border)
	b.WriteString("  ")
	b.WriteString(borderStyle.Render(
		"+" + strings.Repeat("-", colHost+1) +
			"+" + strings.Repeat("-", colSSH+1) +
			"+" + strings.Repeat("-", colSync+1) +
			"+" + strings.Repeat("-", colRPx+1) + "+"))
	b.WriteString("\n")

	// Rows
	for i, h := range m.hosts {
		isSelected := i == m.cursor

		// Cursor indicator
		cursor := "  "
		if isSelected {
			cursor = cursorStyle.Render("> ")
		}

		// Host name (truncate if needed)
		hostName := h.HostAlias
		if len(hostName) > colHost-1 {
			hostName = hostName[:colHost-4] + "..."
		}

		// SSH status
		var sshStatus string
		if h.MasterConnected {
			sshStatus = onlineStyle.Render("*") + " online"
		} else {
			sshStatus = offlineStyle.Render("o") + " offline"
		}

		// Mac sync status
		var syncStatus string
		if h.MacSyncEnabled {
			syncStatus = onlineStyle.Render("*")
		} else {
			syncStatus = disabledStyle.Render("x")
		}

		// Remote proxy status
		var rpxStatus string
		if h.RemoteProxyEnabled {
			rpxStatus = onlineStyle.Render("*")
		} else {
			rpxStatus = disabledStyle.Render("x")
		}

		row := cursor
		row += borderStyle.Render("|")
		row += " " + pad(hostName, colHost)
		row += borderStyle.Render("|")
		row += " " + pad(sshStatus, colSSH)
		row += borderStyle.Render("|")
		row += " " + pad(syncStatus, colSync)
		row += borderStyle.Render("|")
		row += " " + pad(rpxStatus, colRPx)
		row += borderStyle.Render("|")

		if isSelected {
			// Apply selected background - we render the entire row content
			// but the background highlight is applied via the selected style
			b.WriteString(selectedStyle.Render(row))
		} else {
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	// Bottom border
	b.WriteString("  ")
	b.WriteString(borderStyle.Render(
		"+" + strings.Repeat("-", colHost+1) +
			"+" + strings.Repeat("-", colSSH+1) +
			"+" + strings.Repeat("-", colSync+1) +
			"+" + strings.Repeat("-", colRPx+1) + "+"))
	b.WriteString("\n")

	return b.String()
}

// pad right-pads a string to the given visual width.
// It accounts for ANSI escape sequences by measuring visible width.
func pad(s string, width int) string {
	// Count visible characters (strip ANSI escape sequences)
	visible := visibleLen(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// visibleLen returns the visible length of a string, stripping ANSI escapes.
func visibleLen(s string) int {
	length := 0
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		length++
	}
	return length
}
