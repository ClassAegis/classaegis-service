package clean

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Options configures the clean ability; zero values apply defaults.
type Options struct {
	MinAgeHours       int      // temp-category age guard; 0 = default 24h
	Exclusions        []string // path prefixes never touched
	EnableCategories  []string // opt-in categories (e.g. windows_old)
	DisableCategories []string // opt-outs from the default set
}

// Engine is the junk cleanup ability.
type Engine struct {
	opts    Options
	cats    []Category
	recycle recycleAPI

	mu       sync.Mutex // guards run
	run      *cleanRun
	lastScan map[string]any
	lastRun  map[string]any
}

// cleanRun tracks one running scan or clean pass.
type cleanRun struct {
	id      string
	mode    string // scan | run
	ctx     context.Context
	cancel  context.CancelFunc
	started time.Time
	running atomic.Bool

	totalSeen     atomic.Int64
	processed     atomic.Int64
	bytesFound    atomic.Int64
	deletedFiles  atomic.Int64
	deletedBytes  atomic.Int64
	failedFiles   atomic.Int64
	skippedErrors atomic.Int64

	// guarded by run.mu (single-threaded walk, but result building reads)
	mu       sync.Mutex
	items    []junkItem
	catStats map[string]*catStat

	// progress throttle state
	pmu         sync.Mutex
	lastPercent int
	lastEmit    time.Time
}

// junkItem is one junk file (display list only).
type junkItem struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Size     int64  `json:"size"`
}

// catStat accumulates one category's counters.
type catStat struct {
	Files        int64 `json:"files"`
	Bytes        int64 `json:"bytes"`
	DeletedFiles int64 `json:"deletedFiles"`
	DeletedBytes int64 `json:"deletedBytes"`
	FailedFiles  int64 `json:"failedFiles"`
}

// recycleAPI abstracts the recycle-bin shell API (test seam).
type recycleAPI interface {
	// Query reports aggregate item count and byte size for one drive root.
	Query(drive string) (files int64, bytes int64, err error)
	// Empty empties the recycle bin of one drive root.
	Empty(drive string) error
}

// New creates the engine with the real category set and shell API.
func New(opts Options) *Engine {
	minAge := 24 * time.Hour
	if opts.MinAgeHours < 0 {
		minAge = 0 // explicit opt-out
	} else if opts.MinAgeHours > 0 {
		minAge = time.Duration(opts.MinAgeHours) * time.Hour
	}
	defs := defaultCategories(minAge)
	cats, msg := resolveCategories(defs, opts.EnableCategories, opts.DisableCategories, nil)
	if msg != "" {
		log.Printf("[clean] config category error: %s; using defaults", msg)
		cats, _ = resolveCategories(defs, nil, nil, nil)
	}
	return &Engine{
		opts:    opts,
		cats:    cats,
		recycle: &shellRecycler{},
	}
}

// resolveDefaults is the fallback category set (no config overrides).
func resolveDefaults(minAge time.Duration) []Category {
	cats, _ := resolveCategories(defaultCategories(minAge), nil, nil, nil)
	return cats
}

// Name implements ability.Ability.
func (e *Engine) Name() string { return "clean" }

// Types implements ability.Ability; documented in the protocol spec §6.4.
func (e *Engine) Types() []string {
	return []string{"CLEAN_SCAN", "CLEAN_RUN", "CLEAN_STOP", "CLEAN_STATUS"}
}

// Handle routes one clean command.
func (e *Engine) Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	switch msgType {
	case "CLEAN_SCAN":
		return e.handleScan(msgId, payload, prog)
	case "CLEAN_RUN":
		return e.handleScanRun(msgId, payload, prog)
	case "CLEAN_STOP":
		return e.handleStop()
	case "CLEAN_STATUS":
		return e.handleStatus()
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType, nil
	}
}

// parseCategories validates the payload category subset against e.cats.
func (e *Engine) parseCategories(payload map[string]any) ([]Category, bool, string) {
	raw, ok := payload["categories"].([]any)
	if !ok || len(raw) == 0 {
		return e.cats, false, ""
	}
	var ids []string
	for _, v := range raw {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, false, "categories must be strings"
		}
		ids = append(ids, s)
	}
	byID := map[string]Category{}
	for _, c := range e.cats {
		byID[c.ID] = c
	}
	var out []Category
	for _, id := range ids {
		c, ok := byID[id]
		if !ok {
			return nil, false, "unknown or not-enabled category: " + id
		}
		out = append(out, c)
	}
	return out, true, ""
}

// handleScan / handleScanRun share one executor: mode scan collects and
// reports; mode run deletes as it goes. Both run synchronously (core's
// loopback window for CLEAN_* is 1h) with PROGRESS via prog.
func (e *Engine) handleScan(msgId string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	return e.execute(msgId, "scan", payload, prog)
}

