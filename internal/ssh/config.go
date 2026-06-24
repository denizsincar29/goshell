package ssh

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Host represents a parsed SSH config entry
type Host struct {
	Name         string
	Hostname     string
	User         string
	Port         string
	IdentityFile string
}

// ParseSSHConfig parses ~/.ssh/config and returns all Host entries
func ParseSSHConfig() ([]Host, error) {
	cfgPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	f, err := os.Open(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var hosts []Host
	var current *Host
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "host":
			if current != nil && current.Name != "*" {
				hosts = append(hosts, *current)
			}
			current = &Host{Name: val, Port: "22", User: os.Getenv("USER")}
		case "hostname":
			if current != nil {
				current.Hostname = val
			}
		case "user":
			if current != nil {
				current.User = val
			}
		case "port":
			if current != nil {
				current.Port = val
			}
		case "identityfile":
			if current != nil {
				current.IdentityFile = expandTilde(val)
			}
		}
	}
	if current != nil && current.Name != "*" {
		hosts = append(hosts, *current)
	}
	return hosts, scanner.Err()
}

// ListKeys returns available private key files from ~/.ssh/
func ListKeys() []string {
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil
	}
	var keys []string
	pubExts := map[string]bool{".pub": true}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if pubExts[ext] || name == "known_hosts" || name == "config" || name == "authorized_keys" {
			continue
		}
		// Check if a .pub counterpart exists (it's a private key)
		pubPath := filepath.Join(sshDir, name+".pub")
		if _, err := os.Stat(pubPath); err == nil {
			keys = append(keys, filepath.Join(sshDir, name))
		}
	}
	return keys
}

// ConnectParams holds everything needed to open an SSH connection
type ConnectParams struct {
	Host     string
	Port     string
	User     string
	KeyFile  string
	Password string
	SudoPass string
}

// Client wraps an SSH client with helper methods
type Client struct {
	inner    *gossh.Client
	SudoPass string
	Params   ConnectParams
}

// Connect opens an SSH connection using key auth (falling back to password)
func Connect(p ConnectParams) (*Client, error) {
	var authMethods []gossh.AuthMethod

	if p.KeyFile != "" {
		keyBytes, err := os.ReadFile(p.KeyFile)
		if err == nil {
			signer, err := gossh.ParsePrivateKey(keyBytes)
			if err == nil {
				authMethods = append(authMethods, gossh.PublicKeys(signer))
			}
		}
	}
	// Try default keys if none specified
	if len(authMethods) == 0 {
		for _, kf := range ListKeys() {
			keyBytes, err := os.ReadFile(kf)
			if err != nil {
				continue
			}
			signer, err := gossh.ParsePrivateKey(keyBytes)
			if err != nil {
				continue
			}
			authMethods = append(authMethods, gossh.PublicKeys(signer))
		}
	}
	if p.Password != "" {
		authMethods = append(authMethods, gossh.Password(p.Password))
	}

	khPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	var hostKeyCallback gossh.HostKeyCallback
	if _, err := os.Stat(khPath); err == nil {
		hostKeyCallback, _ = knownhosts.New(khPath)
	}
	if hostKeyCallback == nil {
		hostKeyCallback = gossh.InsecureIgnoreHostKey() // fallback - user can see warning
	}

	cfg := &gossh.ClientConfig{
		User:            p.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(p.Host, p.Port)
	inner, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &Client{inner: inner, SudoPass: p.SudoPass, Params: p}, nil
}

// Run runs a command and returns combined output
func (c *Client) Run(cmd string) (string, error) {
	sess, err := c.inner.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

// RunSudo runs a command with sudo, piping the password
func (c *Client) RunSudo(cmd string) (string, error) {
	fullCmd := fmt.Sprintf("echo %s | sudo -S sh -c %s",
		shellescape(c.SudoPass), shellescape(cmd))
	return c.Run(fullCmd)
}

// RunMaybeSudo runs with sudo only if useSudo is true
func (c *Client) RunMaybeSudo(cmd string, useSudo bool) (string, error) {
	if useSudo {
		return c.RunSudo(cmd)
	}
	return c.Run(cmd)
}

// Close closes the underlying SSH connection
func (c *Client) Close() error {
	return c.inner.Close()
}

func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}
