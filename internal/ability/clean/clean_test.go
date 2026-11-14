// SPDX-License-Identifier: AGPL-3.0-only
package clean

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeRecycler records empty calls; query is scripted per drive.
type fakeRecycler struct {
	files    map[string]int64
	bytes    map[string]int64
	emptied  []string
	queryErr error
}

func (f *fakeRecycler) Query(drive string) (int64, int64, error) {
	if f.queryErr != nil {
		return 0, 0, f.queryErr
	}
	return f.files[drive], f.bytes[drive], nil
}

func (f *fakeRecycler) Empty(drive string) error {
	f.emptied = append(f.emptied, drive)
	return nil
}

// useTestEnv replaces the %VAR% expander with a fixed mapping (no process
// env changes: they break tooling that relies on TEMP).
func useTestEnv(t *testing.T, m map[string]string) {
	t.Helper()
	old := expandEnv
	expandEnv = func(p string) string {
		return envRe.ReplaceAllStringFunc(p, func(v string) string {
			return m[v[1:len(v)-1]]
		})
	}
	t.Cleanup(func() { expandEnv = old })
}

func newTestEngine(t *testing.T, minAgeHours int) (*Engine, *fakeRecycler) {
	t.Helper()
	eng := New(Options{
		MinAgeHours: minAgeHours,
	})
	fr := &fakeRecycler{
		files: map[string]int64{"C:\\": 3},
		bytes: map[string]int64{"C:\\": 4096},
	}
	eng.recycle = fr
	return eng, fr
}

func writeJunk(t *testing.T, path string, size int, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if age != 0 {
		past := time.Now().Add(-age)
		if err := os.Chtimes(path, past, past); err != nil {
			t.Fatal(err)
		}
	}
}

