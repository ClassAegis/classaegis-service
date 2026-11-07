// agent is the all-in-one debug mode: core + worker in a single process.
// Handy for quick local testing; production runs cmd/core + cmd/worker.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	// In-process executor: dispatcher calls the platform directly.
	exec := core.ExecutorFunc(func(_ string, msgType string, payload map[string]any) (bool, string, string) {
		return worker.ExecuteCommand(win.New(), msgType, payload)
	})

	agent, err := core.NewAgent(cfg, exec, "windows")
	if err != nil {
		log.Fatalf("new agent failed: %v", err)
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		agent.Stop()
		os.Exit(0)
	}()

	if err := agent.Run(); err != nil {
		log.Fatalf("agent run failed: %v", err)
	}
}
