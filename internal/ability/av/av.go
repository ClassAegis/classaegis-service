// Package av implements the local antivirus engine ability (本地杀毒引擎):
// hash + pattern signature scanning of local files, double-extension
// heuristics, a quarantine store, and throttled progress reporting. The
// process is loopback-only; signature updates arrive as local files
// (network push goes through core in a later milestone).
package av

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Options configures the av ability; zero values apply defaults.
type Options struct {
	DbPath           string   // signature DB JSON file
	QuarantineDir    string   // quarantine store directory
	Threads          int      // hashing goroutines; 0 = auto
	MaxFileSizeMB    int      // files above are skipped; 0 = 512
	Exclusions       []string // path prefixes to skip
	EnableHeuristics bool     // double-extension heuristic
	DefaultOnDetect  string   // "report" | "quarantine"; empty = quarantine
}

// Engine is the local antivirus engine ability.
type Engine struct {
	opts       Options
	dbPath     string
	db         atomic.Pointer[SignatureDB]
	quarantine *Quarantine

	mu   sync.Mutex // guards scan
	scan *scanRun   // current scan; nil when idle
}

// New creates the engine, applies defaults, loads the signature DB (an
// unreadable DB degrades to an empty one — scanning and heuristics still
// work) and prepares the quarantine store.
func New(opts Options) *Engine {
	if opts.DbPath == "" {
		opts.DbPath = filepath.Join(programDataDir(), "signatures.json")
	}
	if opts.QuarantineDir == "" {
		opts.QuarantineDir = filepath.Join(programDataDir(), "quarantine")
	}
	if opts.MaxFileSizeMB <= 0 {
		opts.MaxFileSizeMB = 512
	}
	if opts.DefaultOnDetect != "report" && opts.DefaultOnDetect != "quarantine" {
		opts.DefaultOnDetect = "quarantine"
	}
	if opts.Threads <= 0 {
		opts.Threads = runtime.NumCPU()
		if opts.Threads > 8 {
			opts.Threads = 8
		}
	}
	// self-protection: never scan our own ProgramData tree (quarantine store,
	// signature DB, logs would recurse / deadlock the store)
	opts.Exclusions = append(opts.Exclusions, programDataDir(), opts.QuarantineDir)
	e := &Engine{dbPath: opts.DbPath, opts: opts, quarantine: NewQuarantine(opts.QuarantineDir)}
	if db, err := LoadSignatureDB(opts.DbPath); err != nil {
		log.Printf("[av] signature db not loaded (%v); running with empty db", err)
		e.db.Store(EmptySignatureDB())
	} else {
		e.db.Store(db)
		log.Printf("[av] signature db loaded: version=%d hashes=%d patterns=%d",
			db.Version, len(db.Hashes), len(db.Patterns))
	}
	return e
}

func programDataDir() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "oneclassmesh", "av")
}

// Name implements ability.Ability.
func (e *Engine) Name() string { return "av" }

// Types implements ability.Ability. The set is documented in
// docs/design-av.md and the protocol spec.
func (e *Engine) Types() []string {
	return []string{
		"AV_SCAN", "AV_STOP_SCAN", "AV_STATUS", "AV_DB_INFO", "AV_DB_UPDATE",
		"AV_QUARANTINE_LIST", "AV_QUARANTINE_RESTORE", "AV_QUARANTINE_DELETE",
	}
}

// Handle routes one AV command. Long operations (scan) run synchronously;
// progress goes through prog (core turns it into PROGRESS receipts).
func (e *Engine) Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	switch msgType {
	case "AV_SCAN":
		return e.handleScan(msgId, payload, prog)
	case "AV_STOP_SCAN":
		return e.handleStopScan()
	case "AV_STATUS":
		return e.handleStatus()
	case "AV_DB_INFO":
		return e.handleDBInfo()
	case "AV_DB_UPDATE":
		return e.handleDBUpdate(payload)
	case "AV_QUARANTINE_LIST":
		return e.handleQuarantineList()
	case "AV_QUARANTINE_RESTORE":
		return e.handleQuarantineRestore(payload)
	case "AV_QUARANTINE_DELETE":
		return e.handleQuarantineDelete(payload)
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType, nil
	}
}

// --- payload helpers (values may be string or json.Number) ---

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func num(m map[string]any, key string) int64 {
	if n, ok := m[key].(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	return 0
}

// --- scan run state ---

// scanRun tracks one running scan; counters are atomic, findings are
// guarded by mu.
type scanRun struct {
	id            string
	ctx           context.Context
	cancel        context.CancelFunc
	started       time.Time
	running       atomic.Bool
	totalSeen     atomic.Int64 // files discovered so far
	processed     atomic.Int64
	bytesHashed   atomic.Int64
	threats       atomic.Int64
	skippedLarge  atomic.Int64
	skippedErrors atomic.Int64

	mu                sync.Mutex
	findings          []Finding
	findingsTruncated bool

	// progress throttle state
	pmu         sync.Mutex
	lastPercent int
	lastEmit    time.Time
}

// scanOptions is the parsed AV_SCAN payload.
type scanOptions struct {
	mode     string // quick | full | custom
	roots    []string
	onDetect string // report | quarantine
}

// Finding is one detection. JSON tags match the receipt result schema.
type Finding struct {
	Path         string `json:"path"`
	Rule         string `json:"rule"`
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Action       string `json:"action"` // detected | quarantined | quarantine_failed
	QuarantineID string `json:"quarantineId,omitempty"`
	Sha256       string `json:"sha256"`
}

// parseScanPayload validates the AV_SCAN payload.
func parseScanPayload(payload map[string]any, defOnDetect string) (scanOptions, string) {
	mode := str(payload, "mode")
	if mode == "" {
		mode = "quick"
	}
	onDetect := str(payload, "on_detect")
	if onDetect == "" {
		onDetect = defOnDetect
	}
	if onDetect != "report" && onDetect != "quarantine" {
		return scanOptions{}, "on_detect must be report|quarantine"
	}
	opts := scanOptions{mode: mode, onDetect: onDetect}
	switch mode {
	case "quick":
		opts.roots = quickScanRoots()
	case "full":
		opts.roots = fullScanRoots()
	case "custom":
		raw, ok := payload["paths"].([]any)
		if !ok || len(raw) == 0 {
			return scanOptions{}, "custom mode requires paths (non-empty array)"
		}
		for _, v := range raw {
			p, ok := v.(string)
			if !ok || p == "" {
				return scanOptions{}, "paths must be strings"
			}
			opts.roots = append(opts.roots, p)
		}
	default:
		return scanOptions{}, "mode must be quick|full|custom"
	}
	return opts, ""
}

// quickScanRoots: user temp, downloads, desktop (常见感染面).
func quickScanRoots() []string {
	var roots []string
	for _, env := range []string{"TEMP", "TMP", "USERPROFILE"} {
		if v := os.Getenv(env); v != "" {
			switch env {
			case "TEMP", "TMP":
				roots = append(roots, v)
			case "USERPROFILE":
				for _, sub := range []string{"Downloads", "Desktop"} {
					roots = append(roots, filepath.Join(v, sub))
				}
			}
		}
	}
	return roots
}

// fullScanRoots: all fixed drives (exclusions still apply).
func fullScanRoots() []string {
	var roots []string
	for _, d := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		root := string(d) + `:\`
		if _, err := os.Stat(root); err == nil {
			roots = append(roots, root)
		}
	}
	return roots
}
