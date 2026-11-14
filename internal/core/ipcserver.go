// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core/netops"
	"github.com/OneClassMesh/oneclassmesh-agent/internal/ipc"
)

// abilityAlive tracks the last ability-process registration timestamp
// (unix seconds).
var abilityAlive int64

// ServeIPC serves the core process loopback endpoints:
//   - POST /register: ability process announces itself (token-authed)
//   - POST /event:    ability process pushes progress events (token-authed);
//     core publishes PROGRESS receipts on cmd/up
//   - GET  /status:   snapshot for the desktop UI
func (a *Agent) ServeIPC(token string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.Header().Set("Allow", http.MethodPost)
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !ipc.CheckToken(r, token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		atomic.StoreInt64(&abilityAlive, time.Now().Unix())
		rw.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/event", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.Header().Set("Allow", http.MethodPost)
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !ipc.CheckToken(r, token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		var ev ipc.EventRequest
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil || ev.MsgID == "" {
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}
		jsonStr, err := BuildReceipt(ev.MsgID, "PROGRESS", ev.Result, "", "")
		if err != nil {
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}
		a.publishReceipt(jsonStr)
		rw.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/netexec", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.Header().Set("Allow", http.MethodPost)
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !ipc.CheckToken(r, token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		var req ipc.NetExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Op == "" || req.Args == nil {
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}
		result, err := netops.Execute(req.Op, req.Args)
		rw.Header().Set("Content-Type", "application/json")
		resp := ipc.NetExecResponse{OK: err == nil}
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Result = result
		}
		_ = json.NewEncoder(rw).Encode(resp)
	})
	mux.HandleFunc("/media/upload", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !ipc.CheckToken(r, token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		var req ipc.MediaUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MsgID == "" || req.Path == "" {
			http.Error(rw, "bad request", http.StatusBadRequest)
			return
		}
		if a.cfg.Agent.MediaUploadURL == "" {
			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(ipc.MediaUploadResponse{OK: false, Error: "media_upload_url is not configured"})
			return
		}
		captureRoot := filepath.Clean(filepath.Join(os.Getenv("LOCALAPPDATA"), "ClassAegis", "screen-monitor"))
		cleanPath := filepath.Clean(req.Path)
		if !strings.HasPrefix(strings.ToLower(cleanPath), strings.ToLower(captureRoot+string(os.PathSeparator))) {
			http.Error(rw, "path is outside screen-monitor capture directory", http.StatusBadRequest)
			return
		}
		file, err := os.Open(req.Path)
		if err != nil {
			_ = json.NewEncoder(rw).Encode(ipc.MediaUploadResponse{OK: false, Error: err.Error()})
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() < 4 || info.Size() > 512*1024 {
			http.Error(rw, "frame must be a regular file no larger than 512 KiB", http.StatusBadRequest)
			return
		}
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("frame", "frame.jpg")
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if err == nil {
			meta, _ := json.Marshal(req.Meta)
			_ = mw.WriteField("meta", string(meta))
			_ = mw.WriteField("msgId", req.MsgID)
		}
		_ = mw.Close()
		if err != nil {
			_ = json.NewEncoder(rw).Encode(ipc.MediaUploadResponse{OK: false, Error: err.Error()})
			return
		}
		up, err := http.NewRequest(http.MethodPost, a.cfg.Agent.MediaUploadURL, &body)
		if err != nil {
			_ = json.NewEncoder(rw).Encode(ipc.MediaUploadResponse{OK: false, Error: err.Error()})
			return
		}
		up.Header.Set("Content-Type", mw.FormDataContentType())
		if a.cfg.Agent.MediaUploadToken != "" {
			up.Header.Set("Authorization", "Bearer "+a.cfg.Agent.MediaUploadToken)
		}
		resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(up)
		if err != nil {
			_ = json.NewEncoder(rw).Encode(ipc.MediaUploadResponse{OK: false, Error: err.Error()})
			return
		}
		defer resp.Body.Close()
		var result map[string]any
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
				_ = json.NewEncoder(rw).Encode(ipc.MediaUploadResponse{OK: false, Error: "upload service returned invalid JSON: " + err.Error()})
				return
			}
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(ipc.MediaUploadResponse{OK: resp.StatusCode >= 200 && resp.StatusCode < 300, Result: result})
	})
	mux.HandleFunc("/status", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.Header().Set("Allow", http.MethodGet)
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !ipc.CheckToken(r, token) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		alive := time.Now().Unix()-atomic.LoadInt64(&abilityAlive) < 90
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"sn":              a.cfg.Device.SN,
			"schoolCode":      a.cfg.Device.SchoolCode,
			"connected":       a.client != nil && a.client.IsConnectionOpen(),
			"abilityAlive":    alive,
			"agentVersion":    "0.1.0",
			"protocolVersion": "0.1",
		})
	})
	addr := "127.0.0.1:" + strconv.Itoa(ipc.CorePort)
	server := &http.Server{Addr: addr, Handler: mux}
	return server.ListenAndServe()
}
