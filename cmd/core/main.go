// core is the core process of ClassAegis agent (课佑管家).
// It is the ONLY component with external network access:
// MQTT to the intranet server, loopback IPC to the ability process and ui.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/core/logx"
)

func main() {
	configPath := flag.String("config", "conf/config.toml", "path to config.toml")
	flag.Parse()

	if err := logx.Setup("core"); err != nil {
		log.Printf("file log disabled: %v", err)
	}

	cfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	executor := core.NewIPCExecutor(ipcToken(cfg))
	agent, err := core.NewAgent(cfg, executor, "windows")
	if err != nil {
		log.Fatalf("new agent failed: %v", err)
	}

	go func() {
		if err := agent.ServeIPC(ipcToken(cfg)); err != nil {
			log.Printf("[core] ipc server stopped: %v", err)
		}
	}()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		agent.Stop()
		os.Exit(0)
	}()

	log.Printf("[core] started, sn=%s school=%s", cfg.Device.SN, cfg.Device.SchoolCode)
	if err := agent.Run(); err != nil {
		log.Fatalf("agent run failed: %v", err)
	}
}

func ipcToken(cfg *core.Config) string {
	if cfg.Agent.IPCToken != "" {
		return cfg.Agent.IPCToken
	}
	return "ocm-" + cfg.Device.MqttToken
}
