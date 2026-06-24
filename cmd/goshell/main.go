package main

import (
	"log"

	"github.com/crgimenes/glaze"
	_ "github.com/crgimenes/glaze/embedded"
	"github.com/denizsincar29/goshell/internal/config"
	"github.com/denizsincar29/goshell/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: could not load config: %v", err)
		cfg = config.DefaultConfig()
	}

	srv := ui.NewServer(cfg)
	mux := srv.Mux()

	err = glaze.AppWindow(glaze.AppOptions{
		Title:     "GoShell - Accessible SSH Manager",
		Width:     1100,
		Height:    750,
		Hint:      glaze.HintNone,
		Transport: glaze.AppTransportAuto,
		Debug:     false,
		Handler:   mux,
		OnReadyInfo: func(info glaze.AppReadyInfo) {
			log.Printf("GoShell ready: transport=%s url=%s", info.Transport, info.URL)
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
