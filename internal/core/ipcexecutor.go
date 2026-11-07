package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ipc"
)

// IPCExecutor forwards verified commands to the worker process over loopback.
// Loopback-only: worker has no external network access.
type IPCExecutor struct {
	token string
}

func NewIPCExecutor(token string) *IPCExecutor {
	return &IPCExecutor{token: token}
}

// Execute sends one command to the worker and waits for the result.
// Per-type timeout doubles as the command timeout: on transport timeout it
// reports E_WORKER_OFFLINE (receipt FAILED). REBOOT gets a longer window.
func (e *IPCExecutor) Execute(msgId, msgType string, payload map[string]any) (bool, string, string) {
	timeout := 30 * time.Second
	if msgType == "REBOOT" {
		timeout = 120 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	body, err := MarshalCanonical(ipc.ExecuteRequest{MsgID: msgId, Type: msgType, Payload: payload})
	if err != nil {
		return false, "E_FAILED", "marshal request failed: " + err.Error()
	}
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:39818/execute", bytes.NewReader([]byte(body)))
	if err != nil {
		return false, "E_FAILED", "build request failed: " + err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ipc.TokenHeader, e.token)

	resp, err := client.Do(req)
	if err != nil {
		return false, "E_WORKER_OFFLINE", "worker unreachable: " + err.Error()
	}
	defer resp.Body.Close()

	var out ipc.ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, "E_FAILED", "decode worker response failed: " + err.Error()
	}
	return out.OK, out.Code, out.Message
}
