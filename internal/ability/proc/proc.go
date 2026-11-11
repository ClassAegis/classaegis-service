// Package proc implements the process acceleration and per-process traffic
// ability (进程加速/逐进程流量统计/限速): working-set trimming, priority
// adjustment, TCP-estats-based traffic accounting, and experimental
// policy-based QoS throttling (elevation required).
package proc

import (
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Options configures the proc ability; zero values apply defaults.
type Options struct {
	PollIntervalMS int // traffic poller tick; 0 = 5000, clamped [1000, 60000]
}

// Engine is the process management ability.
type Engine struct {
	opts    Options
	ops     sysOps
	mu      sync.Mutex
	accum   map[uint32]*trafficAcc // per-pid traffic accumulator
	usage   map[uint32]*usageAcc
	tick    time.Time
	stop    chan struct{}
	started bool
}

// trafficAcc accumulates one pid's traffic (bytes in/out + rates).
type trafficAcc struct {
	TotalIn               int64
	TotalOut              int64
	prevIn                uint64
	prevOut               uint64
	prevLanIn, prevLanOut uint64
	prevWanIn, prevWanOut uint64
	seen                  bool
	RateIn                int64 // bytes/s over the last interval
	RateOut               int64
	lastSeen              time.Time
	LanIn, LanOut         int64
	WanIn, WanOut         int64
}

type usageAcc struct {
	Process           string
	ForegroundSeconds int64
	LastSample        time.Time
	LanIn, LanOut     int64
	WanIn, WanOut     int64
}

// New creates the engine with the platform ops.
func New(opts Options) *Engine {
	if opts.PollIntervalMS <= 0 {
		opts.PollIntervalMS = 5000
	}
	if opts.PollIntervalMS < 1000 {
		opts.PollIntervalMS = 1000
	}
	if opts.PollIntervalMS > 60000 {
		opts.PollIntervalMS = 60000
	}
	return &Engine{
		opts:  opts,
		ops:   newSysOps(),
		accum: map[uint32]*trafficAcc{},
		usage: map[uint32]*usageAcc{},
		stop:  make(chan struct{}),
	}
}

// Start launches the traffic poller loop (idempotent).
func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return
	}
	e.started = true
	go e.loop()
}

func (e *Engine) loop() {
	ticker := time.NewTicker(time.Duration(e.opts.PollIntervalMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-ticker.C:
			e.pollOnce()
		}
	}
}

// pollOnce takes one connection sample and folds deltas into the
// accumulator. Called by the loop; also driven directly by tests.
func (e *Engine) pollOnce() {
	samples, err := e.ops.connSamples()
	if err != nil {
		log.Printf("[proc] traffic sample failed: %v", err)
		return
	}
	now := time.Now()
	e.fold(samples, now, 200*time.Millisecond)
	e.sampleForeground(now)
}

func (e *Engine) sampleForeground(now time.Time) {
	pid, err := e.ops.foregroundPID()
	if err != nil || pid == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	u := e.usage[pid]
	if u == nil {
		u = &usageAcc{}
		e.usage[pid] = u
	}
	if !u.LastSample.IsZero() {
		d := now.Sub(u.LastSample)
		if d > 0 && d < 2*time.Duration(e.opts.PollIntervalMS)*time.Millisecond {
			u.ForegroundSeconds += int64(d / time.Second)
		}
	}
	u.LastSample = now
	if ps, err := e.ops.listProcesses(); err == nil {
		for _, p := range ps {
			if p.PID == pid {
				u.Process = p.Name
				break
			}
		}
	}
}

