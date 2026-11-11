package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var errNoMediaWindow = errors.New("no supported media player foreground")

type screenJob struct {
	mu           sync.Mutex
	active       bool
	frames       int
	lastAnalysis map[string]any
	lastError    string
	cancel       context.CancelFunc
	done         chan struct{}
}

func (a *Ability) screenMonitor(msgID, typ string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	switch typ {
	case "AI_SCREEN_MONITOR_STATUS":
		return true, "", "", a.screenStatus()
	case "AI_SCREEN_MONITOR_STOP":
		a.screenJob.mu.Lock()
		cancel, done, active := a.screenJob.cancel, a.screenJob.done, a.screenJob.active
		a.screenJob.mu.Unlock()
		if active && cancel != nil {
			cancel()
			<-done
		}
		return true, "", "", a.screenStatus()
	case "AI_SCREEN_MONITOR_START":
		if runtime.GOOS != "windows" {
			return false, "E_PLATFORM_UNSUPPORTED", "AI screen monitoring requires Windows", nil
		}
		if a.upload == nil {
			return false, "E_PLATFORM_UNSUPPORTED", "Core media upload proxy is not configured", nil
		}
		if consent, ok := payload["consent"].(bool); !ok || !consent {
			return false, "E_USER_CONSENT_REQUIRED", "consent=true and local confirmation are required", nil
		}
		seconds, interval, width := intParam(payload, "durationSeconds", 60), intParam(payload, "intervalSeconds", 5), intParam(payload, "width", 320)
		if seconds < 1 || seconds > 600 || interval < 1 || interval > 30 || (width != 320 && width != 640) {
			return false, "E_PARAM", "durationSeconds=1..600, intervalSeconds=1..30, width=320|640 required", nil
		}
		if !confirmScreenMonitoring(seconds, interval, width) {
			return false, "E_USER_CONSENT_REQUIRED", "local user declined or confirmation dialog unavailable", nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		a.screenJob.mu.Lock()
		if a.screenJob.active {
			a.screenJob.mu.Unlock()
			cancel()
			return false, "E_DEVICE_BUSY", "AI screen monitor already running", nil
		}
		a.screenJob.active, a.screenJob.frames, a.screenJob.cancel, a.screenJob.done = true, 0, cancel, make(chan struct{})
		a.screenJob.lastAnalysis = nil
		a.screenJob.lastError = ""
		done := a.screenJob.done
		a.screenJob.mu.Unlock()
		go a.runScreenMonitor(ctx, done, msgID, seconds, interval, width, prog)
		<-done
		a.screenJob.mu.Lock()
		defer a.screenJob.mu.Unlock()
		if ctx.Err() != nil {
			return true, "", "", map[string]any{"active": false, "stopped": true, "frames": a.screenJob.frames, "lastAnalysis": a.screenJob.lastAnalysis}
		}
		return true, "", "", map[string]any{"active": false, "frames": a.screenJob.frames, "lastAnalysis": a.screenJob.lastAnalysis, "lastError": a.screenJob.lastError}
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown screen monitor command", nil
	}
}

func (a *Ability) runScreenMonitor(ctx context.Context, done chan struct{}, msgID string, seconds, interval, width int, prog func(map[string]any)) {
	defer close(done)
	defer func() {
		a.screenJob.mu.Lock()
		a.screenJob.active = false
		a.screenJob.cancel = nil
		a.screenJob.done = nil
		a.screenJob.mu.Unlock()
	}()
	root := filepath.Join(os.Getenv("LOCALAPPDATA"), "ClassAegis", "screen-monitor")
	if err := os.MkdirAll(root, 0700); err != nil {
		a.screenJob.mu.Lock()
		a.screenJob.lastError = err.Error()
		a.screenJob.mu.Unlock()
		return
	}
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for frameNo := 1; time.Now().Before(deadline); frameNo++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		path := filepath.Join(root, fmt.Sprintf("frame-%d-%d.jpg", time.Now().UnixNano(), frameNo))
		captureErr := captureForegroundMediaWindow(path, width)
		if captureErr == nil {
			analysis, err := a.upload(msgID, path, map[string]any{"frame": frameNo, "capturedAt": time.Now().UTC().Format(time.RFC3339), "width": width, "source": "foreground-media-player-window"})
			_ = os.Remove(path)
			if err != nil {
				a.screenJob.mu.Lock()
				a.screenJob.lastError = err.Error()
				a.screenJob.mu.Unlock()
				if prog != nil {
					prog(map[string]any{"type": "ai_screen_monitor", "frame": frameNo, "uploaded": false, "error": err.Error()})
				}
				return
			}
			a.screenJob.mu.Lock()
			a.screenJob.frames++
			a.screenJob.lastAnalysis = analysis
			frames := a.screenJob.frames
			a.screenJob.mu.Unlock()
			if prog != nil {
				prog(map[string]any{"type": "ai_screen_monitor", "frame": frames, "uploaded": true, "analysis": analysis})
			}
		} else if !errors.Is(captureErr, errNoMediaWindow) {
			a.screenJob.mu.Lock()
			a.screenJob.lastError = captureErr.Error()
			a.screenJob.mu.Unlock()
			_ = os.Remove(path)
			return
		}
		if captureErr != nil {
			_ = os.Remove(path)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(interval) * time.Second):
		}
	}
	a.screenJob.mu.Lock()
	if a.screenJob.frames == 0 && a.screenJob.lastError == "" && ctx.Err() == nil {
		a.screenJob.lastError = "no supported media player was in the foreground"
	}
	a.screenJob.mu.Unlock()
}

func (a *Ability) screenStatus() map[string]any {
	a.screenJob.mu.Lock()
	defer a.screenJob.mu.Unlock()
	return map[string]any{"active": a.screenJob.active, "frames": a.screenJob.frames, "lastAnalysis": a.screenJob.lastAnalysis, "lastError": a.screenJob.lastError}
}

func intParam(m map[string]any, key string, fallback int) int {
	switch n := m[key].(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return fallback
}
