package main

import (
	"log"
	"net/http"

	"dnsmanager/internal/config"
	"dnsmanager/internal/server"
)

func main() {
	cfg := config.Load()

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	log.Printf("dnsmanager listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, srv.Handler()); err != nil {
		log.Fatalf("serve http: %v", err)
	}
}
