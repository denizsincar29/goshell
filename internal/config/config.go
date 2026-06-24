package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type SavedHost struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Port     string `json:"port"`
	User     string `json:"user"`
	KeyFile  string `json:"key_file"`
	SudoPass string `json:"sudo_pass"` // stored encrypted in future; plaintext for now
}

type Config struct {
	SavedHosts   []SavedHost `json:"saved_hosts"`
	GlobalSudo   string      `json:"global_sudo_pass"`
	LastHost     string      `json:"last_host"`
	DefaultLines int         `json:"default_log_lines"`
	FontSize     int         `json:"font_size"`
	Theme        string      `json:"theme"` // "default", "high-contrast"
}

func DefaultConfig() *Config {
	return &Config{
		DefaultLines: 200,
		FontSize:     12,
		Theme:        "default",
	}
}

func configPath() string {
	home := os.Getenv("HOME")
	dir := filepath.Join(home, ".config", "goshell")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "config.json")
}

func Load() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

func (c *Config) FindHost(name string) *SavedHost {
	for i := range c.SavedHosts {
		if c.SavedHosts[i].Name == name {
			return &c.SavedHosts[i]
		}
	}
	return nil
}

func (c *Config) UpsertHost(h SavedHost) {
	for i := range c.SavedHosts {
		if c.SavedHosts[i].Name == h.Name {
			c.SavedHosts[i] = h
			return
		}
	}
	c.SavedHosts = append(c.SavedHosts, h)
}

func (c *Config) DeleteHost(name string) {
	var out []SavedHost
	for _, h := range c.SavedHosts {
		if h.Name != name {
			out = append(out, h)
		}
	}
	c.SavedHosts = out
}
