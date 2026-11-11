//go:build windows

package audit

import (
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"unicode/utf16"
	"unsafe"
)

// TestESEEndToEnd creates a real ESE database via esent.dll, inserts rows,
// then reads them back through the same binding layer the audit engine uses.
// createTestESE builds a fresh ESE database with a History table and two
// rows via esent.dll; returns false after logging a fatal test error.
func createTestESE(t *testing.T, dir, dbPath string) bool {
	logBase := "ocm"
	var inst uintptr
	if r := int32(callProc(procJetCreateInstanceW, &inst, wstr("ocm-test"))); r < 0 {
		t.Fatalf("JetCreateInstanceW: %d", r)
		return false
	}
	wDir := wstr(dir)
	setParam := func(pid uintptr, str *uint16) error {
		r := int32(callProc(procJetSetSysParamW, &inst, 0, pid, 0, str))
		if r != 0 {
			return fmt.Errorf("SetSystemParameter(%d): JET_err %d", pid, r)
		}
		return nil
	}
	for pid, s := range map[uintptr]*uint16{0: wDir, 1: wDir, 2: wDir, 3: wstr(logBase)} {
		if err := setParam(pid, s); err != nil {
			t.Fatal(err)
			return false
		}
	}
	if r := int32(callProc(procJetInit, &inst)); r < 0 {
		t.Fatalf("JetInit: %d", r)
		return false
	}
	var ses uintptr
	if r := int32(callProc(procJetBeginSessionW, inst, &ses, 0, 0)); r < 0 {
		t.Fatalf("JetBeginSessionW: %d", r)
		return false
	}
	wDB := wstr(dbPath)
	var db uintptr
	if r := int32(callProc(esent.NewProc("JetCreateDatabaseW"), ses, wDB, 0, &db, 0)); r < 0 {
		t.Fatalf("JetCreateDatabaseW: %d", r)
		return false
	}
	var cols [4]jetColumnDef
	mk := func(i int, coltyp uint32, cp uint16, cbMax uint32) {
		cols[i].CbStruct = uint32(unsafe.Sizeof(cols[i]))
		cols[i].Coltyp = coltyp
		cols[i].Cp = cp
		cols[i].CbMax = cbMax
	}
	mk(0, jetColtypLongText, 1200, 0)
	mk(1, jetColtypLongText, 1200, 0)
	mk(2, jetColtypDateTime, 0, 8)
	mk(3, 4, 0, 4) // JET_coltypLong
	var tid uintptr
	if r := int32(callProc(esent.NewProc("JetCreateTable"), ses, db,
		astr("History"), 1, 80, &tid)); r < 0 {
		t.Fatalf("JetCreateTable: %d", r)
		return false
	}
	jetAddColumn := esent.NewProc("JetAddColumn")
	var colIDs [4]uint32
	for i, name := range []string{"Url", "Title", "LastVisited", "UsageCount"} {
		if r := int32(callProc(jetAddColumn, ses, tid, astr(name),
			&cols[i], 0, 0, &colIDs[i])); r < 0 {
			t.Fatalf("JetAddColumn %s: %d", name, r)
			return false
		}
	}
	jetBegin := esent.NewProc("JetBeginTransaction")
	jetPrepare := esent.NewProc("JetPrepareUpdate")
	jetSetColumn := esent.NewProc("JetSetColumn")
	jetUpdate := esent.NewProc("JetUpdate")
	jetCommit := esent.NewProc("JetCommitTransaction")
	insertRow := func(url, title string, oleDays float64, usage int64) {
		_ = callProc(jetBegin, ses, 0)
		_ = callProc(jetPrepare, ses, tid, 0)
		setUTF16 := func(colid uint32, s string) {
			u := utf16.Encode([]rune(s))
			b := make([]byte, len(u)*2)
			for i, w := range u {
				binary.LittleEndian.PutUint16(b[2*i:], w)
			}
			callProc(jetSetColumn, ses, tid, colid, &b[0], len(b), 0, 0)
		}
		setUTF16(colIDs[0], url)
		setUTF16(colIDs[1], title)
		var d [8]byte
		binary.LittleEndian.PutUint64(d[:], math.Float64bits(oleDays))
		callProc(jetSetColumn, ses, tid, colIDs[2], &d[0], 8, 0, 0)
		var n [4]byte
		binary.LittleEndian.PutUint32(n[:], uint32(usage))
		callProc(jetSetColumn, ses, tid, colIDs[3], &n[0], 4, 0, 0)
		_ = callProc(jetUpdate, ses, tid, 0, 0, 0)
		_ = callProc(jetCommit, ses, 0, 0)
	}
	insertRow("https://example.edu/", "教学站", 45000.5, 3)
	insertRow("https://www.example.com/", "Example", 45001.5, 1)
	_ = callProc(esent.NewProc("JetCloseTable"), ses, tid)
	_ = callProc(esent.NewProc("JetCloseDatabase"), ses, db, 0)
	_ = callProc(esent.NewProc("JetEndSession"), ses, 0)
	_ = callProc(procJetTerm, inst, 0)
	return true
}

func TestESEEndToEnd(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.dat")
	if !createTestESE(t, dir, dbPath) {
		return
	}

	// read back through the audit binding layer
	js, err := openESECopy(dbPath, filepath.Join(dir, "work"))
	if err != nil {
		t.Fatalf("openESECopy: %v", err)
	}
	defer js.Close()
	recs, err := js.readHistory(100)
	if err != nil {
		t.Fatalf("readHistory: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	if recs[0].URL != "https://example.edu/" || recs[0].Title != "教学站" {
		t.Fatalf("record0 wrong: %+v", recs[0])
	}
	if recs[0].Time != "2023-03-15T12:00:00Z" {
		t.Fatalf("OLE time conversion: %q", recs[0].Time)
	}
	if recs[1].Visits != 1 {
		t.Fatalf("usage count wrong: %+v", recs[1])
	}
}
