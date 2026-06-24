package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/crgimenes/glaze"

	"github.com/denizsincar29/goshell/internal/config"
	"github.com/denizsincar29/goshell/internal/ui"
)

// AppWindow (glaze's HTTP-handler convenience wrapper) and Bind are
// mutually exclusive in this library: AppWindow creates the window
// internally and never hands back the WebView, while Bind only exists on
// the value glaze.New() returns. Since most of this app's API is simple
// request/response (a great fit for Bind, see internal/ui/service.go) and
// only two things genuinely need a server push (apt output, the terminal —
// see internal/ui/server.go), we assemble the pieces ourselves: our own
// tiny HTTP server for static assets + the two WebSocket routes, our own
// glaze.New() window, Bind for everything else.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: could not load config: %v", err)
		cfg = config.DefaultConfig()
	}

	svc := ui.NewService(cfg)
	mux := ui.NewMux(svc)

	baseURL, err := startServer(mux)
	if err != nil {
		log.Fatal(err)
	}

	w, err := glaze.New(false)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Destroy()

	w.SetTitle("GoShell - Accessible SSH Manager")
	w.SetSize(1100, 750, glaze.HintNone)

	if _, err := glaze.BindMethods(w, "goshell", svc); err != nil {
		log.Fatal("BindMethods:", err)
	}

	w.Navigate(baseURL)
	w.Run()
}

// startServer starts the app's HTTP/WebSocket server on a random loopback
// port and returns its base URL.
func startServer(mux http.Handler) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	addr := ln.Addr().(*net.TCPAddr)
	return fmt.Sprintf("http://127.0.0.1:%d", addr.Port), nil
}