// fold folds one sample batch into the accumulator; minElapsed guards
// against back-to-back ticks (tests pass 0 for determinism).
func (e *Engine) fold(samples []connSample, now time.Time, minElapsed time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tick.IsZero() {
		e.tick = now
	}
	elapsed := now.Sub(e.tick).Seconds()
	if elapsed < minElapsed.Seconds() {
		return
	}
	cur := map[uint32][4]uint64{}
	for _, s := range samples {
		v := cur[s.PID]
		v[0] += s.In
		v[1] += s.Out
		if s.Class == "lan" {
			v[2] += s.In
			v[3] += s.Out
		}
		cur[s.PID] = v
	}
	for pid, v := range cur {
		acc := e.accum[pid]
		if acc == nil {
			acc = &trafficAcc{}
			e.accum[pid] = acc
		}
		if acc.seen {
			din := int64(v[0]) - int64(acc.prevIn)
			dout := int64(v[1]) - int64(acc.prevOut)
			// pid reuse or counter reset: clamp at zero
			if din < 0 {
				din = 0
			}
			if dout < 0 {
				dout = 0
			}
			acc.TotalIn += din
			acc.TotalOut += dout
			dli := clampDelta(v[2], acc.prevLanIn)
			dlo := clampDelta(v[3], acc.prevLanOut)
			dwi := clampDelta(v[0]-v[2], acc.prevWanIn)
			dwo := clampDelta(v[1]-v[3], acc.prevWanOut)
			acc.LanIn += dli
			acc.LanOut += dlo
			acc.WanIn += dwi
			acc.WanOut += dwo
			u := e.usage[pid]
			if u == nil { u = &usageAcc{}; e.usage[pid] = u }
			u.LanIn += dli; u.LanOut += dlo; u.WanIn += dwi; u.WanOut += dwo
			acc.RateIn = int64(float64(din) / elapsed)
			acc.RateOut = int64(float64(dout) / elapsed)
		}
		acc.prevIn, acc.prevOut = v[0], v[1]
		acc.prevLanIn, acc.prevLanOut = v[2], v[3]
		acc.prevWanIn, acc.prevWanOut = v[0]-v[2], v[1]-v[3]
		acc.seen = true
		acc.lastSeen = now
	}
	// fade out pids unseen for 5 intervals
	cutoff := now.Add(-5 * time.Duration(e.opts.PollIntervalMS) * time.Millisecond)
	for pid, acc := range e.accum {
		if acc.lastSeen.Before(cutoff) {
			delete(e.accum, pid)
		}
	}
	e.tick = now
}

func clampDelta(cur, prev uint64) int64 {
	if cur < prev {
		return 0
	}
	return int64(cur - prev)
}

// Name implements ability.Ability.
func (e *Engine) Name() string { return "proc" }

// Types implements ability.Ability; documented in the protocol spec §6.5.
func (e *Engine) Types() []string {
	return []string{"PROC_SPEEDUP", "PROC_LIST", "PROC_PRIORITY", "PROC_THROTTLE", "PROC_USAGE_STATUS", "PROC_USAGE_REPORT"}
}

// Handle routes one proc command.
func (e *Engine) Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	switch msgType {
	case "PROC_SPEEDUP":
		return e.handleSpeedup(payload)
	case "PROC_LIST":
		return e.handleList(payload)
	case "PROC_PRIORITY":
		return e.handlePriority(payload)
	case "PROC_THROTTLE":
		return e.handleThrottle(payload)
	case "PROC_USAGE_STATUS", "PROC_USAGE_REPORT":
		return e.handleUsage(payload, prog)
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType, nil
	}
}

func (e *Engine) handleUsage(payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	duration := procNum(payload, "durationSeconds")
	if duration < 0 || duration > 300 {
		return false, "E_PARAM", "durationSeconds must be 0..300", nil
	}
	if duration > 0 {
		deadline := time.Now().Add(time.Duration(duration) * time.Second)
		for time.Now().Before(deadline) {
			e.pollOnce()
			if prog != nil {
				prog(e.usageSnapshot())
			}
			time.Sleep(time.Duration(e.opts.PollIntervalMS) * time.Millisecond)
		}
	}
	return true, "", "", e.usageSnapshot()
}

func (e *Engine) usageSnapshot() map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	rows := make([]map[string]any, 0, len(e.usage))
	for pid, u := range e.usage {
		row := map[string]any{"pid": pid, "process": u.Process, "foregroundSeconds": u.ForegroundSeconds}
		row["lanIn"] = u.LanIn
		row["lanOut"] = u.LanOut
		row["wanIn"] = u.WanIn
		row["wanOut"] = u.WanOut
		rows = append(rows, row)
	}
	return map[string]any{"processes": rows, "pollIntervalMs": e.opts.PollIntervalMS, "classification": "RFC1918/RFC4193=lan; other destinations=wan; TCP only"}
}

