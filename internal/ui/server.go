package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/denizsincar29/goshell/internal/config"
	sshpkg "github.com/denizsincar29/goshell/internal/ssh"
	"github.com/gorilla/websocket"
)

// Server is the local HTTP server that serves the UI
type Server struct {
	cfg     *config.Config
	clients map[string]*sshpkg.Client // keyed by session ID
	mu      sync.RWMutex
	mux     *http.ServeMux
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Only allow localhost
		host := r.Host
		return strings.HasPrefix(host, "127.0.0.1:") || strings.HasPrefix(host, "localhost:")
	},
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{
		cfg:     cfg,
		clients: make(map[string]*sshpkg.Client),
		mux:     http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Mux returns the configured http.ServeMux so it can be handed to
// glaze.AppWindow (which owns the actual listener/transport).
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

func (s *Server) registerRoutes() {
	// Serve embedded HTML/JS/CSS
	s.mux.HandleFunc("/", serveIndex)
	s.mux.HandleFunc("/static/app.js", serveJS)
	s.mux.HandleFunc("/static/app.css", serveCSS)

	// API
	s.mux.HandleFunc("/api/ssh/hosts", s.handleSSHHosts)
	s.mux.HandleFunc("/api/ssh/keys", s.handleSSHKeys)
	s.mux.HandleFunc("/api/ssh/connect", s.handleConnect)
	s.mux.HandleFunc("/api/ssh/disconnect", s.handleDisconnect)
	s.mux.HandleFunc("/api/config/hosts", s.handleConfigHosts)
	s.mux.HandleFunc("/api/config/save-host", s.handleSaveHost)
	s.mux.HandleFunc("/api/config/delete-host", s.handleDeleteHost)
	s.mux.HandleFunc("/api/config/settings", s.handleSettings)

	// Session endpoints (require active SSH connection)
	s.mux.HandleFunc("/api/services/list", s.withClient(s.handleServicesList))
	s.mux.HandleFunc("/api/services/action", s.withClient(s.handleServicesAction))
	s.mux.HandleFunc("/api/services/logs", s.withClient(s.handleServiceLogs))
	s.mux.HandleFunc("/api/cron/get", s.withClient(s.handleCronGet))
	s.mux.HandleFunc("/api/cron/set", s.withClient(s.handleCronSet))
	s.mux.HandleFunc("/api/files/list", s.withClient(s.handleFilesList))
	s.mux.HandleFunc("/api/files/read", s.withClient(s.handleFileRead))
	s.mux.HandleFunc("/api/files/write", s.withClient(s.handleFileWrite))
	s.mux.HandleFunc("/api/files/chmod", s.withClient(s.handleChmod))
	s.mux.HandleFunc("/api/files/chown", s.withClient(s.handleChown))
	s.mux.HandleFunc("/api/resources", s.withClient(s.handleResources))
	s.mux.HandleFunc("/api/processes", s.withClient(s.handleProcesses))
	s.mux.HandleFunc("/api/disk", s.withClient(s.handleDisk))

	// WebSocket endpoints for live streaming
	s.mux.HandleFunc("/ws/terminal", s.withClientWS(s.handleTerminalWS))
	s.mux.HandleFunc("/ws/apt", s.withClientWS(s.handleAptWS))
}

// ---- Middleware ----

func (s *Server) withClient(h func(http.ResponseWriter, *http.Request, *sshpkg.Client)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := s.getClient()
		if client == nil {
			jsonError(w, "not connected", http.StatusUnauthorized)
			return
		}
		h(w, r, client)
	}
}

func (s *Server) withClientWS(h func(*websocket.Conn, *sshpkg.Client)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := s.getClient()
		if client == nil {
			http.Error(w, "not connected", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WS upgrade:", err)
			return
		}
		defer conn.Close()
		h(conn, client)
	}
}

func (s *Server) getClient() *sshpkg.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients["default"]
}

func (s *Server) setClient(c *sshpkg.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.clients["default"]; ok && old != nil {
		old.Close()
	}
	s.clients["default"] = c
}

// ---- Connection endpoints ----

func (s *Server) handleSSHHosts(w http.ResponseWriter, r *http.Request) {
	hosts, _ := sshpkg.ParseSSHConfig()
	jsonOK(w, hosts)
}