// TestScanTempAgeGuard: files younger than the age guard are not counted.
func TestScanTempAgeGuard(t *testing.T) {
	tmpDir := t.TempDir()
	root := t.TempDir()
	useTestEnv(t, map[string]string{"TEMP": tmpDir, "TMP": tmpDir, "SystemRoot": root})
	if err := os.MkdirAll(filepath.Join(root, "Temp"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng, _ := newTestEngine(t, 24)
	writeJunk(t, filepath.Join(tmpDir, "old.tmp"), 100, 48*time.Hour)
	writeJunk(t, filepath.Join(tmpDir, "fresh.tmp"), 50, 0)
	ok, code, message, res := eng.Handle("m1", "CLEAN_SCAN", map[string]any{"categories": []any{"temp"}}, nil)
	if !ok {
		t.Fatalf("scan failed: %s %s", code, message)
	}
	if res["totalFiles"] != int64(1) || res["totalBytes"] != int64(100) {
		t.Fatalf("want 1 file/100 bytes, got %v", res)
	}

	findings := res["findings"].([]junkItem)
	if len(findings) != 1 || findings[0].Path != filepath.Join(tmpDir, "old.tmp") {
		t.Fatalf("findings wrong: %+v", findings)
	}
}

// TestRunDeletesJunkAndSkipsLocked: run mode deletes eligible files; a file
// held open (Windows sharing lock) counts as failed, not deleted.
func TestRunDeletesJunkAndSkipsLocked(t *testing.T) {
	tmpDir := t.TempDir()
	root := t.TempDir()
	useTestEnv(t, map[string]string{"TEMP": tmpDir, "TMP": tmpDir, "SystemRoot": root})
	if err := os.MkdirAll(filepath.Join(root, "Temp"), 0o755); err != nil {
		t.Fatal(err)
	}
	eng, _ := newTestEngine(t, -1) // no age guard
	junk := filepath.Join(tmpDir, "junk.bin")
	writeJunk(t, junk, 200, 0)
	locked := filepath.Join(tmpDir, "locked.bin")
	writeJunk(t, locked, 100, 0)
	f, err := os.Open(locked)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ok, code, message, res := eng.Handle("m1", "CLEAN_RUN", map[string]any{"categories": []any{"temp"}}, nil)
	if !ok {
		t.Fatalf("run failed: %s %s", code, message)
	}
	if res["deletedFiles"] != int64(1) || res["deletedBytes"] != int64(200) {
		t.Fatalf("want 1 deleted/200 bytes, got %v", res)
	}
	if res["failedFiles"] != int64(1) {
		t.Fatalf("want 1 failed (locked), got %v", res)
	}
	if _, err := os.Stat(locked); err != nil {
		t.Fatal("locked file must survive")
	}
	if _, err := os.Stat(junk); !os.IsNotExist(err) {
		t.Fatal("junk file should be gone")
	}
}

// TestRecycleBinFlow: scan counts the aggregate; run empties via the API.
func TestRecycleBinFlow(t *testing.T) {
	eng, fr := newTestEngine(t, 24)
	ok, code, message, res := eng.Handle("m1", "CLEAN_SCAN", map[string]any{"categories": []any{"recycle_bin"}}, nil)
	if !ok {
		t.Fatalf("scan failed: %s %s", code, message)
	}
	if res["totalFiles"] != int64(3) || res["totalBytes"] != int64(4096) {
		t.Fatalf("want 3/4096, got %v", res)
	}
	if len(fr.emptied) != 0 {
		t.Fatal("scan must not empty the bin")
	}
	ok, code, message, res = eng.Handle("m2", "CLEAN_RUN", map[string]any{"categories": []any{"recycle_bin"}}, nil)
	if !ok {
		t.Fatalf("run failed: %s %s", code, message)
	}
	if res["deletedFiles"] != int64(3) || res["deletedBytes"] != int64(4096) {
		t.Fatalf("want 3 deleted/4096 bytes, got %v", res)
	}
	if len(fr.emptied) != 1 || fr.emptied[0] != "C:\\" {
		t.Fatalf("bin should be emptied on C:\\ once, got %v", fr.emptied)
	}
}

// TestCategoryParamErrors: unknown and not-enabled categories are rejected;
// a running pass blocks a new one.
func TestCategoryParamErrors(t *testing.T) {
	eng, _ := newTestEngine(t, 24)
	ok, code, _, _ := eng.Handle("m1", "CLEAN_SCAN", map[string]any{"categories": []any{"nonexistent"}}, nil)
	if ok || code != "E_PARAM" {
		t.Fatalf("unknown category must fail, got ok=%v code=%s", ok, code)
	}
	ok, code, _, _ = eng.Handle("m2", "CLEAN_SCAN", map[string]any{"categories": []any{"windows_old"}}, nil)
	if ok || code != "E_PARAM" {
		t.Fatalf("not-enabled category must fail, got ok=%v code=%s", ok, code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &cleanRun{id: "x", mode: "scan", ctx: ctx, cancel: cancel,
		started: time.Now(), catStats: map[string]*catStat{}}
	run.running.Store(true)
	eng.run = run
	ok, code, _, _ = eng.Handle("m3", "CLEAN_SCAN", map[string]any{"categories": []any{"temp"}}, nil)
	if ok || code != "E_SCAN_RUNNING" {
		t.Fatalf("want E_SCAN_RUNNING, got ok=%v code=%s", ok, code)
	}
}

// TestScanCancelled: a pre-cancelled run reports cancelled=true.
func TestScanCancelled(t *testing.T) {
	eng, _ := newTestEngine(t, 24)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run := &cleanRun{id: "x", mode: "scan", ctx: ctx, cancel: cancel,
		started: time.Now(), catStats: map[string]*catStat{}}
	run.running.Store(true)
	summary, ok, code, message := eng.runPass(run, eng.cats, "scan", nil)
	if !ok || code != "" {
		t.Fatalf("cancelled pass should return ok: %s %s", code, message)
	}
	if summary["cancelled"] != true {
		t.Fatalf("want cancelled=true, got %v", summary)
	}
}

// TestStatusAfterScan: CLEAN_STATUS exposes the last scan summary.
func TestStatusAfterScan(t *testing.T) {
	eng, _ := newTestEngine(t, 24)
	tmpDir := t.TempDir()
	root := t.TempDir()
	useTestEnv(t, map[string]string{"TEMP": tmpDir, "TMP": tmpDir, "SystemRoot": root})
	ok, _, _, _ := eng.Handle("m1", "CLEAN_SCAN", map[string]any{"categories": []any{"temp"}}, nil)
	if !ok {
		t.Fatal("scan failed")
	}
	ok, _, _, res := eng.Handle("m2", "CLEAN_STATUS", nil, nil)
	if !ok || res["running"] != false || res["lastScan"] == nil {
		t.Fatalf("status wrong: %v", res)
	}
}

// TestResolveCategoriesMatrix: defaults, opt-in/out, subsets, unknown.
func TestResolveCategoriesMatrix(t *testing.T) {
	defs := defaultCategories(0)
	cats, msg := resolveCategories(defs, nil, nil, nil)
	if msg != "" || len(cats) != 7 {
		t.Fatalf("default set: %d cats, msg=%q", len(cats), msg)
	}
	cats, msg = resolveCategories(defs, []string{"windows_old"}, nil, nil)
	if msg != "" || len(cats) != 8 {
		t.Fatalf("opt-in windows_old: %d cats, msg=%q", len(cats), msg)
	}
	cats, msg = resolveCategories(defs, nil, []string{"thumbcache"}, nil)
	if msg != "" || len(cats) != 6 {
		t.Fatalf("disable thumbcache: %d cats msg=%q", len(cats), msg)
	}
	// a subset containing a not-enabled category errors (protocol §6.4)
	if _, msg = resolveCategories(defs, nil, []string{"thumbcache"}, []string{"temp", "thumbcache"}); msg == "" {
		t.Fatal("not-enabled subset id must error")
	}
	cats, msg = resolveCategories(defs, nil, nil, []string{"temp", "windows_update"})
	if msg != "" || len(cats) != 2 || cats[0].ID != "temp" || cats[1].ID != "windows_update" {
		t.Fatalf("valid subset: %+v msg=%q", cats, msg)
	}
	if _, msg = resolveCategories(defs, nil, nil, []string{"nope"}); msg == "" {
		t.Fatal("unknown subset id must error")
	}
}

// TestExpandEnv: %VAR% expansion; unknown vars vanish.
func TestExpandEnv(t *testing.T) {
	t.Setenv("OCM_TEST_VAR", "hello")
	if got := expandEnv("a_%OCM_TEST_VAR%_b"); got != "a_hello_b" {
		t.Fatalf("expand failed: %q", got)
	}
	if got := expandEnv("%OCM_MISSING%"); got != "" {
		t.Fatalf("missing var should vanish, got %q", got)
	}
}