func (e *Engine) handleScanRun(msgId string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	return e.execute(msgId, "run", payload, prog)
}

func (e *Engine) handleStop() (bool, string, string, map[string]any) {
	e.mu.Lock()
	run := e.run
	e.mu.Unlock()
	if run == nil || !run.running.Load() {
		return false, "E_NO_SCAN", "no scan or clean is running", nil
	}
	run.cancel()
	log.Printf("[clean] stop requested for %s", run.id)
	return true, "", "", map[string]any{"stopRequested": true}
}

func (e *Engine) handleStatus() (bool, string, string, map[string]any) {
	e.mu.Lock()
	run := e.run
	running := run != nil && run.running.Load()
	lastScan := e.lastScan
	lastRun := e.lastRun
	e.mu.Unlock()
	return true, "", "", map[string]any{
		"running":  running,
		"lastScan": lastScan,
		"lastRun":  lastRun,
	}
}

// execute validates the payload, registers the run and performs the pass
// synchronously; the terminal receipt carries the summary.
func (e *Engine) execute(msgId, mode string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	cats, _, perr := e.parseCategories(payload)
	if perr != "" {
		return false, "E_PARAM", perr, nil
	}
	e.mu.Lock()
	if e.run != nil && e.run.running.Load() {
		e.mu.Unlock()
		return false, "E_SCAN_RUNNING", "a scan or clean is already running", nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &cleanRun{
		id:       newRunID(),
		mode:     mode,
		ctx:      ctx,
		cancel:   cancel,
		started:  time.Now(),
		catStats: map[string]*catStat{},
	}
	run.running.Store(true)
	e.run = run
	e.mu.Unlock()

	result, ok, code, message := e.runPass(run, cats, mode, prog)
	e.mu.Lock()
	if e.run == run {
		e.run = nil
	}
	if mode == "scan" {
		e.lastScan = result
	} else {
		e.lastRun = result
	}
	e.mu.Unlock()
	return ok, code, message, result
}

// runPass walks every category (recycle via shell API, others via WalkDir).
func (e *Engine) runPass(run *cleanRun, cats []Category, mode string, prog func(map[string]any)) (map[string]any, bool, string, string) {
	start := time.Now()
	idsList := make([]string, 0, len(cats))
	for _, c := range cats {
		idsList = append(idsList, c.ID)
	}
	log.Printf("[clean] %s %s started: categories=%v", mode, run.id, idsList)

	for _, cat := range cats {
		if run.ctx.Err() != nil {
			break
		}
		if cat.Recycle {
			e.handleRecycle(run, cat, mode, prog)
		} else {
			e.walkCategory(run, cat, mode, prog)
		}
	}
	cancelled := run.ctx.Err() != nil
	run.running.Store(false)

	summary := e.buildSummary(run, cats, mode, cancelled, start)
	log.Printf("[clean] %s %s finished: cancelled=%v files=%d bytes=%d deleted=%d(%d files) failed=%d errors=%d duration=%s",
		mode, run.id, cancelled, run.processed.Load(), run.bytesFound.Load(),
		run.deletedFiles.Load(), run.deletedBytes.Load(), run.failedFiles.Load(),
		run.skippedErrors.Load(), time.Since(start).Round(time.Millisecond))
	run.cancel()
	return summary, true, "", ""
}

// walkCategory walks one category's roots, accounting sizes and (in run
// mode) deleting junk files in place.
func (e *Engine) walkCategory(run *cleanRun, cat Category, mode string, prog func(map[string]any)) {
	seenRoots := map[string]bool{}
	for _, tpl := range cat.Roots {
		if run.ctx.Err() != nil {
			return
		}
		root := expandEnv(tpl)
		if root == "" {
			continue
		}
		key := strings.ToLower(root)
		if seenRoots[key] {
			continue // %TEMP% and %TMP% are usually the same path
		}
		seenRoots[key] = true
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			continue // root missing or not a directory: normal (e.g. no Chrome)
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if run.ctx.Err() != nil {
				return fs.SkipAll
			}
			if err != nil {
				run.skippedErrors.Add(1)
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
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
			if cat.MinAge > 0 && time.Since(info.ModTime()) < cat.MinAge {
				return nil
			}
			if cat.Match != nil && !cat.Match(d.Name()) {
				return nil
			}
			if e.excluded(path) {
				return nil
			}
			size := info.Size()
			run.totalSeen.Add(1)
			run.bytesFound.Add(size)
			st := run.statFor(cat.ID)
			st.Files++
			st.Bytes += size

			if mode == "run" {
				if err := os.Remove(path); err != nil {
					st.FailedFiles++
					run.failedFiles.Add(1)
				} else {
					st.DeletedFiles++
					st.DeletedBytes += size
					run.deletedFiles.Add(1)
					run.deletedBytes.Add(size)
				}
			}
			run.mu.Lock()
			if len(run.items) < maxDisplayItems {
				run.items = append(run.items, junkItem{Path: path, Category: cat.ID, Size: size})
			}
			run.mu.Unlock()
			run.processed.Add(1)
			e.maybeProgress(run, prog)
			return nil
		})
	}
}

