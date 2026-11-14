// SPDX-License-Identifier: AGPL-3.0-only
package ability

import (
	"encoding/json"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/platform"
)

// desktopAbility is the built-in desktop capability module: the five M0
// commands executed via the platform adapter.
type desktopAbility struct {
	p      platform.Platform
	stream *streamPlayer
}

// NewDesktop returns the built-in desktop ability backed by a platform.
func NewDesktop(p platform.Platform) Ability {
	return &desktopAbility{p: p, stream: newStreamPlayer("")}
}

func NewDesktopWithMpv(p platform.Platform, mpvPath string) Ability {
	return &desktopAbility{p: p, stream: newStreamPlayer(mpvPath)}
}

func (d *desktopAbility) Name() string { return "desktop" }

func (d *desktopAbility) Types() []string {
	return []string{"LOCK_SCREEN", "UNLOCK_SCREEN", "SEND_MESSAGE", "SET_VOLUME", "REBOOT", "AUDIO_SET_VOLUME", "AUDIO_SET_MUTE", "AUDIO_VOICE_MESSAGE", "CAMERA_CAPTURE_START", "CAMERA_CAPTURE_STOP", "CAMERA_STATUS", "DISPLAY_SET_MODE", "APP_LAUNCH", "STREAM_START", "STREAM_STOP", "STREAM_STATUS"}
}

// Handle executes one of the five desktop commands via the platform.
func (d *desktopAbility) Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (ok bool, code string, message string, result map[string]any) {
	switch msgType {
	case "STREAM_START":
		result, code, err := d.stream.start(payload)
		if err != nil {
			return false, code, err.Error(), nil
		}
		return true, "", "", result
	case "STREAM_STOP":
		result, code, err := d.stream.stop()
		if err != nil {
			return false, code, err.Error(), nil
		}
		return true, "", "", result
	case "STREAM_STATUS":
		return true, "", "", d.stream.status()
	case "LOCK_SCREEN":
		if err := d.p.Lock(); err != nil {
			return false, "E_PLATFORM_UNSUPPORTED", err.Error(), nil
		}
	case "UNLOCK_SCREEN":
		if err := d.p.Unlock(); err != nil {
			return false, "E_FAILED", err.Error(), nil
		}
	case "SEND_MESSAGE":
		title := fieldStr(payload, "title")
		body := fieldStr(payload, "body")
		ttl := int(fieldInt(payload, "ttlSeconds", 10))
		if err := d.p.Notify(title, body, ttl); err != nil {
			return false, "E_PLATFORM_UNSUPPORTED", err.Error(), nil
		}
	case "SET_VOLUME":
		vol := int(fieldInt(payload, "volume", 50))
		if err := d.p.SetVolume(vol); err != nil {
			return false, "E_FAILED", err.Error(), nil
		}
	case "AUDIO_SET_VOLUME":
		flow := fieldStr(payload, "flow")
		if flow == "" {
			flow = "render"
		}
		vol := int(fieldInt(payload, "volume", -1))
		if vol < 0 || vol > 100 || (flow != "render" && flow != "capture") {
			return false, "E_PARAM", "flow must be render|capture and volume 0..100", nil
		}
		if err := d.p.SetAudioVolume(flow, vol); err != nil {
			return false, "E_FAILED", err.Error(), nil
		}
	case "AUDIO_SET_MUTE":
		flow := fieldStr(payload, "flow")
		if flow == "" {
			flow = "render"
		}
		mute, ok := payload["mute"].(bool)
		if !ok || (flow != "render" && flow != "capture") {
			return false, "E_PARAM", "flow must be render|capture and mute must be boolean", nil
		}
		if err := d.p.SetAudioMute(flow, mute); err != nil {
			return false, "E_FAILED", err.Error(), nil
		}
	case "AUDIO_VOICE_MESSAGE":
		duration := int(fieldInt(payload, "durationSeconds", 0))
		if duration < 1 || duration > 60 {
			return false, "E_PARAM", "durationSeconds must be 1..60", nil
		}
		if consent, ok := payload["consent"].(bool); !ok || !consent {
			return false, "E_USER_CONSENT_REQUIRED", "consent=true is required; a local confirmation dialog will also be shown", nil
		}
		path, size, err := d.p.RecordVoiceMessage(duration)
		if err != nil {
			if len(err.Error()) >= len("E_USER_CONSENT_REQUIRED") && err.Error()[:len("E_USER_CONSENT_REQUIRED")] == "E_USER_CONSENT_REQUIRED" {
				return false, "E_USER_CONSENT_REQUIRED", err.Error(), nil
			}
			return false, "E_FAILED", err.Error(), nil
		}
		return true, "", "", map[string]any{"file": path, "bytes": size, "durationSeconds": duration}
	case "CAMERA_CAPTURE_START":
		return false, "E_PLATFORM_UNSUPPORTED", "DirectShow/Media Foundation camera capture backend is not included in this build", nil
	case "CAMERA_CAPTURE_STOP", "CAMERA_STATUS":
		return true, "", "", map[string]any{"active": false}
	case "DISPLAY_SET_MODE":
		width, height := int(fieldInt(payload, "width", -1)), int(fieldInt(payload, "height", -1))
		refresh := int(fieldInt(payload, "refreshRate", 0))
		if width < 640 || height < 480 {
			return false, "E_PARAM", "width/height are required", nil
		}
		if err := d.p.SetDisplayResolution(fieldStr(payload, "device"), width, height, refresh); err != nil {
			return false, "E_FAILED", err.Error(), nil
		}
	case "APP_LAUNCH":
		app := fieldStr(payload, "app")
		if app == "" {
			return false, "E_PARAM", "app is required", nil
		}
		if err := d.p.LaunchApplication(app); err != nil {
			return false, "E_FAILED", err.Error(), nil
		}
	case "REBOOT":
		if err := d.p.Reboot(); err != nil {
			return false, "E_PLATFORM_UNSUPPORTED", err.Error(), nil
		}
	}
	return true, "", "", nil
}

// --- payload field helpers (values may be string or json.Number) ---

func fieldStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func fieldInt(m map[string]any, key string, def int64) int64 {
	switch v := m[key].(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	}
	return def
}
