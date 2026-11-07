// Package ipc defines the loopback contract between core and worker.
// Loopback-only, token-authed; the core process is the only component
// with external network access.
package ipc

import "net/http"

const (
	// CorePort serves /register /status on loopback.
	CorePort = 39817
	// WorkerPort serves /execute on loopback.
	WorkerPort = 39818
	// TokenHeader carries the shared loopback token.
	TokenHeader = "X-Ocm-Token"
)

// ExecuteRequest is sent by core to worker for one verified command.
type ExecuteRequest struct {
	MsgID   string         `json:"msgId"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// ExecuteResponse is returned by worker after execution.
type ExecuteResponse struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// CheckToken verifies the shared token on loopback requests.
func CheckToken(r *http.Request, token string) bool {
	return r.Header.Get(TokenHeader) == token
}
