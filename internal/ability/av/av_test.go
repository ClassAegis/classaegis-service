// SPDX-License-Identifier: AGPL-3.0-only
package av

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// eicarBytes assembles the standard 68-byte EICAR test string at runtime
// from fragments, so the full string never appears in source or binaries.
func eicarBytes() []byte {
	s := "X5O!P%@ap[4\\PZX54(P^)7CC)7}$"
	s += "EIC" + "AR-ST" + "ANDARD-A" + "NTIVIRUS-TEST-FILE!$H+H*"
	return []byte(s)
}

// hashOf returns the hex SHA-256 of data.
func hashOf(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// newTestEngine builds an Engine on temp dirs; dbJSON == "" means no db file.
func newTestEngine(t *testing.T, dbJSON string) *Engine {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sig.json")
	if dbJSON != "" {
		if err := os.WriteFile(dbPath, []byte(dbJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return New(Options{
		DbPath:           dbPath,
		QuarantineDir:    filepath.Join(dir, "quar"),
		Threads:          2,
		EnableHeuristics: true,
		DefaultOnDetect:  "quarantine",
	})
}

// TestScanDetectsEICARAndQuarantines is the end-to-end engine test: scan a
// temp tree containing an EICAR test file, expect detection + quarantine +
// restore + delete roundtrip via the command handlers.
func TestScanDetectsEICARAndQuarantines(t *testing.T) {
	shaHex := hashOf(t, eicarBytes())
	dbJSON := fmt.Sprintf(`{"version":1,"source":"test","hashes":[{"sha256":%q,"name":"EICAR-Test-File","severity":"high","category":"test"}]}`, shaHex)
	eng := newTestEngine(t, dbJSON)

	srcDir := t.TempDir()
	eicarPath := filepath.Join(srcDir, "eicar.com")
	if err := os.WriteFile(eicarPath, eicarBytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	benign := filepath.Join(srcDir, "hello.txt")
	if err := os.WriteFile(benign, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	progCount := 0
	ok, code, message, result := eng.Handle("m1", "AV_SCAN", map[string]any{
		"mode":  "custom",
		"paths": []any{srcDir},
	}, func(map[string]any) { progCount++ })
	if !ok {
		t.Fatalf("scan failed: %s %s", code, message)
	}
	if result == nil || result["threatsTotal"] != int64(1) {
		t.Fatalf("want 1 threat, got result=%v", result)
	}
	threats := result["threats"].([]Finding)
	if threats[0].Action != "quarantined" {
		t.Fatalf("want quarantined, got %q", threats[0].Action)
	}
	if _, err := os.Stat(eicarPath); !os.IsNotExist(err) {
		t.Fatalf("eicar file still at origin after quarantine")
	}

	// quarantine list
	_, _, _, listRes := eng.Handle("m2", "AV_QUARANTINE_LIST", nil, nil)
	items, _ := json.Marshal(listRes["items"])
	var parsed []QuarantineItem
	if err := json.Unmarshal(items, &parsed); err != nil || len(parsed) != 1 {
		t.Fatalf("want 1 quarantine item, got %s (%v)", items, err)
	}
	id := parsed[0].ID
	if parsed[0].OriginalPath != eicarPath || parsed[0].Sha256 != shaHex {
		t.Fatalf("quarantine meta mismatch: %+v", parsed[0])
	}

	// restore roundtrip
	ok, code, message, _ = eng.Handle("m3", "AV_QUARANTINE_RESTORE", map[string]any{"id": id}, nil)
	if !ok {
		t.Fatalf("restore failed: %s %s", code, message)
	}
	back, err := os.ReadFile(eicarPath)
	if err != nil || string(back) != string(eicarBytes()) {
		t.Fatalf("restored file content mismatch (err=%v)", err)
	}

	// re-detect and delete permanently
	ok, _, _, _ = eng.Handle("m4", "AV_SCAN", map[string]any{"mode": "custom", "paths": []any{srcDir}}, nil)
	if !ok {
		t.Fatal("rescan failed")
	}
	_, _, _, listRes2 := eng.Handle("m5", "AV_QUARANTINE_LIST", nil, nil)
	items2, _ := json.Marshal(listRes2["items"])
	var parsed2 []QuarantineItem
	_ = json.Unmarshal(items2, &parsed2)
	if len(parsed2) != 1 {
		t.Fatalf("want 1 item after rescan, got %d", len(parsed2))
	}
	ok, code, message, _ = eng.Handle("m6", "AV_QUARANTINE_DELETE", map[string]any{"id": parsed2[0].ID}, nil)
	if !ok {
		t.Fatalf("delete failed: %s %s", code, message)
	}
	if _, err := os.Stat(eicarPath); !os.IsNotExist(err) {
		t.Fatal("file should stay removed after delete")
	}
}

// TestScanPatternRule: a hex-encoded byte pattern in the DB matches content.
func TestScanPatternRule(t *testing.T) {
	pattern := []byte("OCM-SIGNATURE-TEST-PATTERN")
	dbJSON := fmt.Sprintf(`{"version":1,"source":"test","patterns":[{"pattern":%q,"name":"OCM-Signature-Test-Pattern","severity":"medium","category":"test"}]}`, hex.EncodeToString(pattern))
	eng := newTestEngine(t, dbJSON)

	dir := t.TempDir()
	hit := filepath.Join(dir, "hit.dat")
	os.WriteFile(hit, append([]byte("padding-"), pattern...), 0o644)
	os.WriteFile(filepath.Join(dir, "clean.txt"), []byte("nothing here"), 0o644)

	ok, code, message, result := eng.Handle("m1", "AV_SCAN", map[string]any{"mode": "custom", "paths": []any{dir}}, nil)
	if !ok {
		t.Fatalf("scan failed: %s %s", code, message)
	}
	if result["threatsTotal"] != int64(1) {
		t.Fatalf("want 1 threat, got %v", result)
	}
	threats := result["threats"].([]Finding)
	if threats[0].Rule != "OCM-Signature-Test-Pattern" {
		t.Fatalf("wrong rule: %q", threats[0].Rule)
	}
}

// TestScanHeuristicsDoubleExtension: invoice.pdf.exe is flagged without any
// signature rule.
func TestScanHeuristicsDoubleExtension(t *testing.T) {
	eng := newTestEngine(t, "")
	dir := t.TempDir()
	sneaky := filepath.Join(dir, "invoice.pdf.exe")
	os.WriteFile(sneaky, []byte("MZ...pretend"), 0o644)
	os.WriteFile(filepath.Join(dir, "normal.pdf"), []byte("%PDF-1.4"), 0o644)

	ok, code, message, result := eng.Handle("m1", "AV_SCAN", map[string]any{"mode": "custom", "paths": []any{dir}}, nil)
	if !ok {
		t.Fatalf("scan failed: %s %s", code, message)
	}
	if result["threatsTotal"] != int64(1) {
		t.Fatalf("want 1 threat, got %v", result)
	}
	threats := result["threats"].([]Finding)
	if threats[0].Rule != "Heuristic.DoubleExtension" || threats[0].Severity != "medium" {
		t.Fatalf("wrong finding: %+v", threats[0])
	}
}

// TestScanExclusions: excluded subtrees are not scanned.
func TestScanExclusions(t *testing.T) {
	shaHex := hashOf(t, eicarBytes())
	dbJSON := fmt.Sprintf(`{"version":1,"source":"test","hashes":[{"sha256":%q,"name":"EICAR-Test-File","severity":"high","category":"test"}]}`, shaHex)
	eng := newTestEngine(t, dbJSON)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "eicar.com"), eicarBytes(), 0o644)
	sub := filepath.Join(dir, "excluded")
	os.Mkdir(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "eicar.com"), eicarBytes(), 0o644)
	eng.opts.Exclusions = append(eng.opts.Exclusions, sub)

	ok, code, message, result := eng.Handle("m1", "AV_SCAN", map[string]any{"mode": "custom", "paths": []any{dir}}, nil)
	if !ok {
		t.Fatalf("scan failed: %s %s", code, message)
	}
	if result["threatsTotal"] != int64(1) {
		t.Fatalf("want exactly the non-excluded EICAR, got %v", result)
	}
}

// TestScanCancel: a cancelled run reports cancelled=true and scans nothing.
func TestScanCancel(t *testing.T) {
	eng := newTestEngine(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run := &scanRun{id: newScanID(), ctx: ctx, cancel: cancel, started: time.Now()}
	run.running.Store(true)
	opts := scanOptions{mode: "custom", roots: []string{t.TempDir()}, onDetect: "report"}
	summary, ok, code, message := eng.runScan(run, opts, nil)
	if !ok || code != "" {
		t.Fatalf("cancelled scan should return ok: %s %s", code, message)
	}
	if summary["cancelled"] != true {
		t.Fatalf("want cancelled=true, got %v", summary)
	}
	if summary["scannedFiles"] != int64(0) {
		t.Fatalf("want 0 scanned, got %v", summary)
	}
}

// TestSignatureDBValidation: bad version errors; bad hashes dropped; valid
// hex pattern decodes.
func TestSignatureDBValidation(t *testing.T) {
	if _, err := ParseSignatureDB([]byte(`{"version":0}`)); err == nil {
		t.Fatal("version 0 must be rejected")
	}
	db, err := ParseSignatureDB([]byte(`{"version":1,"hashes":[{"sha256":"nothex","name":"bad"},{"sha256":"` + strings.Repeat("a", 64) + `","name":"good"}],"patterns":[{"pattern":"zz","name":"badhex"},{"pattern":"4142","name":"goodpat"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Hashes) != 1 || db.Hashes[0].Name != "good" {
		t.Fatalf("want only the good hash rule, got %+v", db.Hashes)
	}
	db.patternBytes = decodePatterns(db.Patterns)
	if len(db.patternBytes) != 1 {
		t.Fatalf("want only the good pattern, got %d", len(db.patternBytes))
	}
}

// TestHandleDBUpdate: a valid local file replaces the DB; an invalid file
// keeps the old one.
func TestHandleDBUpdate(t *testing.T) {
	eng := newTestEngine(t, "")
	shaHex := hashOf(t, eicarBytes())
	good := fmt.Sprintf(`{"version":2,"source":"test","hashes":[{"sha256":%q,"name":"X","severity":"high","category":"test"}]}`, shaHex)
	bad := `{"broken`

	goodPath := filepath.Join(t.TempDir(), "good.json")
	os.WriteFile(goodPath, []byte(good), 0o644)
	ok, code, message, res := eng.Handle("m1", "AV_DB_UPDATE", map[string]any{"path": goodPath}, nil)
	if !ok || res["version"] != 2 {
		t.Fatalf("db update failed: %s %s res=%v", code, message, res)
	}
	if len(eng.db.Load().Hashes) != 1 {
		t.Fatal("new db not in effect")
	}

	badPath := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(badPath, []byte(bad), 0o644)
	ok, code, _, _ = eng.Handle("m2", "AV_DB_UPDATE", map[string]any{"path": badPath}, nil)
	if ok || code != "E_DB_FORMAT" {
		t.Fatalf("invalid db must be rejected with E_DB_FORMAT, got ok=%v code=%s", ok, code)
	}
	if len(eng.db.Load().Hashes) != 1 {
		t.Fatal("old db must survive a rejected update")
	}
}

// TestScanRunningGuard: second scan while one runs is rejected.
func TestScanRunningGuard(t *testing.T) {
	eng := newTestEngine(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	run := &scanRun{id: newScanID(), ctx: ctx, cancel: cancel, started: time.Now()}
	run.running.Store(true)
	eng.scan = run
	ok, code, _, _ := eng.Handle("m1", "AV_SCAN", map[string]any{"mode": "quick"}, nil)
	if ok || code != "E_SCAN_RUNNING" {
		t.Fatalf("want E_SCAN_RUNNING, got ok=%v code=%s", ok, code)
	}
}

// TestSeedSignatureDB validates the shipped example signature DB against a
// freshly assembled EICAR test string.
func TestSeedSignatureDB(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	seed := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "conf", "av-signatures.example.json")
	db, err := LoadSignatureDB(seed)
	if err != nil {
		t.Fatalf("seed db load failed: %v", err)
	}
	if db.Version != 1 || len(db.Hashes) != 1 || len(db.Patterns) != 1 {
		t.Fatalf("unexpected seed db shape: v%d hashes=%d patterns=%d", db.Version, len(db.Hashes), len(db.Patterns))
	}
	eicar := eicarBytes()
	sum := sha256.Sum256(eicar)
	if _, ok := db.LookupHash(hex.EncodeToString(sum[:])); !ok {
		t.Fatal("seed EICAR hash rule does not match a real EICAR file")
	}
	if _, ok := db.LookupPattern(append([]byte("xx"), []byte("OCM-SIGNATURE-TEST-PATTERN")...)); !ok {
		t.Fatal("seed test pattern rule does not match")
	}
}
