// worker is the business process of OneClassMesh agent (壹课管家).
// It executes verified commands via the platform adapter.
// Loopback-only: it has NO external network access by design.
package main

import (
	"flag"
	"log"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/worker"
	win "github.com/OneClassMesh/oneclassmesh-agent/internal/platform/windows"
)

func main() {
	configPath := flag.String("config", "conf/config.toml", "path to config.toml")
	flag.Parse()

	cfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	token := cfg.Agent.IPCToken
	if token == "" {
		token = "ocm-" + cfg.Device.MqttToken
	}

	w := worker.New(win.New(), token)
	if err := w.RegisterToCore(token); err != nil {
		log.Printf("[worker] register to core failed (will retry on demand): %v", err)
	}
	log.Printf("[worker] started, platform=%s", "windows")
	if err := w.Serve(); err != nil {
		log.Fatalf("worker serve failed: %v", err)
	}
}
