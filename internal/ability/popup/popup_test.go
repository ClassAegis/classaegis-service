// SPDX-License-Identifier: AGPL-3.0-only
package popup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeSource is an in-memory WindowSource for deterministic tests.
type fakeSource struct {
	wins   []WindowInfo
	closed []uintptr
	hidden []uintptr
}

func (f *fakeSource) List() ([]WindowInfo, error) { return f.wins, nil }

func (f *fakeSource) Close(h uintptr) error {
	f.closed = append(f.closed, h)
	return nil
}

func (f *fakeSource) Hide(h uintptr) error {
	f.hidden = append(f.hidden, h)
	return nil
}

func newTestEngine(t *testing.T, rulesJSON string, wins []WindowInfo) (*Engine, *fakeSource) {
	t.Helper()
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.json")
	if rulesJSON != "" {
		if err := os.WriteFile(rulesPath, []byte(rulesJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := &fakeSource{wins: wins}
	eng := NewWithSource(Options{RulesPath: rulesPath, Enabled: true}, src)
	return eng, src
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*推广*", "某软件 - 推广弹窗", true},
		{"*推广*", "正常窗口", false},
		{"notepad?.exe", "notepadA.exe", true},
		{"notepad?.exe", "notepadAA.exe", false},
		{"EXE", "exe", true}, // case-insensitive
		{"*", "anything", true},
		{"a*b*c", "aXbYc", true},
		{"a*b*c", "aXcYb", false},
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.s); got != c.want {
			t.Errorf("globMatch(%q,%q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

// adRules seeds one block rule: process SomeAd.exe, title contains 推广.
const adRules = `{"version":1,"source":"test","block":[{"process":"somead.exe","title":"*推广*","action":"close","note":"SomeAd 推广弹窗"}]}`

func TestTickBlocksAndRecords(t *testing.T) {
	eng, src := newTestEngine(t, adRules, []WindowInfo{
		{HWND: 101, PID: 1001, Process: "SomeAd.exe", Title: "限时推广", Class: "AdClass"},
		{HWND: 102, PID: 1002, Process: "notepad.exe", Title: "a.txt - 记事本"},
	})
	eng.tick()
	if len(src.closed) != 1 || src.closed[0] != 101 {
		t.Fatalf("want hwnd 101 closed, got %v", src.closed)
	}
	if len(src.hidden) != 0 {
		t.Fatalf("hide should not fire for close rules, got %v", src.hidden)
	}
	eng.mu.Lock()
	n := len(eng.events)
	eng.mu.Unlock()
	if n != 1 {
		t.Fatalf("want 1 event, got %d", n)
	}
}

// TestTickCooldown: the same window is not acted on twice within the
// cooldown window (prevents event spam for stubborn popups).
func TestTickCooldown(t *testing.T) {
	eng, src := newTestEngine(t, adRules, []WindowInfo{
		{HWND: 101, PID: 1001, Process: "SomeAd.exe", Title: "限时推广"},
	})
	eng.tick()
	eng.tick()
	eng.tick()
	if len(src.closed) != 1 {
		t.Fatalf("cooldown should keep a single close, got %d", len(src.closed))
	}
	// after cooldown expiry, the window is acted on again
	eng.cooled[101] = time.Now().Add(-11 * time.Second)
	eng.tick()
	if len(src.closed) != 2 {
		t.Fatalf("want second close after cooldown expiry, got %d", len(src.closed))
	}
}

// TestSkipSafety: unidentified, system and self processes are never touched;
// whitelist wins over block rules.
func TestSkipSafety(t *testing.T) {
	rules := `{"version":1,"source":"test","block":[{"title":"*推广*"}],"whitelist":[{"process":"allowed.exe"}]}`
	eng, src := newTestEngine(t, rules, []WindowInfo{
		{HWND: 201, PID: 0, Process: "", Title: "推广"},
		{HWND: 202, PID: 4, Process: "csrss.exe", Title: "推广"},
		{HWND: 203, PID: 5, Process: "ability.exe", Title: "推广"},
		{HWND: 204, PID: 6, Process: "Allowed.exe", Title: "推广"},
		{HWND: 205, PID: 7, Process: "csrss.exe", Title: "推广"},
	})
	eng.tick()
	if len(src.closed) != 0 {
		t.Fatalf("no window should be touched, got %v", src.closed)
	}
}

// TestEventsPullAndClear: POPUP_GET_EVENTS returns oldest-first batches with
// hasMore, and clear=true removes returned events.
func TestEventsPullAndClear(t *testing.T) {
	eng, _ := newTestEngine(t, adRules, nil)
	eng.record(Event{Rule: "r1"})
	eng.record(Event{Rule: "r2"})
	eng.record(Event{Rule: "r3"})

	ok, code, message, res := eng.Handle("m1", "POPUP_GET_EVENTS", map[string]any{}, nil)
	if !ok || len(res["events"].([]Event)) != 3 {
		t.Fatalf("want 3 events: %v %s %s", ok, code, message)
	}
	ok, _, _, res = eng.Handle("m2", "POPUP_GET_EVENTS", map[string]any{"clear": true}, nil)
	evs := res["events"].([]Event)
	if len(evs) != 3 || evs[0].Rule != "r1" {
		t.Fatalf("want oldest-first batch, got %v", evs)
	}
	if res["hasMore"] != false {
		t.Fatal("hasMore should be false")
	}
	eng.mu.Lock()
	n := len(eng.events)
	eng.mu.Unlock()
	if n != 0 {
		t.Fatalf("clear should empty the buffer, got %d", n)
	}
	ok, _, _, res = eng.Handle("m3", "POPUP_GET_EVENTS", map[string]any{"clear": true}, nil)
	if !ok || len(res["events"].([]Event)) != 0 {
		t.Fatal("empty buffer should return empty batch")
	}
}

// TestRulesUpdateFileAndInline: valid file/inline payloads replace rules;
// invalid ones are rejected with E_RULES_FORMAT keeping the old set.
func TestRulesUpdateFileAndInline(t *testing.T) {
	eng, _ := newTestEngine(t, adRules, nil)

	// inline update replaces the rule set
	ok, code, message, res := eng.Handle("m1", "POPUP_RULES_UPDATE", map[string]any{
		"rules": map[string]any{
			"version": 2,
			"block":   []any{map[string]any{"title": "*大促*"}},
		},
	}, nil)
	if !ok || res["version"] != 2 {
		t.Fatalf("inline update failed: %s %s %v", code, message, res)
	}
	ok, _, _, tm := eng.Handle("m2", "POPUP_TEST_MATCH", map[string]any{"title": "年终大促"}, nil)
	if !ok || tm["matched"] != true {
		t.Fatalf("new rule should match: %v", tm)
	}
	ok, _, _, tm = eng.Handle("m3", "POPUP_TEST_MATCH", map[string]any{"title": "限时推广"}, nil)
	if !ok || tm["matched"] != false {
		t.Fatalf("old rule should be gone: %v", tm)
	}

	// file update
	rulesPath := filepath.Join(t.TempDir(), "more.json")
	if err := os.WriteFile(rulesPath, []byte(`{"version":3,"source":"test","block":[{"process":"b.exe"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, code, message, res = eng.Handle("m4", "POPUP_RULES_UPDATE", map[string]any{"path": rulesPath}, nil)
	if !ok || res["version"] != 3 {
		t.Fatalf("file update failed: %s %s %v", code, message, res)
	}

	// invalid file rejected, old rules survive
	badPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badPath, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, code, _, _ = eng.Handle("m5", "POPUP_RULES_UPDATE", map[string]any{"path": badPath}, nil)
	if ok || code != "E_RULES_FORMAT" {
		t.Fatalf("want E_RULES_FORMAT, got ok=%v code=%s", ok, code)
	}
	if e := eng.rules.Load(); e.Version != 3 {
		t.Fatalf("old rule set must survive rejected update, got v%d", e.Version)
	}
}

// TestSetEnabledAndTestMatchErrors: toggle state transitions and param errors.
func TestSetEnabledAndTestMatchErrors(t *testing.T) {
	eng, _ := newTestEngine(t, "", nil)
	ok, code, _, res := eng.Handle("m1", "POPUP_SET_ENABLED", map[string]any{"enabled": false}, nil)
	if !ok || res["enabled"] != false || eng.enabled.Load() {
		t.Fatalf("disable failed: %s %v %v", code, res, eng.enabled.Load())
	}
	ok, code, _, res = eng.Handle("m2", "POPUP_SET_ENABLED", map[string]any{"enabled": true}, nil)
	if !ok || res["enabled"] != true || !eng.enabled.Load() {
		t.Fatalf("enable failed: %s %v %v", code, res, eng.enabled.Load())
	}
	ok, code, _, _ = eng.Handle("m3", "POPUP_SET_ENABLED", map[string]any{"enabled": "yes"}, nil)
	if ok || code != "E_PARAM" {
		t.Fatalf("non-bool enabled must fail with E_PARAM, got ok=%v code=%s", ok, code)
	}
	ok, code, _, _ = eng.Handle("m4", "POPUP_TEST_MATCH", map[string]any{}, nil)
	if ok || code != "E_PARAM" {
		t.Fatalf("empty test match must fail with E_PARAM, got ok=%v code=%s", ok, code)
	}
	// SET_ENABLED on a fresh engine starts the watcher lazily
	if !eng.started {
		t.Fatal("watcher should start after enable")
	}
	// stop the watcher to leave a clean state
	if eng.stop != nil {
		close(eng.stop)
	}
}

// TestParseRuleSetValidation: bad version rejected; empty-dimension rules
// and bad actions dropped.
func TestParseRuleSetValidation(t *testing.T) {
	if _, err := ParseRuleSet([]byte(`{"version":0}`)); err == nil {
		t.Fatal("version 0 must be rejected")
	}
	rs, err := ParseRuleSet([]byte(`{"version":1,"block":[
		{"title":"  ","action":"bogus"},
		{"process":"a.exe"},
		{"title":"*x*","action":"hide"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Block) != 2 {
		t.Fatalf("want 2 valid rules, got %d", len(rs.Block))
	}
	if rs.Block[0].Process != "a.exe" || rs.Block[1].Action != "hide" {
		t.Fatalf("rule order/content wrong: %+v", rs.Block)
	}
}