func (s *Server) handleSSHKeys(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, sshpkg.ListKeys())
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var params sshpkg.ConnectParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if params.Port == "" {
		params.Port = "22"
	}

	client, err := sshpkg.Connect(params)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.setClient(client)

	// Save last host
	s.cfg.LastHost = params.Host
	s.cfg.Save()

	jsonOK(w, map[string]string{
		"status": "connected",
		"host":   params.Host,
		"user":   params.User,
	})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if c, ok := s.clients["default"]; ok && c != nil {
		c.Close()
		delete(s.clients, "default")
	}
	s.mu.Unlock()
	jsonOK(w, map[string]string{"status": "disconnected"})
}

// ---- Config endpoints ----

func (s *Server) handleConfigHosts(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, s.cfg.SavedHosts)
}

func (s *Server) handleSaveHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var h config.SavedHost
	if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.cfg.UpsertHost(h)
	s.cfg.Save()
	jsonOK(w, map[string]string{"status": "saved"})
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	s.cfg.DeleteHost(name)
	s.cfg.Save()
	jsonOK(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jsonOK(w, s.cfg)
		return
	}
	if r.Method == http.MethodPost {
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.cfg.GlobalSudo = newCfg.GlobalSudo
		s.cfg.DefaultLines = newCfg.DefaultLines
		s.cfg.Theme = newCfg.Theme
		s.cfg.Save()
		jsonOK(w, map[string]string{"status": "saved"})
		return
	}
	http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
}

// ---- Services ----

func (s *Server) handleServicesList(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	svcs, err := client.ListServices()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, svcs)
}

func (s *Server) handleServicesAction(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name    string `json:"name"`
		Action  string `json:"action"`
		UseSudo bool   `json:"use_sudo"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	out, err := client.ServiceAction(req.Name, req.Action, req.UseSudo)
	if err != nil {
		jsonError(w, fmt.Sprintf("%v: %s", err, out), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"output": out})
}

func (s *Server) handleServiceLogs(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	name := r.URL.Query().Get("name")
	lines := 200
	if s.cfg.DefaultLines > 0 {
		lines = s.cfg.DefaultLines
	}
	out, err := client.ServiceLogs(name, lines)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"logs": out})
}

// ---- Cron ----

func (s *Server) handleCronGet(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	user := r.URL.Query().Get("user")
	out, err := client.GetCrontab(user)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"crontab": out})
}

func (s *Server) handleCronSet(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		User    string `json:"user"`
		Content string `json:"content"`
		UseSudo bool   `json:"use_sudo"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	_, err := client.SetCrontab(req.User, req.Content, req.UseSudo)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "saved"})
}

// ---- Files ----

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	entries, err := client.ListDir(path)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"path": path, "entries": entries})
}

func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	path := r.URL.Query().Get("path")
	sudo := r.URL.Query().Get("sudo") == "1"
	content, err := client.ReadFile(path, sudo)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"content": content, "path": path})
}

