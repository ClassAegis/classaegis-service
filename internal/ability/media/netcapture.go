// SPDX-License-Identifier: AGPL-3.0-only
package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type captureJob struct {
	mu sync.Mutex
	active bool
	file string
	cancel context.CancelFunc
	done chan struct{}
}

func (a *Ability) capture(typ string, payload map[string]any) (bool, string, string, map[string]any) {
	if typ == "NET_CAPTURE_STATUS" {
		a.captureJob.mu.Lock(); defer a.captureJob.mu.Unlock()
		return true, "", "", map[string]any{"active": a.captureJob.active, "file": a.captureJob.file}
	}
	if typ == "NET_CAPTURE_STOP" {
		a.captureJob.mu.Lock()
		active, cancel, done, file := a.captureJob.active, a.captureJob.cancel, a.captureJob.done, a.captureJob.file
		a.captureJob.mu.Unlock()
		if active && cancel != nil { cancel(); <-done }
		return true, "", "", map[string]any{"active": false, "file": file}
	}
	seconds := 30
	if v, ok := payload["durationSeconds"].(float64); ok { seconds = int(v) }
	if seconds < 1 || seconds > 60 { return false, "E_PARAM", "durationSeconds must be 1..60", nil }
	return a.startCapture(seconds)
}

func (a *Ability) startCapture(seconds int) (bool, string, string, map[string]any) {
	a.captureJob.mu.Lock()
	defer a.captureJob.mu.Unlock()
	if a.captureJob.active { return false, "E_DEVICE_BUSY", "network capture already running", nil }
	if _, err := exec.LookPath("pktmon.exe"); err != nil { return false, "E_PLATFORM_UNSUPPORTED", "pktmon.exe not found", nil }
	consent := `Add-Type -AssemblyName PresentationFramework; $r=[System.Windows.MessageBox]::Show('ClassAegis wants to capture up to 96 bytes per network packet for at most ` + fmt.Sprint(seconds) + ` seconds. The capture stays on this device and may contain network metadata. Start capture?','ClassAegis network diagnostics','YesNo','Warning'); if($r -eq 'Yes'){exit 0}else{exit 2}`
	if err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", consent).Run(); err != nil { return false, "E_USER_CONSENT_REQUIRED", "local user declined or consent dialog unavailable", nil }
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "ClassAegis", "captures")
	if err := os.MkdirAll(dir, 0700); err != nil { return false, "E_FAILED", err.Error(), nil }
	base := filepath.Join(dir, fmt.Sprintf("capture-%s-%d", time.Now().Format("20060102T150405"), time.Now().UnixNano()))
	etlFile, file := base+".etl", base+".pcapng"
	cmd := exec.Command("pktmon.exe", "start", "--capture", "--pkt-size", "96", "--file-name", etlFile, "--file-size", "64", "--log-mode", "circular")
	if out, err := cmd.CombinedOutput(); err != nil { return false, "E_FAILED", fmt.Sprintf("pktmon start failed: %v: %s", err, out), nil }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.captureJob.active, a.captureJob.file, a.captureJob.cancel, a.captureJob.done = true, file, cancel, done
	go func() {
		defer close(done)
		select { case <-time.After(time.Duration(seconds)*time.Second): case <-ctx.Done(): }
		_, _ = exec.Command("pktmon.exe", "stop").CombinedOutput()
		_, _ = exec.Command("pktmon.exe", "etl2pcap", etlFile, "-o", file).CombinedOutput()
		a.captureJob.mu.Lock(); a.captureJob.active = false; a.captureJob.cancel = nil; a.captureJob.done = nil; a.captureJob.mu.Unlock()
	}()
	return true, "", "", map[string]any{"active": true, "file": file, "durationSeconds": seconds, "packetBytes": 96}
}