// handleSpeedup trims working sets (all user processes or a payload-filtered
// subset) and applies optional priority adjustments.
func (e *Engine) handleSpeedup(payload map[string]any) (bool, string, string, map[string]any) {
	var filter map[string]bool
	if raw, ok := payload["processes"].([]any); ok {
		filter = map[string]bool{}
		for _, v := range raw {
			if s, ok := v.(string); ok && s != "" {
				filter[strings.ToLower(s)] = true
			}
		}
	}
	procs, err := e.ops.listProcesses()
	if err != nil {
		return false, "E_FAILED", "list processes failed: " + err.Error(), nil
	}
	var trimmed, failed int64
	var freed int64
	for _, p := range procs {
		if skipProc(p.Name) {
			continue
		}
		if filter != nil && !filter[strings.ToLower(p.Name)] {
			continue
		}
		f, err := e.ops.trimWorkingSet(p.PID)
		if err != nil {
			failed++
			continue
		}
		trimmed++
		freed += f
	}
	// optional priority adjustments
	var prioApplied, prioFailed int64
	if raw, ok := payload["priorities"].([]any); ok {
		for _, v := range raw {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			priName, _ := m["priority"].(string)
			pri, ok := priorityClass(priName)
			if name == "" || !ok {
				prioFailed++
				continue
			}
			procs2, err := e.ops.listProcesses()
			if err != nil {
				prioFailed++
				continue
			}
			for _, p := range procs2 {
				if strings.EqualFold(p.Name, name) && !skipProc(p.Name) {
					if err := e.ops.setPriority(p.PID, pri); err == nil {
						prioApplied++
					} else {
						prioFailed++
					}
				}
			}
		}
	}
	log.Printf("[proc] speedup: trimmed=%d freed=%s failed=%d prio=%d/%d",
		trimmed, humanBytes(freed), failed, prioApplied, prioFailed)
	return true, "", "", map[string]any{
		"trimmedProcesses":  trimmed,
		"freedBytes":        freed,
		"failedProcesses":   failed,
		"prioritiesApplied": prioApplied,
		"prioritiesFailed":  prioFailed,
	}
}

// handleList returns the process snapshot joined with traffic accounting.
func (e *Engine) handleList(payload map[string]any) (bool, string, string, map[string]any) {
	reset := false
	if v, ok := payload["reset"].(bool); ok {
		reset = v
	}
	procs, err := e.ops.listProcesses()
	if err != nil {
		return false, "E_FAILED", "list processes failed: " + err.Error(), nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	rows := make([]map[string]any, 0, len(procs))
	for _, p := range procs {
		if skipProc(p.Name) {
			continue
		}
		row := map[string]any{
			"pid":        p.PID,
			"process":    p.Name,
			"workingSet": p.WorkingSet,
		}
		if acc := e.accum[p.PID]; acc != nil {
			row["totalIn"] = acc.TotalIn
			row["totalOut"] = acc.TotalOut
			row["rateIn"] = acc.RateIn
			row["rateOut"] = acc.RateOut
		} else {
			row["totalIn"] = int64(0)
			row["totalOut"] = int64(0)
			row["rateIn"] = int64(0)
			row["rateOut"] = int64(0)
		}
		rows = append(rows, row)
	}
	// top by working set (memory view is the acceleration UI's primary sort)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i]["workingSet"].(uint64) > rows[j]["workingSet"].(uint64)
	})
	const cap = 100
	truncated := false
	if len(rows) > cap {
		rows = rows[:cap]
		truncated = true
	}
	if reset {
		for _, acc := range e.accum {
			acc.TotalIn, acc.TotalOut = 0, 0
			acc.RateIn, acc.RateOut = 0, 0
		}
	}
	return true, "", "", map[string]any{
		"processes":      rows,
		"truncated":      truncated,
		"pollIntervalMs": e.opts.PollIntervalMS,
	}
}

// handlePriority adjusts the priority of one pid or every process of a name.
func (e *Engine) handlePriority(payload map[string]any) (bool, string, string, map[string]any) {
	priName, _ := payload["priority"].(string)
	pri, ok := priorityClass(priName)
	if !ok {
		return false, "E_PARAM", "priority must be idle|below_normal|normal|above_normal|high", nil
	}
	var pid uint32
	name := ""
	if n := procNum(payload, "pid"); n > 0 {
		pid = uint32(n)
	}
	if s, ok := payload["name"].(string); ok {
		name = s
	}
	if pid == 0 && name == "" {
		return false, "E_PARAM", "pid or name is required", nil
	}
	procs, err := e.ops.listProcesses()
	if err != nil {
		return false, "E_FAILED", "list processes failed: " + err.Error(), nil
	}
	var applied, failed int64
	for _, p := range procs {
		if pid != 0 && p.PID != pid {
			continue
		}
		if name != "" && !strings.EqualFold(p.Name, name) {
			continue
		}
		if skipProc(p.Name) {
			continue
		}
		if err := e.ops.setPriority(p.PID, pri); err == nil {
			applied++
		} else {
			failed++
		}
	}
	if applied == 0 && failed == 0 {
		return false, "E_PARAM", "no matching process", nil
	}
	return true, "", "", map[string]any{"applied": applied, "failed": failed}
}