// handleRecycle processes the recycle-bin category via the shell API:
// per fixed drive, query aggregate counts, empty in run mode.
func (e *Engine) handleRecycle(run *cleanRun, cat Category, mode string, prog func(map[string]any)) {
	for _, d := range fixedDrives() {
		if run.ctx.Err() != nil {
			return
		}
		files, bytes, err := e.recycle.Query(d)
		if err != nil {
			run.skippedErrors.Add(1)
			continue
		}
		if files == 0 {
			continue
		}
		st := run.statFor(cat.ID)
		st.Files += files
		st.Bytes += bytes
		run.totalSeen.Add(files)
		run.bytesFound.Add(bytes)
		run.processed.Add(files)
		if mode == "run" {
			if err := e.recycle.Empty(d); err != nil {
				st.FailedFiles += files
				run.failedFiles.Add(files)
				log.Printf("[clean] empty recycle bin failed on %s: %v", d, err)
			} else {
				st.DeletedFiles += files
				st.DeletedBytes += bytes
				run.deletedFiles.Add(files)
				run.deletedBytes.Add(bytes)
			}
		}
		e.maybeProgress(run, prog)
	}
}

// buildSummary assembles the terminal receipt result.
func (e *Engine) buildSummary(run *cleanRun, cats []Category, mode string, cancelled bool, start time.Time) map[string]any {
	catsResult := []map[string]any{}
	for _, c := range cats {
		st := run.catStats[c.ID]
		if st == nil {
			st = &catStat{}
		}
		catsResult = append(catsResult, map[string]any{
			"id":           c.ID,
			"name":         c.Name,
			"files":        st.Files,
			"bytes":        st.Bytes,
			"deletedFiles": st.DeletedFiles,
			"deletedBytes": st.DeletedBytes,
			"failedFiles":  st.FailedFiles,
		})
	}
	summary := map[string]any{
		"scanId":        run.id,
		"mode":          run.mode,
		"cancelled":     cancelled,
		"categories":    catsResult,
		"totalFiles":    run.processed.Load(),
		"totalBytes":    run.bytesFound.Load(),
		"deletedFiles":  run.deletedFiles.Load(),
		"deletedBytes":  run.deletedBytes.Load(),
		"failedFiles":   run.failedFiles.Load(),
		"skippedErrors": run.skippedErrors.Load(),
		"durationMs":    time.Since(start).Milliseconds(),
	}
	if mode == "scan" {
		run.mu.Lock()
		items := make([]junkItem, len(run.items))
		copy(items, run.items)
		run.mu.Unlock()
		sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
		const topN = 100
		truncated := len(items) > topN
		if truncated {
			items = items[:topN]
		}
		summary["findings"] = items
		summary["findingsTruncated"] = truncated
	}
	return summary
}

// maybeProgress throttles progress events: at most one per 3% or 5s.
func (e *Engine) maybeProgress(run *cleanRun, prog func(map[string]any)) {
	if prog == nil {
		return
	}
	percent := int(run.processed.Load() * 100 / max64(run.totalSeen.Load(), 1))
	run.pmu.Lock()
	due := percent-run.lastPercent >= 3 || time.Since(run.lastEmit) >= 5*time.Second
	if due {
		run.lastPercent = percent
		run.lastEmit = time.Now()
	}
	run.pmu.Unlock()
	if due {
		prog(map[string]any{
			"type":         "clean_progress",
			"phase":        run.mode,
			"scanId":       run.id,
			"percent":      percent,
			"files":        run.processed.Load(),
			"bytes":        run.bytesFound.Load(),
			"deletedBytes": run.deletedBytes.Load(),
		})
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// excluded reports whether path falls under a configured exclusion prefix.
func (e *Engine) excluded(path string) bool {
	for _, x := range e.opts.Exclusions {
		if exclusionMatch(path, x) {
			return true
		}
	}
	return false
}

// exclusionMatch: case-insensitive prefix with a separator boundary
// (duplicated from av by design: a little copying over a little dependency).
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

// newRunID: clean- prefix + UTC timestamp + 4 random bytes (hex).
func newRunID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "clean-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b)
}

const maxDisplayItems = 1000

// statFor returns (creating if needed) the per-category accumulator.
func (r *cleanRun) statFor(id string) *catStat {
	st := r.catStats[id]
	if st == nil {
		st = &catStat{}
		r.catStats[id] = st
	}
	return st
}
