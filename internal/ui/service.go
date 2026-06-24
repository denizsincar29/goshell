package ui

import (
	"errors"
	"sync"

	"github.com/denizsincar29/goshell/internal/config"
	sshpkg "github.com/denizsincar29/goshell/internal/ssh"
)

// Service holds the single active SSH connection and app config.
// Every exported method here becomes window.goshell_<snake_case_name>(...)
// in JS once bound with glaze.BindMethods(w, "goshell", svc).
//
// Methods may return (value, error), error only, or a bare value — glaze
// turns the error into a rejected JS Promise and the value into the
// resolved one. Each call already runs off the UI thread, so blocking SSH
// round-trips here do not freeze the window.
type Service struct {
	mu     sync.RWMutex
	client *sshpkg.Client
	cfg    *config.Config
}

func NewService(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) getClient() (*sshpkg.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.client == nil {
		return nil, errors.New("not connected")
	}
	return s.client, nil
}

// ---- Connection ----

func (s *Service) SSHHosts() ([]sshpkg.Host, error) {
	return sshpkg.ParseSSHConfig()
}

func (s *Service) SSHKeys() []string {
	return sshpkg.ListKeys()
}

func (s *Service) Connect(params sshpkg.ConnectParams) (map[string]string, error) {
	if params.Port == "" {
		params.Port = "22"
	}
	client, err := sshpkg.Connect(params)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.client != nil {
		s.client.Close()
	}
	s.client = client
	s.mu.Unlock()

	s.cfg.LastHost = params.Host
	s.cfg.Save()

	return map[string]string{"host": params.Host, "user": params.User}, nil
}

func (s *Service) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
	return nil
}

// ---- Saved hosts & settings ----

func (s *Service) ConfigHosts() []config.SavedHost {
	return s.cfg.SavedHosts
}

func (s *Service) SaveHost(h config.SavedHost) error {
	s.cfg.UpsertHost(h)
	return s.cfg.Save()
}

func (s *Service) DeleteHost(name string) error {
	s.cfg.DeleteHost(name)
	return s.cfg.Save()
}

func (s *Service) GetSettings() *config.Config {
	return s.cfg
}

type SettingsUpdate struct {
	GlobalSudo   string `json:"GlobalSudo"`
	DefaultLines int    `json:"DefaultLines"`
}

func (s *Service) SaveSettings(upd SettingsUpdate) error {
	s.cfg.GlobalSudo = upd.GlobalSudo
	if upd.DefaultLines > 0 {
		s.cfg.DefaultLines = upd.DefaultLines
	}
	return s.cfg.Save()
}

// ---- Services (systemd) ----

func (s *Service) ListServices() ([]sshpkg.ServiceStatus, error) {
	c, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return c.ListServices()
}

type ServiceActionRequest struct {
	Name    string `json:"name"`
	Action  string `json:"action"`
	UseSudo bool   `json:"use_sudo"`
}

func (s *Service) ServiceAction(req ServiceActionRequest) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	return c.ServiceAction(req.Name, req.Action, req.UseSudo)
}

func (s *Service) ServiceLogs(name string) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	lines := 200
	if s.cfg.DefaultLines > 0 {
		lines = s.cfg.DefaultLines
	}
	return c.ServiceLogs(name, lines)
}

func (s *Service) ServiceStatusDetail(name string) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	return c.ServiceStatusDetail(name)
}

// ---- Crontab ----
//
// GetCrontab/SetCrontab work with the raw crontab text and back the
// advanced "raw editor" fallback. GetCronEntries/SetCronEntries are the
// default path: they parse/serialize on the Go side (see cron.go) so the
// JS form never has to construct or read cron's five-field syntax itself.

func (s *Service) GetCrontab(user string) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	return c.GetCrontab(user)
}

type CrontabUpdate struct {
	User    string `json:"user"`
	Content string `json:"content"`
	UseSudo bool   `json:"use_sudo"`
}

func (s *Service) SetCrontab(req CrontabUpdate) error {
	c, err := s.getClient()
	if err != nil {
		return err
	}
	_, err = c.SetCrontab(req.User, req.Content, req.UseSudo)
	return err
}

func (s *Service) GetCronEntries(user string) ([]CronEntry, error) {
	c, err := s.getClient()
	if err != nil {
		return nil, err
	}
	raw, err := c.GetCrontab(user)
	if err != nil {
		return nil, err
	}
	return ParseCrontab(raw), nil
}

type SetCronEntriesRequest struct {
	User    string      `json:"user"`
	Entries []CronEntry `json:"entries"`
	UseSudo bool        `json:"use_sudo"`
}

func (s *Service) SetCronEntries(req SetCronEntriesRequest) error {
	c, err := s.getClient()
	if err != nil {
		return err
	}
	content := SerializeCrontab(req.Entries)
	_, err = c.SetCrontab(req.User, content, req.UseSudo)
	return err
}

func (s *Service) DescribeCronSchedule(e CronEntry) string {
	return DescribeSchedule(e)
}

// ---- Files ----

type DirListing struct {
	Path    string             `json:"path"`
	Entries []sshpkg.FileEntry `json:"entries"`
}

func (s *Service) ListDir(path string) (*DirListing, error) {
	c, err := s.getClient()
	if err != nil {
		return nil, err
	}
	if path == "" {
		path = "/"
	}
	entries, err := c.ListDir(path)
	if err != nil {
		return nil, err
	}
	return &DirListing{Path: path, Entries: entries}, nil
}

func (s *Service) ReadFile(path string, useSudo bool) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	return c.ReadFile(path, useSudo)
}

type FileWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	UseSudo bool   `json:"use_sudo"`
}

func (s *Service) WriteFile(req FileWriteRequest) error {
	c, err := s.getClient()
	if err != nil {
		return err
	}
	return c.WriteFile(req.Path, req.Content, req.UseSudo)
}

type ChmodRequest struct {
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	Recursive bool   `json:"recursive"`
	UseSudo   bool   `json:"use_sudo"`
}

func (s *Service) Chmod(req ChmodRequest) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	return c.Chmod(req.Path, req.Mode, req.Recursive, req.UseSudo)
}

type ChownRequest struct {
	Path      string `json:"path"`
	Owner     string `json:"owner"`
	Group     string `json:"group"`
	Recursive bool   `json:"recursive"`
	UseSudo   bool   `json:"use_sudo"`
}

func (s *Service) Chown(req ChownRequest) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	return c.Chown(req.Path, req.Owner, req.Group, req.Recursive, req.UseSudo)
}

func (s *Service) DiskUsage() ([]sshpkg.DiskInfo, error) {
	c, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return c.DiskUsage()
}

func (s *Service) DirSize(path string, useSudo bool) (string, error) {
	c, err := s.getClient()
	if err != nil {
		return "", err
	}
	return c.DirSize(path, useSudo)
}

// ---- Resources ----

func (s *Service) GetResources() (*sshpkg.ResourceInfo, error) {
	c, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return c.GetResources()
}

func (s *Service) GetProcesses() ([]sshpkg.ProcessInfo, error) {
	c, err := s.getClient()
	if err != nil {
		return nil, err
	}
	return c.GetProcesses()
}

// clientForWS exposes the live client to the WebSocket handlers in
// server.go, which still need raw streaming (apt output, terminal),
// since Bind has no server-push primitive.
func (s *Service) clientForWS() *sshpkg.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}
