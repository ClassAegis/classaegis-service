package worker

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ipc"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/platform"
)

// Worker is the business process: executes verified commands via the
// platform adapter. Loopback-only; no external network access.
type Worker struct {
	platform platform.Platform
	token    string
}

func New(p platform.Platform, token string) *Worker {
	return &Worker{platform: p, token: token}
}

// ExecuteCommand runs one command against the platform. Shared by the worker
// process and the all-in-one debug mode.
func ExecuteCommand(p platform.Platform, msgType string, payload map[string]any) (bool, string, string) {
	switch msgType {
	case "LOCK_SCREEN":
		if err := p.Lock(); err != nil {
			return false, "E_PLATFORM_UNSUPPORTED", err.Error()
		}
	case "UNLOCK_SCREEN":
		if err := p.Unlock(); err != nil {
			return false, "E_FAILED", err.Error()
		}
	case "SEND_MESSAGE":
		title := fieldStr(payload, "title")
		body := fieldStr(payload, "body")
		ttl := int(fieldInt(payload, "ttlSeconds", 10))
		if err := p.Notify(title, body, ttl); err != nil {
			return false, "E_PLATFORM_UNSUPPORTED", err.Error()
		}
	case "SET_VOLUME":
		vol := int(fieldInt(payload, "volume", 50))
		if err := p.SetVolume(vol); err != nil {
			return false, "E_FAILED", err.Error()
		}
	case "REBOOT":
		if err := p.Reboot(); err != nil {
			return false, "E_PLATFORM_UNSUPPORTED", err.Error()
		}
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType
	}
	return true, "", ""
}

// Serve runs the loopback HTTP server on 127.0.0.1:39818 (POST /execute).
func (w *Worker) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/execute", func(rw http.ResponseWriter, r *http.Request) {
		if !ipc.CheckToken(r, w.token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		var req ipc.ExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}
		ok, code, message := ExecuteCommand(w.platform, req.Type, req.Payload)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(ipc.ExecuteResponse{OK: ok, Code: code, Message: message})
	})
	addr := "127.0.0.1:" + strconv.Itoa(ipc.WorkerPort)
	log.Printf("[worker] listening on %s", addr)
	server := &http.Server{Addr: addr, Handler: mux}
	return server.ListenAndServe()
}

// RegisterToCore announces the worker to the core process (best effort).
func (w *Worker) RegisterToCore(token string) error {
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:39817/register", nil)
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
	return nil
}

func fieldStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func fieldInt(m map[string]any, key string, def int64) int64 {
	if v, ok := m[key].(json.Number); ok {
		if i, err := v.Int64(); err == nil {
			return i
		}
	}
	return def
}
