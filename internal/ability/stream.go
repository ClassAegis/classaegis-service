package ability

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// streamPlayer owns the single interactive mpv window used for巡课 playback.
// It receives an already verified command from Core and never invokes a shell.
type streamPlayer struct {
	mu   sync.Mutex
	path string
	cmd  *exec.Cmd
	url  string
}

func newStreamPlayer(configured string) *streamPlayer {
	return &streamPlayer{path: resolveMpvPath(configured)}
}

func resolveMpvPath(configured string) string {
	if configured != "" {
		return configured
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		for _, p := range []string{filepath.Join(base, "mpv.exe"), filepath.Join(base, "bin", "mpv.exe")} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return "mpv.exe"
}

func (s *streamPlayer) start(payload map[string]any) (map[string]any, string, error) {
	streamURL := fieldStr(payload, "url")
	if err := validateStreamURL(streamURL); err != nil {
		return nil, "E_PARAM", err
	}
	if raw, exists := payload["volume"]; exists {
		volume := int(fieldInt(payload, "volume", -1))
		if _, ok := raw.(string); ok || volume < 0 || volume > 100 {
			return nil, "E_PARAM", fmt.Errorf("volume must be an integer from 0 to 100")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return nil, "E_STREAM_ACTIVE", fmt.Errorf("a stream is already playing")
	}
	args := []string{"--no-config", "--force-window=immediate", "--terminal=no", "--really-quiet"}
	if fullscreen, ok := payload["fullscreen"].(bool); ok && fullscreen {
		args = append(args, "--fullscreen")
	}
	if volume := int(fieldInt(payload, "volume", -1)); volume >= 0 && volume <= 100 {
		args = append(args, fmt.Sprintf("--volume=%d", volume))
	}
	args = append(args, streamURL)
	cmd := exec.Command(s.path, args...)
	if err := cmd.Start(); err != nil {
		return nil, "E_FAILED", fmt.Errorf("start mpv: %w", err)
	}
	s.cmd, s.url = cmd, redactURL(streamURL)
	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		if s.cmd == cmd {
			s.cmd, s.url = nil, ""
		}
		s.mu.Unlock()
	}()
	return map[string]any{"active": true, "pid": cmd.Process.Pid, "url": s.url, "executable": s.path}, "", nil
}

func (s *streamPlayer) stop() (map[string]any, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return map[string]any{"active": false}, "", nil
	}
	if err := s.cmd.Process.Kill(); err != nil {
		return nil, "E_FAILED", fmt.Errorf("stop mpv: %w", err)
	}
	pid := s.cmd.Process.Pid
	s.cmd, s.url = nil, ""
	return map[string]any{"active": false, "pid": pid}, "", nil
}

func (s *streamPlayer) status() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.cmd != nil && s.cmd.Process != nil
	result := map[string]any{"active": active, "executable": s.path}
	if active {
		result["pid"], result["url"] = s.cmd.Process.Pid, s.url
	}
	return result
}

func validateStreamURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("url must be an absolute RTMP/RTP URL")
	}
	switch strings.ToLower(u.Scheme) {
	case "rtmp", "rtp":
	default:
		return fmt.Errorf("unsupported stream protocol %q", u.Scheme)
	}
	if u.User != nil || u.Fragment != "" {
		return fmt.Errorf("stream URL must not contain credentials or fragments")
	}
	return nil
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	u.RawQuery = ""
	return u.String()
}
