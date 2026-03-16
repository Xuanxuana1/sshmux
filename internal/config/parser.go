package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HostEntry represents a parsed SSH config Host block.
type HostEntry struct {
	Alias        string
	Hostname     string
	User         string
	Port         string
	IdentityFile string
	ProxyJump    string
	ProxyCommand string
	SourceFile   string
}

// ParseFile parses an OpenSSH config file and returns concrete host entries.
// Wildcard/template hosts (containing * or ?) are skipped.
// Include directives are resolved recursively.
func ParseFile(path string) ([]HostEntry, error) {
	path, err := expandPath(path)
	if err != nil {
		return nil, fmt.Errorf("expand path %s: %w", path, err)
	}
	return parseFileRecursive(path, 0)
}

const maxIncludeDepth = 10

func parseFileRecursive(path string, depth int) ([]HostEntry, error) {
	if depth > maxIncludeDepth {
		return nil, fmt.Errorf("include depth exceeded at %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ssh config %s: %w", path, err)
	}
	defer f.Close()

	var hosts []HostEntry
	var current *HostEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value := splitKeyValue(line)
		if key == "" {
			continue
		}

		switch strings.ToLower(key) {
		case "host":
			// Save previous entry if it exists and is concrete
			if current != nil && !isWildcard(current.Alias) {
				hosts = append(hosts, *current)
			}
			current = &HostEntry{
				Alias:      value,
				SourceFile: path,
			}
		case "hostname":
			if current != nil {
				current.Hostname = value
			}
		case "user":
			if current != nil {
				current.User = value
			}
		case "port":
			if current != nil {
				current.Port = value
			}
		case "identityfile":
			if current != nil {
				current.IdentityFile = value
			}
		case "proxyjump":
			if current != nil {
				current.ProxyJump = value
			}
		case "proxycommand":
			if current != nil {
				current.ProxyCommand = value
			}
		case "include":
			// Resolve Include directive
			included, err := resolveInclude(value, path, depth)
			if err != nil {
				return nil, fmt.Errorf("resolve include %s: %w", value, err)
			}
			hosts = append(hosts, included...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ssh config %s: %w", path, err)
	}

	// Save the last entry
	if current != nil && !isWildcard(current.Alias) {
		hosts = append(hosts, *current)
	}

	return hosts, nil
}

// splitKeyValue splits a line into key and value, handling both "Key Value"
// and "Key=Value" formats. Leading/trailing whitespace is trimmed.
func splitKeyValue(line string) (string, string) {
	line = strings.TrimSpace(line)
	// Handle Key=Value
	if idx := strings.IndexByte(line, '='); idx > 0 {
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		return key, val
	}
	// Handle Key Value (split on first whitespace)
	for i, c := range line {
		if c == ' ' || c == '\t' {
			return line[:i], strings.TrimSpace(line[i+1:])
		}
	}
	return line, ""
}

// isWildcard returns true if the alias contains wildcard characters.
func isWildcard(alias string) bool {
	return strings.ContainsAny(alias, "*?")
}

// resolveInclude expands an Include directive value to a list of HostEntries.
func resolveInclude(pattern string, parentFile string, depth int) ([]HostEntry, error) {
	pattern = strings.TrimSpace(pattern)

	// Expand ~ prefix
	if strings.HasPrefix(pattern, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		pattern = filepath.Join(home, pattern[1:])
	} else if !filepath.IsAbs(pattern) {
		// Relative to the parent config file's directory
		pattern = filepath.Join(filepath.Dir(parentFile), pattern)
	}

	// Glob to expand wildcards in Include paths
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob include pattern %s: %w", pattern, err)
	}

	var allHosts []HostEntry
	for _, match := range matches {
		entries, err := parseFileRecursive(match, depth+1)
		if err != nil {
			return nil, fmt.Errorf("parse included file %s: %w", match, err)
		}
		allHosts = append(allHosts, entries...)
	}
	return allHosts, nil
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}

// DefaultConfigPath returns the default SSH config path (~/.ssh/config).
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "config"), nil
}
