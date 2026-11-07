package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/ipc"
)

// workerAlive tracks the last worker registration timestamp (unix seconds).
var workerAlive int64

// ServeIPC runs the core process loopback server:
//   - POST /register: worker announces itself (token-authed)
//   - GET  /status:   snapshot for the future Electron UI
func (a *Agent) ServeIPC(token string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", func(rw http.ResponseWriter, r *http.Request) {
		if !ipc.CheckToken(r, token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		atomic.StoreInt64(&workerAlive, time.Now().Unix())
		rw.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/status", func(rw http.ResponseWriter, r *http.Request) {
		if !ipc.CheckToken(r, token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		alive := time.Now().Unix()-atomic.LoadInt64(&workerAlive) < 90
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"sn":         a.cfg.Device.SN,
			"schoolCode": a.cfg.Device.SchoolCode,
			"connected":  a.client != nil && a.client.IsConnectionOpen(),
			"workerAlive": alive,
			"agentVersion": "0.1.0",
			"protocolVersion": "0.1",
		})
	})
	addr := "127.0.0.1:" + strconv.Itoa(ipc.CorePort)
	server := &http.Server{Addr: addr, Handler: mux}
	return server.ListenAndServe()
}
