// SPDX-License-Identifier: AGPL-3.0-only
package popup

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Options configures the popup ability; zero values apply defaults.
type Options struct {
	RulesPath  string // rule set JSON file
	IntervalMS int    // watcher tick; 0 = 1000, clamped [200, 60000]
	Action     string // default action close | hide
	MaxEvents  int    // event buffer cap; 0 = 500
	Enabled    bool   // watcher starts running
}

// Engine is the popup/ad blocking ability.
type Engine struct {
	opts   Options
	rules  atomic.Pointer[RuleSet]
	src    WindowSource
	cooled map[uintptr]time.Time

	mu      sync.Mutex
	events  []Event
	stop    chan struct{}
	started bool

	enabled      atomic.Bool
	blockedTotal atomic.Int64
}

// Event is one interception record (buffered for cloud aggregation).
type Event struct {
	Time    string `json:"time"`
	PID     uint32 `json:"pid"`
	Process string `json:"process"`
	Title   string `json:"title"`
	Class   string `json:"class,omitempty"`
	Rule    string `json:"rule"`
	Action  string `json:"action"` // close | hide
}

// New creates the engine with the real Windows window source.
func New(opts Options) *Engine {
	return NewWithSource(opts, &windowsSource{})
}

// NewWithSource creates the engine with an injected window source (tests).
func NewWithSource(opts Options, src WindowSource) *Engine {
	if opts.RulesPath == "" {
		opts.RulesPath = filepath.Join(dataDir(), "rules.json")
	}
	if opts.IntervalMS <= 0 {
		opts.IntervalMS = 1000
	}
	if opts.IntervalMS < 200 {
		opts.IntervalMS = 200
	}
	if opts.IntervalMS > 60000 {
		opts.IntervalMS = 60000
	}
	if opts.Action != "close" && opts.Action != "hide" {
		opts.Action = "close"
	}
	if opts.MaxEvents <= 0 {
		opts.MaxEvents = 500
	}
	e := &Engine{
		opts:    opts,
		src:     src,
		cooled:  map[uintptr]time.Time{},
		stop:    make(chan struct{}),
		started: false,
	}
	if rs, err := LoadRuleSet(opts.RulesPath); err != nil {
		log.Printf("[popup] rule set not loaded (%v); running with empty rules", err)
		e.rules.Store(EmptyRuleSet())
	} else {
		e.rules.Store(rs)
		log.Printf("[popup] rule set loaded: version=%d block=%d whitelist=%d",
			rs.Version, len(rs.Block), len(rs.Whitelist))
	}
	e.enabled.Store(opts.Enabled)
	return e
}

func dataDir() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "oneclassmesh", "popup")
}

// Name implements ability.Ability.
func (e *Engine) Name() string { return "popup" }

// Types implements ability.Ability; documented in the protocol spec §6.3.
func (e *Engine) Types() []string {
	return []string{
		"POPUP_STATUS", "POPUP_SET_ENABLED", "POPUP_RULES_UPDATE",
		"POPUP_GET_EVENTS", "POPUP_TEST_MATCH",
	}
}

// Start launches the watcher loop (idempotent).
func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started || e.stop == nil {
		return
	}
	if e.enabled.Load() {
		e.started = true
		go e.loop()
	}
}

// loop runs the watcher until stopped.
func (e *Engine) loop() {
	ticker := time.NewTicker(time.Duration(e.opts.IntervalMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-ticker.C:
			if e.enabled.Load() {
				e.tick()
			}
		}
	}
}

// tick performs one enumeration + match + act pass. Exported as a method
// so tests drive it deterministically.
func (e *Engine) tick() {
	wins, err := e.src.List()
	if err != nil {
		return
	}
	rs := e.rules.Load()
	for _, w := range wins {
		if skipWindow(w) {
			continue
		}
		if rs.MatchWhitelist(w) {
			continue
		}
		rule, hit := rs.MatchBlock(w)
		if !hit {
			continue
		}
		if t, seen := e.cooled[w.HWND]; seen && time.Since(t) < 10*time.Second {
			continue
		}
		e.cooled[w.HWND] = time.Now()
		action := rule.Action
		if action == "" {
			action = e.opts.Action
		}
		var actErr error
		if action == "hide" {
			actErr = e.src.Hide(w.HWND)
		} else {
			actErr = e.src.Close(w.HWND)
		}
		if actErr != nil {
			log.Printf("[popup] action failed hwnd=%#x: %v", w.HWND, actErr)
			continue
		}
		e.blockedTotal.Add(1)
		e.record(Event{
			Time:    time.Now().Format(time.RFC3339),
			PID:     w.PID,
			Process: w.Process,
			Title:   w.Title,
			Class:   w.Class,
			Rule:    ruleDisplay(rule),
			Action:  action,
		})
	}
	e.pruneCooled()
}

