// SPDX-License-Identifier: AGPL-3.0-only
package av

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// runScan executes one scan to completion. Called synchronously from
// handleScan; progress is posted through prog (throttled in maybeProgress).
func (e *Engine) runScan(run *scanRun, opts scanOptions, prog func(map[string]any)) (map[string]any, bool, string, string) {
	start := time.Now()
	log.Printf("[av] scan %s started: mode=%s roots=%v on_detect=%s", run.id, opts.mode, opts.roots, opts.onDetect)

	files := make(chan string, 256)
	var wg sync.WaitGroup

	// walker: feeds candidate files into the channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(files)
		for _, root := range opts.roots {
			if run.ctx.Err() != nil {
				return
			}
			_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					if d != nil && d.IsDir() {
						run.skippedErrors.Add(1)
						return fs.SkipDir
					}
					run.skippedErrors.Add(1)
					return nil
				}
				if run.ctx.Err() != nil {
					return fs.SkipAll
				}
				if d.IsDir() {
					if path != root && e.excluded(path) {
						return fs.SkipDir
					}
					return nil
				}
				if !d.Type().IsRegular() {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					run.skippedErrors.Add(1)
					return nil
				}
				if info.Size() > int64(e.opts.MaxFileSizeMB)*1024*1024 {
					run.skippedLarge.Add(1)
					return nil
				}
				run.totalSeen.Add(1)
				select {
				case files <- path:
					return nil
				case <-run.ctx.Done():
					return fs.SkipAll
				}
			})
		}
	}()

	// hashers: hash + match + act, in parallel
	for i := 0; i < e.opts.Threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-run.ctx.Done():
					return
				case path, ok := <-files:
					if !ok {
						return
					}
					e.inspect(run, path, opts.onDetect)
					e.maybeProgress(run, prog)
				}
			}
		}()
	}

	wg.Wait()
	cancelled := run.ctx.Err() != nil
	run.running.Store(false)
	run.cancel()
	findings := run.snapshotFindings()
	summary := map[string]any{
		"scanId":            run.id,
		"mode":              opts.mode,
		"roots":             opts.roots,
		"cancelled":         cancelled,
		"scannedFiles":      run.processed.Load(),
		"scannedBytes":      run.bytesHashed.Load(),
		"threatsTotal":      run.threats.Load(),
		"threats":           findings.list,
		"findingsTruncated": findings.truncated,
		"skippedLarge":      run.skippedLarge.Load(),
		"skippedErrors":     run.skippedErrors.Load(),
		"durationMs":        time.Since(start).Milliseconds(),
	}
	log.Printf("[av] scan %s finished: cancelled=%v scanned=%d bytes=%d threats=%d errors=%d duration=%s",
		run.id, cancelled, run.processed.Load(), run.bytesHashed.Load(), run.threats.Load(), run.skippedErrors.Load(), time.Since(start).Round(time.Millisecond))
	return summary, true, "", ""
}

// inspect hashes one file, matches the signature DB, applies heuristics,
// and performs the configured on-detect action.
func (e *Engine) inspect(run *scanRun, path string, onDetect string) {
	sum, size, err := hashFile(path)
	if err != nil {
		run.skippedErrors.Add(1)
		return
	}
	run.bytesHashed.Add(size)
	run.processed.Add(1)
	db := e.db.Load()

	var rule, severity, category string
	if hr, ok := db.LookupHash(sum); ok {
		rule, severity, category = hr.Name, hr.Severity, hr.Category
	} else if size <= patternScanLimit {
		if content, err := os.ReadFile(path); err == nil {
			if pr, ok := db.LookupPattern(content); ok {
				rule, severity, category = pr.Name, pr.Severity, pr.Category
			}
		}
	}
	if rule == "" && e.opts.EnableHeuristics {
		if doubleExtension(filepath.Base(path)) {
			rule, severity, category = "Heuristic.DoubleExtension", "medium", "heuristic"
		}
	}
	if rule == "" {
		return
	}
	run.threats.Add(1)
	fnd := Finding{
		Path:     path,
		Rule:     rule,
		Severity: severity,
		Category: category,
		Sha256:   sum,
		Action:   "detected",
	}
	if onDetect == "quarantine" {
		if item, err := e.quarantine.Quarantine(path, sum, rule, severity, category); err == nil {
			fnd.Action = "quarantined"
			fnd.QuarantineID = item.ID
		} else {
			fnd.Action = "quarantine_failed"
			log.Printf("[av] quarantine failed for %s: %v", path, err)
		}
	}
	run.mu.Lock()
	if len(run.findings) < maxFindingsInList {
		run.findings = append(run.findings, fnd)
	} else {
		run.findingsTruncated = true
	}
	run.mu.Unlock()
}

// hashFile streams one file through SHA-256 and closes the handle before
// returning, so a later quarantine can move the file (Windows sharing
// rules forbid renaming open files).
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	closeErr := f.Close()
	if err != nil {
		return "", 0, err
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// snapshotFindings returns a copy of the findings list (capped).
type findingList struct {
	list      []Finding
	truncated bool
}

func (r *scanRun) snapshotFindings() findingList {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := make([]Finding, len(r.findings))
	copy(list, r.findings)
	return findingList{list: list, truncated: r.findingsTruncated}
}

// maybeProgress throttles progress events: at most one per 3% or 5s.
func (e *Engine) maybeProgress(run *scanRun, prog func(map[string]any)) {
	if prog == nil {
		return
	}
	total := run.totalSeen.Load()
	if total == 0 {
		return
	}
	percent := int(run.processed.Load() * 100 / total)
	run.pmu.Lock()
	due := percent-run.lastPercent >= 3 || time.Since(run.lastEmit) >= 5*time.Second
	if due {
		run.lastPercent = percent
		run.lastEmit = time.Now()
	}
	run.pmu.Unlock()
	if due && prog != nil {
		prog(map[string]any{
			"type":      "av_scan_progress",
			"scanId":    run.id,
			"percent":   percent,
			"processed": run.processed.Load(),
			"totalSeen": total,
			"threats":   run.threats.Load(),
		})
	}
}

// doubleExtension flags obviously disguised executables, e.g. invoice.pdf.exe.
var doubleExtRe = regexp.MustCompile(`(?i)\.(pdf|doc|docx|xls|xlsx|ppt|pptx|jpg|jpeg|png|gif|txt)\.(exe|scr|com|pif|bat|cmd|js|vbs|wsf|hta)$`)

func doubleExtension(base string) bool {
	return doubleExtRe.MatchString(base)
}

const maxFindingsInList = 100

// excluded reports whether path falls under a configured exclusion prefix.
func (e *Engine) excluded(path string) bool {
	for _, x := range e.opts.Exclusions {
		if exclusionMatch(path, x) {
			return true
		}
	}
	return false
}

// exclusionMatch does a case-insensitive prefix match with a separator
// boundary: C:\Windows must not match C:\WindowsXYZ.
func exclusionMatch(path, prefix string) bool {
	if prefix == "" {
		return false
	}
	p, x := strings.ToLower(path), strings.ToLower(prefix)
	if p == x {
		return true
	}
	if !strings.HasPrefix(p, x) {
		return false
	}
	next := p[len(x)]
	return next == '\\' || next == '/'
}

// newScanID: scan- prefix + UTC timestamp + 4 random bytes (hex).
func newScanID() string {
	return "scan-" + newQuarantineID()
}
