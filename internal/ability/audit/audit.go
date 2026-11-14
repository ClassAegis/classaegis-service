// SPDX-License-Identifier: AGPL-3.0-only
// Package audit implements the web behavior audit ability (上网行为审计):
// browser history / downloads / cache-entry reading from local browser
// databases. ESE (WebCacheV01.dat, IE/legacy Edge) via esent.dll on a
// recovered copy; Chromium (Edge/Chrome) SQLite History via a minimal
// read-only parser. Loopback-only process; records go to the collector for
// authorized review only.
package audit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Options configures the audit ability; zero values apply defaults.
type Options struct {
	WorkDir    string // temp copies live here; 0 = ProgramData\oneclassmesh\audit\work
	MaxRecords int    // per-source result cap; 0 = 200, max 1000
}

// Engine is the web behavior audit ability.
type Engine struct {
	opts Options
	mu   sync.Mutex
}

// record is one audit row (history / download / cache entry).
type record struct {
	Source  string `json:"source"` // ese | edge | chrome
	Profile string `json:"profile,omitempty"`
	URL     string `json:"url"`
	Title   string `json:"title,omitempty"`
	Time    string `json:"time,omitempty"`
	Visits  int64  `json:"visits,omitempty"`
	Path    string `json:"path,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Kind    string `json:"kind"` // history | download | cache
}

// New creates the engine.
func New(opts Options) *Engine {
	if opts.WorkDir == "" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = "."
		}
		opts.WorkDir = filepath.Join(base, "oneclassmesh", "audit", "work")
	}
	if opts.MaxRecords <= 0 {
		opts.MaxRecords = 200
	}
	if opts.MaxRecords > 1000 {
		opts.MaxRecords = 1000
	}
	return &Engine{opts: opts}
}

// Name implements ability.Ability.
func (e *Engine) Name() string { return "audit" }

// Types implements ability.Ability; documented in the protocol spec §6.6.
func (e *Engine) Types() []string {
	return []string{"AUDIT_SOURCES", "AUDIT_BROWSE", "AUDIT_DOWNLOADS", "AUDIT_CACHE"}
}

// Handle routes one audit command.
func (e *Engine) Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	switch msgType {
	case "AUDIT_SOURCES":
		return e.handleSources()
	case "AUDIT_BROWSE":
		return e.handleBrowse(payload)
	case "AUDIT_DOWNLOADS", "AUDIT_CACHE":
		return e.handleRecords(msgType, payload)
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType, nil
	}
}

// sourceDir returns the per-source work subdirectory.
func (e *Engine) sourceDir(source string) string {
	return filepath.Join(e.opts.WorkDir, source)
}

// webcachePath: IE/legacy-Edge ESE database location.
func webcachePath() (string, bool) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", false
	}
	p := filepath.Join(local, "Microsoft", "Windows", "WebCache", "WebCacheV01.dat")
	if _, err := os.Stat(p); err != nil {
		return p, false
	}
	return p, true
}

// chromiumProfiles discovers Edge/Chrome History databases across profiles.
func chromiumProfiles() []chromiumProfile {
	var out []chromiumProfile
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return out
	}
	browsers := []struct{ source, root string }{
		{"edge", `Microsoft\Edge\User Data`},
		{"chrome", `Google\Chrome\User Data`},
	}
	for _, b := range browsers {
		root := filepath.Join(local, b.root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			hist := filepath.Join(root, ent.Name(), "History")
			if _, err := os.Stat(hist); err == nil {
				out = append(out, chromiumProfile{Source: b.source, Profile: ent.Name(), Path: hist})
			}
		}
	}
	return out
}

type chromiumProfile struct {
	Source  string
	Profile string
	Path    string
}

// parseSourceLimit reads the payload source selector and row limit.
func (e *Engine) parseSourceLimit(payload map[string]any) (string, int) {
	src, _ := payload["source"].(string)
	if src == "" {
		src = "all"
	}
	limit := e.opts.MaxRecords
	if n, ok := payload["limit"].(json.Number); ok {
		if i, err := n.Int64(); err == nil && i > 0 {
			limit = int(i)
		}
	}
	if limit > e.opts.MaxRecords {
		limit = e.opts.MaxRecords
	}
	return src, limit
}

// handleSources enumerates local audit data sources.
func (e *Engine) handleSources() (bool, string, string, map[string]any) {
	var sources []map[string]any
	path, avail := webcachePath()
	sources = append(sources, map[string]any{"source": "ese", "path": path, "available": avail})
	for _, p := range chromiumProfiles() {
		sources = append(sources, map[string]any{
			"source": p.Source, "profile": p.Profile, "path": p.Path, "available": true,
		})
	}
	return true, "", "", map[string]any{"sources": sources}
}

// handleBrowse merges history records from the selected sources.
func (e *Engine) handleBrowse(payload map[string]any) (bool, string, string, map[string]any) {
	src, limit := e.parseSourceLimit(payload)
	var records []record
	var errorsList []string
	if src == "all" || src == "ese" {
		recs, err := e.readESE("history", limit)
		if err != nil {
			errorsList = append(errorsList, "ese: "+err.Error())
		} else {
			records = append(records, recs...)
		}
	}
	if src == "all" || src == "chromium" || src == "edge" || src == "chrome" {
		for _, p := range chromiumProfiles() {
			if src != "all" && src != "chromium" && p.Source != src {
				continue
			}
			recs, err := e.readChromium(p, "history", limit)
			if err != nil {
				errorsList = append(errorsList, p.Source+"/"+p.Profile+": "+err.Error())
				continue
			}
			records = append(records, recs...)
			if len(records) >= limit {
				break
			}
		}
	}
	if records == nil {
		records = []record{}
	}
	return true, "", "", map[string]any{
		"source": src, "records": records, "total": len(records), "errors": errorsList,
	}
}

// handleRecords routes downloads/cache queries.
func (e *Engine) handleRecords(msgType string, payload map[string]any) (bool, string, string, map[string]any) {
	kind := "download"
	if msgType == "AUDIT_CACHE" {
		kind = "cache"
	}
	src, limit := e.parseSourceLimit(payload)
	var records []record
	var errorsList []string
	if (src == "all" || src == "ese") && kind == "download" {
		recs, err := e.readESE(kind, limit)
		if err != nil {
			errorsList = append(errorsList, "ese: "+err.Error())
		} else {
			records = append(records, recs...)
		}
	}
	if (src == "all" || src == "ese") && kind == "cache" {
		recs, err := e.readESE(kind, limit)
		if err != nil {
			errorsList = append(errorsList, "ese: "+err.Error())
		} else {
			records = append(records, recs...)
		}
	}
	if (src == "all" || src == "chromium" || src == "edge" || src == "chrome") && kind == "download" {
		for _, p := range chromiumProfiles() {
			if src != "all" && src != "chromium" && p.Source != src {
				continue
			}
			recs, err := e.readChromium(p, kind, limit)
			if err != nil {
				errorsList = append(errorsList, p.Source+"/"+p.Profile+": "+err.Error())
				continue
			}
			records = append(records, recs...)
			if len(records) >= limit {
				break
			}
		}
	}
	if kind == "cache" && (src == "chromium" || src == "edge" || src == "chrome") {
		errorsList = append(errorsList, "cache is only available from the ese source (chromium block-file format out of scope)")
	}
	if records == nil {
		records = []record{}
	}
	return true, "", "", map[string]any{
		"source": src, "records": records, "total": len(records), "errors": errorsList,
	}
}

// audit package sentinel errors (source-level, not command failures).
var (
	errNoSource         = errors.New("source not available on this machine")
	errUnknownKind      = errors.New("unknown record kind")
	errCacheUnsupported = errors.New("cache reading is ESE-only in v0.1")
)