// skipWindow: windows we never touch, regardless of rules — unidentified
// owners, system critical processes, and our own process family.
func skipWindow(w WindowInfo) bool {
	if w.Process == "" {
		return true
	}
	p := strings.ToLower(w.Process)
	if p == "explorer.exe" {
		return true
	}
	for _, s := range systemProcesses {
		if p == s {
			return true
		}
	}
	for _, s := range selfProcesses {
		if p == s {
			return true
		}
	}
	return false
}

// systemProcesses: hard-skip list; blocking these can break the OS shell.
var systemProcesses = []string{
	"csrss.exe", "winlogon.exe", "wininit.exe", "services.exe",
	"lsass.exe", "smss.exe", "dwm.exe", "fontdrvhost.exe", "svchost.exe",
	"sihost.exe", "taskhostw.exe", "ctfmon.exe", "searchhost.exe",
	"shellexperiencehost.exe", "startmenuexperiencehost.exe",
}

// selfProcesses: OneClassMesh agent family.
var selfProcesses = []string{
	"launcher.exe", "core.exe", "ability.exe", "agent.exe", "ocmctl.exe",
	"ClassAegisLauncher.exe", "ClassAegisCore.exe", "ClassAegisAbility.exe", "ClassAegisDebug.exe", "ClassAegisCtl.exe", "ClassAegisService.exe",
	"课佑管家.exe", "守界管家.exe", "壹课管家.exe",
}

func ruleDisplay(r Rule) string {
	if r.Note != "" {
		return r.Note
	}
	parts := []string{}
	if r.Process != "" {
		parts = append(parts, "process="+r.Process)
	}
	if r.Title != "" {
		parts = append(parts, "title="+r.Title)
	}
	if r.Class != "" {
		parts = append(parts, "class="+r.Class)
	}
	return strings.Join(parts, ",")
}

// record appends an event, dropping the oldest beyond MaxEvents.
func (e *Engine) record(ev Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, ev)
	if len(e.events) > e.opts.MaxEvents {
		e.events = e.events[len(e.events)-e.opts.MaxEvents:]
	}
}

// pruneCooled drops stale cooldown entries (periodic map GC).
func (e *Engine) pruneCooled() {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	for h, t := range e.cooled {
		if now.Sub(t) >= 10*time.Second {
			delete(e.cooled, h)
		}
	}
}

// Handle routes one popup command; no long tasks, prog is unused.
func (e *Engine) Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	switch msgType {
	case "POPUP_STATUS":
		return e.handleStatus()
	case "POPUP_SET_ENABLED":
		return e.handleSetEnabled(payload)
	case "POPUP_RULES_UPDATE":
		return e.handleRulesUpdate(payload)
	case "POPUP_GET_EVENTS":
		return e.handleGetEvents(payload)
	case "POPUP_TEST_MATCH":
		return e.handleTestMatch(payload)
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType, nil
	}
}

func (e *Engine) handleStatus() (bool, string, string, map[string]any) {
	e.mu.Lock()
	buffered := len(e.events)
	started := e.started
	e.mu.Unlock()
	rs := e.rules.Load()
	return true, "", "", map[string]any{
		"enabled":       e.enabled.Load(),
		"watching":      started && e.enabled.Load(),
		"intervalMs":    e.opts.IntervalMS,
		"action":        e.opts.Action,
		"kernelMode":    false,
		"captureUpload": map[string]any{"supported": false, "reason": "window capture and cloud recognition require a signed capture backend and Core upload contract"},
		"rules":         rulesInfo(rs),
		"stats":         map[string]any{"blockedTotal": e.blockedTotal.Load(), "eventsBuffered": buffered},
	}
}

