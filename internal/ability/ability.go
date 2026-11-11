// Package ability hosts the business capability modules (能力模块) of
// 课佑管家. The ability process listens on loopback only and executes
// verified commands forwarded by core; it has NO external network access
// by design.
package ability

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ipc"
)

// Reporter forwards non-terminal progress of a running command to core
// (POST /event, which publishes a PROGRESS receipt on cmd/up). The
// all-in-one debug mode publishes directly via the in-process agent.
type Reporter func(msgId string, result map[string]any)

// Ability is one business capability module (能力模块). A module declares
// the command types it owns and executes them when the host routes verified
// commands to it. Modules have no network access; input is the server
// verified payload, output is (ok, code, message) plus optional progress.
type Ability interface {
	// Name is the stable module name, e.g. "desktop", "av".
	Name() string
	// Types returns the command types this ability owns.
	Types() []string
	// Handle executes one verified command of an owned type. prog posts
	// non-terminal progress (throttling is the module's responsibility);
	// result carries command-specific data for the receipt.
	Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (ok bool, code string, message string, result map[string]any)
}

// Host is the ability process core: a registry of capability modules plus
// the loopback HTTP server that core talks to.
type Host struct {
	token     string
	reporter  Reporter
	abilities []Ability
	route     map[string]Ability
}

// NewHost creates an empty host. reporter may be nil (progress discarded).
func NewHost(token string, reporter Reporter) *Host {
	if reporter == nil {
		reporter = func(string, map[string]any) {}
	}
	return &Host{token: token, reporter: reporter, route: map[string]Ability{}}
}

// Register adds a capability module; duplicate command types fail fast.
func (h *Host) Register(a Ability) error {
	for _, t := range a.Types() {
		if _, dup := h.route[t]; dup {
			return fmt.Errorf("command type %s already registered", t)
		}
		h.route[t] = a
	}
	h.abilities = append(h.abilities, a)
	return nil
}

// Abilities returns the registered module names (for logs and status).
func (h *Host) Abilities() []string {
	names := make([]string, 0, len(h.abilities))
	for _, a := range h.abilities {
		names = append(names, a.Name())
	}
	return names
}

// ExecuteCommand routes one verified command to its ability. Unknown types
// answer E_UNSUPPORTED_TYPE (protocol behavior for unknown types, §5.4).
func (h *Host) ExecuteCommand(msgId, msgType string, payload map[string]any) (bool, string, string, map[string]any) {
	a, found := h.route[msgType]
	if !found {
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType, nil
	}
	return a.Handle(msgId, msgType, payload, func(result map[string]any) {
		h.reporter(msgId, result)
	})
}

// Serve runs the loopback HTTP server: POST /execute (token-authed).
// Handlers run concurrently, so a long scan does not block stop/status
// commands arriving on other goroutines.
func (h *Host) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/execute", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.Header().Set("Allow", http.MethodPost)
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !ipc.CheckToken(r, h.token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		var req ipc.ExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MsgID == "" || req.Type == "" || req.Payload == nil {
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}
		ok, code, message, result := h.ExecuteCommand(req.MsgID, req.Type, req.Payload)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(ipc.ExecuteResponse{OK: ok, Code: code, Message: message, Result: result})
	})
	addr := "127.0.0.1:" + strconv.Itoa(ipc.AbilityPort)
	log.Printf("[ability] listening on %s", addr)
	server := &http.Server{Addr: addr, Handler: mux}
	return server.ListenAndServe()
}

// RegisterToCore announces the ability process to core (best effort).
func (h *Host) RegisterToCore(token string) error {
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(ipc.CorePort)+"/register", nil)
	if err != nil {
		return err
	}
	req.Header.Set(ipc.TokenHeader, token)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("core register returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// PostNetExec delegates one network operation to core /netexec (15s
// timeout; long probes use their own loopback windows per command type).
// Core is the only networked component.
func PostNetExec(token, op string, args map[string]any) (map[string]any, error) {
	body, err := json.Marshal(ipc.NetExecRequest{Op: op, Args: args})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(ipc.CorePort)+"/netexec", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ipc.TokenHeader, token)
	client := &http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("core netexec returned HTTP %d", resp.StatusCode)
	}
	var out ipc.NetExecResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, errors.New(out.Error)
	}
	return out.Result, nil
}

// PostEvent posts one progress event to core /event (best effort, 3s
// timeout). Core wraps it into a PROGRESS receipt published on cmd/up.
func PostEvent(token, msgId string, result map[string]any) error {
	body, err := json.Marshal(ipc.EventRequest{MsgID: msgId, Result: result})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(ipc.CorePort)+"/event", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ipc.TokenHeader, token)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("core event returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func PostMediaUpload(token, msgID, path string, meta map[string]any) (map[string]any, error) {
	body, err := json.Marshal(ipc.MediaUploadRequest{MsgID: msgID, Path: path, Meta: meta})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(ipc.CorePort)+"/media/upload", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ipc.TokenHeader, token)
	resp, err := (&http.Client{Timeout: 65 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out ipc.MediaUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, errors.New(out.Error)
	}
	return out.Result, nil
}
