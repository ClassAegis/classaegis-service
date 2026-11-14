// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ipc"
)

// IPCExecutor forwards verified commands to the ability process over
// loopback. Loopback-only: the ability process has no external network
// access.
type IPCExecutor struct {
	token string
}

func NewIPCExecutor(token string) *IPCExecutor {
	return &IPCExecutor{token: token}
}

// Execute sends one command to the ability process and waits for the
// result. The loopback window comes from ipc.CommandTimeout, so
// long-running types (scans) get proportionally long windows; on transport
// timeout it reports E_ABILITY_OFFLINE (receipt FAILED). REBOOT keeps 120s.
func (e *IPCExecutor) Execute(msgId, msgType string, payload map[string]any) (bool, string, string, map[string]any) {
	timeout := ipc.CommandTimeout(msgType)
	client := &http.Client{Timeout: timeout}

	body, err := MarshalCanonical(ipc.ExecuteRequest{MsgID: msgId, Type: msgType, Payload: payload})
	if err != nil {
		return false, "E_FAILED", "marshal request failed: " + err.Error(), nil
	}
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(ipc.AbilityPort)+"/execute", bytes.NewReader([]byte(body)))
	if err != nil {
		return false, "E_FAILED", "build request failed: " + err.Error(), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ipc.TokenHeader, e.token)

	resp, err := client.Do(req)
	if err != nil {
		return false, "E_ABILITY_OFFLINE", "ability unreachable: " + err.Error(), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false, "E_ABILITY_OFFLINE", "ability returned HTTP " + strconv.Itoa(resp.StatusCode), nil
	}

	var out ipc.ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, "E_FAILED", "decode ability response failed: " + err.Error(), nil
	}
	return out.OK, out.Code, out.Message, out.Result
}
