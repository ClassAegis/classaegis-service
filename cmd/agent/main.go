//go:build windows

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core"
	win "github.com/OneClassMesh/oneclassmesh-agent/internal/platform/windows"
)

func main() {
	configPath := flag.String("config", "conf/config.toml", "path to config.toml")
	flag.Parse()

	cfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	agent, err := core.NewAgent(cfg, win.New())
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
