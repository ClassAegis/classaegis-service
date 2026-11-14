// SPDX-License-Identifier: AGPL-3.0-only
// ability is the business capability process of ClassAegis agent (课佑管家).
// It hosts capability modules and executes verified commands forwarded by
// core over loopback HTTP. Loopback-only: NO external network access.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/core"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/core/logx"
	win "github.com/OneClassMesh/oneclassmesh-agent/internal/platform/windows"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/audit"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/av"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/clean"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/media"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/popup"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/proc"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ability/wol"
)

func main() {
	configPath := flag.String("config", "conf/config.toml", "path to config.toml")
	flag.Parse()

	if err := logx.Setup("ability"); err != nil {
		log.Printf("file log disabled: %v", err)
	}

	cfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	token := cfg.Agent.IPCToken
	if token == "" {
		token = "ocm-" + cfg.Device.MqttToken
	}

	// Progress events go to core POST /event → PROGRESS receipts on cmd/up.
	h := ability.NewHost(token, func(msgId string, result map[string]any) {
		if err := ability.PostEvent(token, msgId, result); err != nil {
			log.Printf("[ability] progress post failed: %v", err)
		}
	})
	if err := h.Register(ability.NewDesktopWithMpv(win.New(), cfg.Agent.MpvPath)); err != nil {
		log.Fatalf("register desktop ability failed: %v", err)
	}
	if err := h.Register(media.NewWithUpload(func(msgID, path string, meta map[string]any) (map[string]any, error) {
		return ability.PostMediaUpload(token, msgID, path, meta)
	})); err != nil {
		log.Fatalf("register media ability failed: %v", err)
	}
	// popup ability: always registered (toggle at runtime); watcher runs
	// only while enabled.
	popupEnabled := cfg.Popup.Enabled == nil || *cfg.Popup.Enabled
	popupEng := popup.New(popup.Options{
		RulesPath:  cfg.Popup.RulesPath,
		IntervalMS: cfg.Popup.IntervalMS,
		Action:     cfg.Popup.Action,
		MaxEvents:  cfg.Popup.MaxEvents,
		Enabled:    popupEnabled,
	})
	if err := h.Register(popupEng); err != nil {
		log.Fatalf("register popup ability failed: %v", err)
	}
	popupEng.Start()
	if cfg.Proc.Enabled == nil || *cfg.Proc.Enabled {
		procEng := proc.New(proc.Options{PollIntervalMS: cfg.Proc.PollIntervalMS})
		if err := h.Register(procEng); err != nil {
			log.Fatalf("register proc ability failed: %v", err)
		}
		procEng.Start()
	}
	if cfg.Wol.Enabled == nil || *cfg.Wol.Enabled {
		h.Register(wol.New(wol.Options{DefPort: cfg.Wol.DefPort, NetExec: func(op string, args map[string]any) (map[string]any, error) {
			return ability.PostNetExec(token, op, args)
		}}))
	}
	if cfg.Audit.Enabled == nil || *cfg.Audit.Enabled {
		if err := h.Register(audit.New(audit.Options{MaxRecords: cfg.Audit.MaxRecords})); err != nil {
			log.Fatalf("register audit ability failed: %v", err)
		}
	}
	var cleanEng *clean.Engine
	if cfg.Clean.Enabled == nil || *cfg.Clean.Enabled {
		cleanEng = clean.New(clean.Options{
			MinAgeHours:       cfg.Clean.MinAgeHours,
			Exclusions:        cfg.Clean.Exclusions,
			EnableCategories:  cfg.Clean.EnableCategories,
			DisableCategories: cfg.Clean.DisableCategories,
		})
		if err := h.Register(cleanEng); err != nil {
			log.Fatalf("register clean ability failed: %v", err)
		}
	}
	var avEng *av.Engine
	if cfg.Av.Enabled == nil || *cfg.Av.Enabled {
		heuristics := cfg.Av.EnableHeuristics == nil || *cfg.Av.EnableHeuristics
		avEng = av.New(av.Options{
			DbPath:           cfg.Av.DbPath,
			QuarantineDir:    cfg.Av.QuarantineDir,
			Threads:          cfg.Av.Threads,
			MaxFileSizeMB:    cfg.Av.MaxFileSizeMB,
			Exclusions:       cfg.Av.Exclusions,
			EnableHeuristics: heuristics,
			DefaultOnDetect:  cfg.Av.DefaultOnDetect,
		})
		if err := h.Register(avEng); err != nil {
			log.Fatalf("register av ability failed: %v", err)
		}
	}
	startScheduledJobs(cfg, avEng, cleanEng)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			if err := h.RegisterToCore(token); err != nil {
				log.Printf("[ability] register to core failed: %v", err)
			}
			<-ticker.C
		}
	}()
	log.Printf("[ability] started, platform=windows abilities=%v", h.Abilities())
	if err := h.Serve(); err != nil {
		log.Fatalf("ability serve failed: %v", err)
	}
}

func startScheduledJobs(cfg *core.Config, avEng *av.Engine, cleanEng *clean.Engine) {
	if avEng != nil && cfg.Av.ScanIntervalHours > 0 {
		go runOnInterval("av-scan", time.Duration(cfg.Av.ScanIntervalHours)*time.Hour, func() {
			mode := cfg.Av.ScanMode
			if mode == "" {
				mode = "quick"
			}
			ok, code, message, result := avEng.Handle("scheduled-av-"+time.Now().Format("20060102T150405"), "AV_SCAN", map[string]any{"mode": mode}, nil)
			logScheduledResult("av scan", ok, code, message, result)
		})
	}
	if cleanEng != nil && cfg.Clean.ScanIntervalHours > 0 {
		go runOnInterval("clean-scan", time.Duration(cfg.Clean.ScanIntervalHours)*time.Hour, func() {
			ok, code, message, result := cleanEng.Handle("scheduled-clean-scan-"+time.Now().Format("20060102T150405"), "CLEAN_SCAN", scheduledCategories(cfg.Clean.ScheduledCategories), nil)
			logScheduledResult("clean scan", ok, code, message, result)
		})
	}
	if cleanEng != nil && cfg.Clean.RunIntervalHours > 0 {
		if len(cfg.Clean.ScheduledCategories) == 0 {
			log.Printf("[ability] scheduled clean run disabled: scheduled_categories must be explicit")
			return
		}
		go runOnInterval("clean-run", time.Duration(cfg.Clean.RunIntervalHours)*time.Hour, func() {
			ok, code, message, result := cleanEng.Handle("scheduled-clean-run-"+time.Now().Format("20060102T150405"), "CLEAN_RUN", scheduledCategories(cfg.Clean.ScheduledCategories), nil)
			logScheduledResult("clean run", ok, code, message, result)
		})
	}
}

func scheduledCategories(ids []string) map[string]any {
	values := make([]any, len(ids))
	for i, id := range ids {
		values[i] = id
	}
	if len(values) == 0 {
		return map[string]any{}
	}
	return map[string]any{"categories": values}
}

func runOnInterval(name string, interval time.Duration, run func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("[ability] scheduled job %s enabled: interval=%s", name, interval)
	for range ticker.C {
		run()
	}
}

func logScheduledResult(name string, ok bool, code, message string, result map[string]any) {
	if !ok {
		log.Printf("[ability] scheduled %s failed: %s %s", name, code, message)
		return
	}
	log.Printf("[ability] scheduled %s completed: %v", name, result)
}