func rulesInfo(rs *RuleSet) map[string]any {
	if rs == nil {
		return map[string]any{"loaded": false}
	}
	return map[string]any{
		"loaded":    true,
		"version":   rs.Version,
		"updated":   rs.Updated,
		"source":    rs.Source,
		"block":     len(rs.Block),
		"whitelist": len(rs.Whitelist),
	}
}

// handleSetEnabled toggles the watcher, starting the loop on first enable.
func (e *Engine) handleSetEnabled(payload map[string]any) (bool, string, string, map[string]any) {
	enabled := false
	if v, ok := payload["enabled"].(bool); ok {
		enabled = v
	} else if v := payload["enabled"]; v != nil {
		return false, "E_PARAM", "enabled must be a boolean", nil
	}
	e.enabled.Store(enabled)
	e.Start()
	e.mu.Lock()
	started := e.started
	e.mu.Unlock()
	log.Printf("[popup] enabled=%v", enabled)
	return true, "", "", map[string]any{"enabled": enabled, "watching": started && enabled}
}

// handleRulesUpdate replaces the rule set from a local file or an inline
// payload; the in-memory set is only replaced after validation.
func (e *Engine) handleRulesUpdate(payload map[string]any) (bool, string, string, map[string]any) {
	var rs *RuleSet
	var err error
	if path := strField(payload, "path"); path != "" {
		rs, err = LoadRuleSet(path)
	} else if inline, ok := payload["rules"]; ok && inline != nil {
		var raw []byte
		raw, err = json.Marshal(inline)
		if err == nil {
			rs, err = ParseRuleSet(raw)
		}
	} else {
		return false, "E_PARAM", "path or rules is required", nil
	}
	if err != nil {
		log.Printf("[popup] rules update rejected: %v", err)
		return false, "E_RULES_FORMAT", "invalid rule set: " + err.Error(), nil
	}
	e.rules.Store(rs)
	if serr := rs.Save(e.opts.RulesPath); serr != nil {
		log.Printf("[popup] rules updated in memory but failed to save: %v", serr)
	}
	log.Printf("[popup] rule set updated: version=%d block=%d whitelist=%d",
		rs.Version, len(rs.Block), len(rs.Whitelist))
	return true, "", "", rulesInfo(rs)
}

// handleGetEvents returns buffered events, oldest first (capped 100).
func (e *Engine) handleGetEvents(payload map[string]any) (bool, string, string, map[string]any) {
	clear := false
	if v, ok := payload["clear"].(bool); ok {
		clear = v
	}
	const chunk = 100
	e.mu.Lock()
	defer e.mu.Unlock()
	n := chunk
	if len(e.events) < n {
		n = len(e.events)
	}
	batch := make([]Event, n)
	copy(batch, e.events[:n])
	remaining := len(e.events) - n
	if clear {
		e.events = e.events[n:]
	}
	return true, "", "", map[string]any{
		"events":    batch,
		"remaining": remaining,
		"hasMore":   remaining > 0,
	}
}

// handleTestMatch dry-runs the current rules against a synthetic window.
func (e *Engine) handleTestMatch(payload map[string]any) (bool, string, string, map[string]any) {
	w := WindowInfo{
		Process: strField(payload, "process"),
		Title:   strField(payload, "title"),
		Class:   strField(payload, "class"),
	}
	if w.Process == "" && w.Title == "" && w.Class == "" {
		return false, "E_PARAM", "process, title or class is required", nil
	}
	rs := e.rules.Load()
	if rs.MatchWhitelist(w) {
		return true, "", "", map[string]any{"matched": false, "reason": "whitelisted"}
	}
	if rule, hit := rs.MatchBlock(w); hit {
		return true, "", "", map[string]any{"matched": true, "rule": ruleDisplay(rule), "action": ruleAction(rule, e.opts.Action)}
	}
	return true, "", "", map[string]any{"matched": false, "reason": "no rule"}
}

// ruleAction returns the rule action or the engine default.
func ruleAction(r Rule, def string) string {
	if r.Action == "close" || r.Action == "hide" {
		return r.Action
	}
	return def
}

// strField reads a string field (payload values may be string or number).
func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
