// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows
// +build windows

package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// readESE reads one record kind from a recovered WebCache copy.
func (e *Engine) readESE(kind string, limit int) ([]record, error) {
	dbPath, avail := webcachePath()
	if !avail && dbPath == "" {
		return nil, errNoSource
	}
	dir := e.sourceDir("ese")
	_ = os.RemoveAll(dir) // fresh copy each query
	var js *jetSession
	var err error
	if avail {
		js, err = openESECopy(dbPath, dir)
	} else {
		err = errNoSource
	}
	if err != nil {
		return nil, err
	}
	defer js.Close()
	switch kind {
	case "history":
		return js.readHistory(limit)
	case "download":
		return js.readDownloads(limit)
	case "cache":
		return js.readCache(limit)
	}
	return nil, errUnknownKind
}

// readChromium copies one profile's History file and parses the requested
// table. Chromium locks the live file, hence copy-then-parse.
func (e *Engine) readChromium(p chromiumProfile, kind string, limit int) ([]record, error) {
	dir := e.sourceDir(p.Source + "-" + p.Profile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	dst := filepath.Join(dir, "History")
	_ = os.Remove(dst)
	if err := copyFile(p.Path, dst); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		return nil, err
	}
	db, err := openSQLite(raw)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	switch kind {
	case "history":
		rows, err := db.selectRows("urls", limit)
		if err != nil {
			return nil, err
		}
		var out []record
		for _, r := range rows {
			url, _ := r["url"].(string)
			if url == "" {
				continue
			}
			rec := record{
				Source:  p.Source,
				Profile: p.Profile,
				Kind:    "history",
				URL:     url,
				Title:   strOf(r["title"]),
				Visits:  intOf(r["visit_count"]),
			}
			if us, ok := r["last_visit_time"].(int64); ok {
				rec.Time = chromiumTime(us)
			}
			out = append(out, rec)
		}
		return out, nil
	case "download":
		rows, err := db.selectRows("downloads", limit)
		if err != nil {
			return nil, err
		}
		var out []record
		for _, r := range rows {
			rec := record{
				Source:  p.Source,
				Profile: p.Profile,
				Kind:    "download",
				URL:     strOf(r["original_url"]),
				Path:    strOf(r["target_path"]),
				Title:   strOf(r["mime_type"]),
			}
			if us, ok := r["start_time"].(int64); ok {
				rec.Time = chromiumTime(us)
			}
			if n, ok := r["received_bytes"].(int64); ok {
				rec.Size = n
			}
			out = append(out, rec)
		}
		return out, nil
	case "cache":
		return nil, errCacheUnsupported
	}
	return nil, errUnknownKind
}

// copyFile copies src to dst (full read into memory; audit files are small).
func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o600)
}

// strOf renders any value as a trimmed string.
func strOf(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

// intOf coerces any numeric-ish value to int64.
func intOf(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	default:
		return 0
	}
}
