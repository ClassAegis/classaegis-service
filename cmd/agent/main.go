// agent is the all-in-one debug mode: core + ability in a single process.
// Handy for quick local testing; production runs cmd/core + cmd/ability.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/core"
	win "github.com/OneClassMesh/oneclassmesh-agent/internal/platform/windows"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/audit"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/clean"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/popup"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/proc"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/wol"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core/netops"
)

func main() {
	configPath := flag.String("config", "conf/config.toml", "path to config.toml")
	flag.Parse()

	cfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// Ability host with all capability modules, executed in-process.
	// Progress receipts publish directly via the in-process agent.
	var agent *core.Agent
	host := ability.NewHost("", func(msgId string, result map[string]any) {
		if agent != nil {
			agent.PublishProgress(msgId, result)
		}
	})
	if err := host.Register(ability.NewDesktopWithMpv(win.New(), cfg.Agent.MpvPath)); err != nil {
		log.Fatalf("register desktop ability failed: %v", err)
	}
	popupEng := popup.New(popup.Options{Enabled: true})
	if err := host.Register(popupEng); err != nil {
		log.Fatalf("register popup ability failed: %v", err)
	}
	popupEng.Start()
	if err := host.Register(clean.New(clean.Options{MinAgeHours: 24})); err != nil {
		log.Fatalf("register clean ability failed: %v", err)
	}
	if err := host.Register(audit.New(audit.Options{MaxRecords: 100})); err != nil {
		log.Fatalf("register audit ability failed: %v", err)
	}
	if err := host.Register(wol.New(wol.Options{NetExec: netops.Execute})); err != nil {
		log.Fatalf("register wol ability failed: %v", err)
	}
	procEng := proc.New(proc.Options{})
	if err := host.Register(procEng); err != nil {
		log.Fatalf("register proc ability failed: %v", err)
	}
	procEng.Start()

	exec := core.ExecutorFunc(host.ExecuteCommand)

	agent, err = core.NewAgent(cfg, exec, "windows")
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
