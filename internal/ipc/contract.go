// Package ipc defines the loopback contract between core and the ability
// process. Loopback-only, token-authed; the core process is the only
// component with external network access.
package ipc

import (
	"net/http"
	"time"
)

const (
	// CorePort serves /register /status /event on loopback.
	CorePort = 39817
	// AbilityPort serves /execute on loopback.
	AbilityPort = 39818
	// TokenHeader carries the shared loopback token.
	TokenHeader = "X-Ocm-Token"
)

// ExecuteRequest is sent by core to the ability process for one verified
// command.
type ExecuteRequest struct {
	MsgID   string         `json:"msgId"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// ExecuteResponse is returned by the ability process after execution.
// Result carries command-specific data published in the receipt's result
// field (e.g. scan summary, quarantine lists).
type ExecuteResponse struct {
	OK      bool           `json:"ok"`
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
}

// NetExecRequest is posted by the ability process to core /netexec: one
// delegated network operation (WOL magic packet, subnet probe, ARP table).
// Core is the only component with external network access; the ability
// process stays loopback-only.
type NetExecRequest struct {
	Op   string         `json:"op"`   // wol | discover | arp
	Args map[string]any `json:"args"` // operation arguments
}

// NetExecResponse carries the delegated operation result.
type NetExecResponse struct {
	OK     bool           `json:"ok"`
	Error  string         `json:"error,omitempty"`
	Result map[string]any `json:"result,omitempty"`
}

type MediaUploadRequest struct {
	MsgID string         `json:"msgId"`
	Path  string         `json:"path"`
	Meta  map[string]any `json:"meta"`
}

type MediaUploadResponse struct {
	OK     bool           `json:"ok"`
	Error  string         `json:"error,omitempty"`
	Result map[string]any `json:"result,omitempty"`
}

// EventRequest is posted by the ability process to core /event for
// non-terminal progress of a running command; core wraps it into a
// PROGRESS receipt published on cmd/up.
type EventRequest struct {
	MsgID  string         `json:"msgId"`
	Result map[string]any `json:"result"`
}

// CheckToken verifies the shared token on loopback requests.
func CheckToken(r *http.Request, token string) bool {
	return r.Header.Get(TokenHeader) == token
}

// CommandTimeout returns the loopback execution window per command type.
// Long-running types get proportionally long windows; REBOOT keeps its
// historical 120s window. Default 30s.
func CommandTimeout(msgType string) time.Duration {
	switch msgType {
	case "REBOOT":
		return 120 * time.Second
	case "AV_SCAN":
		return 4 * time.Hour
	case "AV_DB_UPDATE":
		return 10 * time.Minute
	case "CLEAN_SCAN", "CLEAN_RUN":
		return time.Hour
	case "PROC_SPEEDUP":
		return 60 * time.Second
	case "PROC_USAGE_REPORT":
		return 6 * time.Minute
	case "AUDIT_BROWSE", "AUDIT_DOWNLOADS", "AUDIT_CACHE":
		return 60 * time.Second
	case "WOL_WAKE":
		return 15 * time.Second
	case "WOL_DISCOVER":
		return 60 * time.Second
	case "WOL_ARP":
		return 30 * time.Second
	case "AUDIO_VOICE_MESSAGE", "CAMERA_CAPTURE_START":
		return 90 * time.Second
	case "NET_CAPTURE_START":
		return 90 * time.Second
	case "AI_SCREEN_MONITOR_START":
		return 11 * time.Minute
	case "STREAM_START", "STREAM_STOP", "STREAM_STATUS":
		return 30 * time.Second
	default:
		return 30 * time.Second
	}
}