// handleThrottle applies or removes an experimental policy-QoS throttle.
// Policy-QoS throttles outbound traffic of the matched executable; requires
// an elevated process (registry HKLM write).
func (e *Engine) handleThrottle(payload map[string]any) (bool, string, string, map[string]any) {
	exe, _ := payload["process"].(string)
	if exe == "" || !exeNameRe.MatchString(exe) {
		return false, "E_PARAM", "process must be a bare executable name like app.exe", nil
	}
	rate := procNum(payload, "rateKBps")
	if rate < 0 || rate > 10*1024*1024 {
		return false, "E_PARAM", "rateKBps out of range [0, 10485760]", nil
	}
	if rate > 0 && !e.ops.isElevated() {
		return false, "E_PLATFORM_UNSUPPORTED", "throttle requires an elevated process (admin)", nil
	}
	if err := e.ops.qosApply(exe, uint32(rate)); err != nil {
		log.Printf("[proc] qos apply %s=%dKBps failed: %v", exe, rate, err)
		return false, "E_FAILED", "qos apply failed: " + err.Error(), nil
	}
	log.Printf("[proc] qos throttle %s rateKBps=%d", exe, rate)
	return true, "", "", map[string]any{"process": exe, "rateKBps": rate, "experimental": true}
}

// --- payload helpers ---

// procNum reads a numeric field (json.Number, float64 or numeric string).
func procNum(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case interface{ Int64() (int64, error) }:
		if n, err := v.Int64(); err == nil {
			return n
		}
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// exeNameRe: bare executable names only (no paths, no wildcards).
var exeNameRe = regexp.MustCompile(`(?i)^[a-z0-9_.\-]{1,64}\.exe$`)

// skipProc: processes never touched by speedup/priority.
func skipProc(name string) bool {
	p := strings.ToLower(name)
	if p == "" {
		return true
	}
	if p == "explorer.exe" {
		return false // shell trimming is harmless; rules may still exclude
	}
	for _, s := range criticalProcesses {
		if p == s {
			return true
		}
	}
	for _, s := range ownProcesses {
		if p == s {
			return true
		}
	}
	return false
}

var criticalProcesses = []string{
	"csrss.exe", "winlogon.exe", "wininit.exe", "services.exe",
	"lsass.exe", "smss.exe", "dwm.exe", "fontdrvhost.exe",
	"svchost.exe", "system", "idle", "registry", "memcompression",
}

var ownProcesses = []string{
	"launcher.exe", "core.exe", "ability.exe", "agent.exe", "ocmctl.exe",
	"classaegislauncher.exe", "classaegiscore.exe", "classaegisability.exe", "classaegisdebug.exe", "classaegisctl.exe", "classaegisservice.exe",
	"课佑管家.exe", "守界管家.exe", "壹课管家.exe",
}

// priorityClass maps the protocol priority names to Windows classes.
func priorityClass(name string) (uint32, bool) {
	switch strings.ToLower(name) {
	case "idle":
		return priorityIdle, true
	case "below_normal":
		return priorityBelowNormal, true
	case "normal":
		return priorityNormal, true
	case "above_normal":
		return priorityAboveNormal, true
	case "high":
		return priorityHigh, true
	}
	return 0, false
}

// humanBytes renders a byte count for logs.
func humanBytes(n int64) string {
	const kb, mb = 1024, 1024 * 1024
	switch {
	case n >= mb:
		return strconv.FormatInt(n/mb, 10) + "." + strconv.Itoa(int((n%mb)*10/mb)) + "MB"
	case n >= kb:
		return strconv.FormatInt(n/kb, 10) + "KB"
	default:
		return strconv.FormatInt(n, 10) + "B"
	}
}

// --- platform seam (implementations in ops_windows.go / ops_other.go) ---

// procInfo is one process snapshot row.
type procInfo struct {
	PID        uint32
	Name       string
	WorkingSet uint64
}

// connSample is one TCP connection's cumulative byte counters.
type connSample struct {
	PID   uint32
	In    uint64
	Out   uint64
	Class string
}

// sysOps abstracts platform operations behind testable seams.
type sysOps interface {
	listProcesses() ([]procInfo, error)
	trimWorkingSet(pid uint32) (int64, error)
	setPriority(pid uint32, pri uint32) error
	connSamples() ([]connSample, error)
	qosApply(exe string, rateKBps uint32) error
	isElevated() bool
	foregroundPID() (uint32, error)
}
