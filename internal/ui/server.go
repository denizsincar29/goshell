package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return strings.HasPrefix(r.Host, "127.0.0.1:") || strings.HasPrefix(r.Host, "localhost:")
	},
}

// NewMux builds the http.Handler passed to glaze.AppWindow. It only serves
// the embedded frontend and the two streaming WebSocket endpoints; every
// other operation is a direct Bind call from JS straight into Service's
// methods (see service.go), so there is no JSON request/response routing
// layer left to maintain here.
func NewMux(svc *Service) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/static/app.js", serveJS)
	mux.HandleFunc("/static/app.css", serveCSS)
	mux.HandleFunc("/ws/terminal", wsHandler(svc, handleTerminalWS))
	mux.HandleFunc("/ws/apt", wsHandler(svc, handleAptWS))
	return mux
}

func wsHandler(svc *Service, h func(*websocket.Conn, *Service)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("ws upgrade:", err)
			return
		}
		defer conn.Close()
		h(conn, svc)
	}
}

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

// ---- WebSocket: Terminal ----
// Genuinely bidirectional and long-lived (stdin keeps flowing while output
// keeps streaming), which Bind's call-in/Promise-out shape cannot express.

func handleTerminalWS(conn *websocket.Conn, svc *Service) {
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var req struct {
		Cmd     string `json:"cmd"`
		UseSudo bool   `json:"use_sudo"`
	}
	if err := json.Unmarshal(msgBytes, &req); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"done","error":"bad request"}`))
		return
	}

	client := svc.clientForWS()
	if client == nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"done","error":"not connected"}`))
		return
	}

	cmd := req.Cmd
	if req.UseSudo && client.SudoPass != "" {
		cmd = fmt.Sprintf("echo %s | sudo -S bash -c %s", shellQuote(client.SudoPass), shellQuote(req.Cmd))
	}

	inputCh := make(chan string, 32)
	doneCh := client.RunInteractiveShell(cmd, func(output string) {
		data, _ := json.Marshal(map[string]string{"type": "output", "data": output})
		conn.WriteMessage(websocket.TextMessage, data)
	}, inputCh)

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
// Live progress + output while update/upgrade runs. Same reasoning as the
// terminal: this is a push stream, not a single request/response.

func handleAptWS(conn *websocket.Conn, svc *Service) {
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var req struct {
		Operation    string `json:"operation"`
		ConfigAction string `json:"config_action"`
	}
	if err := json.Unmarshal(msgBytes, &req); err != nil {
		return
	}

	client := svc.clientForWS()
	if client == nil {
		data, _ := json.Marshal(map[string]string{"type": "done", "error": "not connected"})
		conn.WriteMessage(websocket.TextMessage, data)
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
	outputCb := func(text string) { sendMsg("output", text) }

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
		err := <-doneCh
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
		if err := run(req.Operation); err != nil {
			sendMsg("done", err.Error())
			return
		}
	}
	sendMsg("done", "")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