func (s *Server) handleFileWrite(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		UseSudo bool   `json:"use_sudo"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := client.WriteFile(req.Path, req.Content, req.UseSudo); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "saved"})
}

func (s *Server) handleChmod(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path      string `json:"path"`
		Mode      string `json:"mode"`
		Recursive bool   `json:"recursive"`
		UseSudo   bool   `json:"use_sudo"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	out, err := client.Chmod(req.Path, req.Mode, req.Recursive, req.UseSudo)
	if err != nil {
		jsonError(w, fmt.Sprintf("%v: %s", err, out), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleChown(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path      string `json:"path"`
		Owner     string `json:"owner"`
		Group     string `json:"group"`
		Recursive bool   `json:"recursive"`
		UseSudo   bool   `json:"use_sudo"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	out, err := client.Chown(req.Path, req.Owner, req.Group, req.Recursive, req.UseSudo)
	if err != nil {
		jsonError(w, fmt.Sprintf("%v: %s", err, out), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

// ---- Resources ----

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	ri, err := client.GetResources()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, ri)
}

func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	out, err := client.GetProcesses()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"output": out})
}

func (s *Server) handleDisk(w http.ResponseWriter, r *http.Request, client *sshpkg.Client) {
	out, err := client.DiskUsage()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"output": out})
}

// ---- WebSocket: Terminal ----

func (s *Server) handleTerminalWS(conn *websocket.Conn, client *sshpkg.Client) {
	// Read first message: the command to run
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var req struct {
		Cmd     string `json:"cmd"`
		UseSudo bool   `json:"use_sudo"`
	}
	if err := json.Unmarshal(msgBytes, &req); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"bad request"}`))
		return
	}

	cmd := req.Cmd
	if req.UseSudo && client.SudoPass != "" {
		cmd = fmt.Sprintf("echo %s | sudo -S bash -c %s",
			shellescape(client.SudoPass), shellescape(req.Cmd))
	}

	inputCh := make(chan string, 32)
	doneCh := client.RunInteractiveShell(cmd, func(output string) {
		data, _ := json.Marshal(map[string]string{"type": "output", "data": output})
		conn.WriteMessage(websocket.TextMessage, data)
	}, inputCh)

	// Read stdin from websocket
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				close(inputCh)
				return
			}
			var inp struct {
				Input string `json:"input"`
			}
			if json.Unmarshal(msg, &inp) == nil {
				select {
				case inputCh <- inp.Input:
				default:
				}
			}
		}
	}()

	err = <-doneCh
	msg := map[string]string{"type": "done"}
	if err != nil {
		msg["error"] = err.Error()
	}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}

// ---- WebSocket: APT ----

func (s *Server) handleAptWS(conn *websocket.Conn, client *sshpkg.Client) {
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var req struct {
		Operation    string `json:"operation"`     // "update", "upgrade", "dist-upgrade", "update+upgrade"
		ConfigAction string `json:"config_action"` // "keep", "new", "default"
	}
	if err := json.Unmarshal(msgBytes, &req); err != nil {
		return
	}

	sendMsg := func(typ, data string) {
		b, _ := json.Marshal(map[string]string{"type": typ, "data": data})
		conn.WriteMessage(websocket.TextMessage, b)
	}

	sendProgress := func(pct int, label string) {
		b, _ := json.Marshal(map[string]interface{}{"type": "progress", "pct": pct, "label": label})
		conn.WriteMessage(websocket.TextMessage, b)
	}

	outputCb := func(text string) {
		sendMsg("output", text)
	}

	run := func(op string) error {
		var doneCh chan error
		switch op {
		case "update":
			sendProgress(0, "Running apt-get update…")
			doneCh = client.RunAptUpdate(outputCb)
		case "upgrade":
			sendProgress(0, "Running apt-get upgrade…")
			doneCh = client.RunAptUpgrade(outputCb, req.ConfigAction)
		case "dist-upgrade":
			sendProgress(0, "Running apt-get dist-upgrade…")
			doneCh = client.RunAptDistUpgrade(outputCb, req.ConfigAction)
		default:
			return fmt.Errorf("unknown operation: %s", op)
		}
		// Pulse progress while running
		ticker := time.NewTicker(300 * time.Millisecond)
		pct := 0
		go func() {
			for range ticker.C {
				pct = (pct + 3) % 95 // pulse between 0-95 until done
			}
		}()
		err := <-doneCh
		ticker.Stop()
		if err != nil {
			sendProgress(0, "Failed: "+err.Error())
			return err
		}
		sendProgress(100, op+" complete")
		return nil
	}

	switch req.Operation {
	case "update+upgrade":
		sendMsg("output", "=== Step 1: apt-get update ===\n")
		if err := run("update"); err != nil {
			sendMsg("done", "update failed: "+err.Error())
			return
		}
		sendMsg("output", "\n=== Step 2: apt-get upgrade ===\n")
		if err := run("upgrade"); err != nil {
			sendMsg("done", "upgrade failed: "+err.Error())
			return
		}
	default:
		run(req.Operation)
	}

	sendMsg("done", "")
}

// ---- Helpers ----

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// serveIndex, serveJS, serveCSS serve embedded frontend
func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, indexHTML)
}

func serveJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	io.WriteString(w, appJS)
}

func serveCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	io.WriteString(w, appCSS)
}
