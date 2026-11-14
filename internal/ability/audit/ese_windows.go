// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package audit

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ESE (Extensible Storage Engine) reader via esent.dll, Windows-native.
// The live WebCacheV01.dat is locked by the system, so we copy the database
// together with its log/checkpoint files and soft-recover the copy.

const (
	jetErrNoCurrentRecord = -1603
	jetDbReadOnly         = 0x1
	jetTableReadOnly      = 0x2
	jetMoveFirst          = 0
	jetMoveNext           = 1
	jetColInfo            = 0

	jetColtypDateTime   = 8
	jetColtypBinary     = 9
	jetColtypText       = 10
	jetColtypLongBinary = 11
	jetColtypLongText   = 12

	jetCPUnicode = 1200
)

var (
	esent                  = windows.NewLazySystemDLL("esent.dll")
	procJetCreateInstanceW = esent.NewProc("JetCreateInstanceW")
	procJetSetSysParamW    = esent.NewProc("JetSetSystemParameterW")
	procJetInit            = esent.NewProc("JetInit")
	procJetBeginSessionW   = esent.NewProc("JetBeginSessionW")
	procJetAttachDatabaseW = esent.NewProc("JetAttachDatabaseW")
	procJetOpenDatabaseW   = esent.NewProc("JetOpenDatabaseW")
	procJetOpenTable       = esent.NewProc("JetOpenTable")
	procJetGetTableColInfo = esent.NewProc("JetGetTableColumnInfoW")
	procJetMove            = esent.NewProc("JetMove")
	procJetRetrieveColumn  = esent.NewProc("JetRetrieveColumn")
	procJetCloseTable      = esent.NewProc("JetCloseTable")
	procJetAttachDb        = esent.NewProc("JetAttachDatabaseW")
	procJetDetachDatabaseW = esent.NewProc("JetDetachDatabaseW")
	procJetEndSession      = esent.NewProc("JetEndSession")
	procJetTerm            = esent.NewProc("JetTerm2")
)

// jetErr converts a JET_API error code into an error.
func jetErr(code int32, what string) error {
	if code == 0 {
		return nil
	}
	return fmt.Errorf("%s: JET_err %d", what, code)
}

// jetSession bundles one recovered database copy and its cursor state.
type jetSession struct {
	inst, ses, db uintptr
	path          string
}

// openESECopy copies the WebCache database + logs to workDir, soft-recovers
// the copy via JetInit, and opens it read-only.
func openESECopy(dbPath, workDir string) (*jetSession, error) {
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return nil, err
	}
	dir := filepath.Dir(dbPath)
	base := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	copiedLogBase := ""
	entries, _ := os.ReadDir(dir)
	var copied []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".dat" && ext != ".log" && ext != ".jrs" && ext != ".chk" {
			continue
		}
		if !strings.HasPrefix(e.Name(), base) && ext == ".dat" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // live files held without share-read stay unreadable (VSS is out of v0.1 scope)
		}
		if err := os.WriteFile(filepath.Join(workDir, e.Name()), raw, 0o600); err != nil {
			continue
		}
		if ext == ".log" {
			lb := strings.TrimSuffix(e.Name(), ext)
			// <base>tmp.log is the temp-db log; real bases are <= 3 chars
			if !strings.HasSuffix(lb, "tmp") && len(lb) <= 3 && copiedLogBase == "" {
				copiedLogBase = lb
			}
		}
		copied = append(copied, e.Name())
	}
	if copied == nil {
		return nil, errors.New("webcache files not copyable (all locked)")
	}
	datCopied := false
	for _, c := range copied {
		if strings.HasSuffix(c, ".dat") {
			datCopied = true
		}
	}
	if !datCopied {
		return nil, errors.New("WebCacheV01.dat is held exclusively by the system; live copy requires VSS (out of v0.1 scope)")
	}
	return jetOpenRecovered(workDir, base, copiedLogBase, copiedLogBase != "")
}

// jetOpenRecovered runs the ESE pipeline: instance -> params -> init
// (recovery) -> session -> attach -> open.
func jetOpenRecovered(workDir, dbBase, logBase string, copiedLogs bool) (*jetSession, error) {
	var inst uintptr
	if err := jetErr(int32(callProc(procJetCreateInstanceW, &inst, wstr("ocm-audit"))), "JetCreateInstanceW"); err != nil {
		return nil, err
	}
	wWork := wstr(workDir)
	if logBase == "" {
		logBase = "edb"
	}
	setParam := func(pid uintptr, str *uint16) error {
		err := jetErr(int32(callProc(procJetSetSysParamW, &inst, 0, pid, 0, str)), fmt.Sprintf("JetSetSystemParameterW(%d)", pid))
		return err
	}
	if err := setParam(0, wWork); err != nil { // SystemPath (string slot)
		return nil, err
	}
	if err := setParam(1, wWork); err != nil { // TempPath
		return nil, err
	}
	if err := setParam(2, wWork); err != nil { // LogFilePath
		return nil, err
	}
	if err := setParam(3, wstr(logBase)); err != nil { // BaseName
		return nil, err
	}
	// soft recovery replays the copied logs; databases copied without logs
	// (e.g. WebCache under circular logging) must attach with recovery off
	recParam := wstr("on")
	if !copiedLogs {
		recParam = wstr("off")
	}
	if err := jetErr(int32(callProc(procJetSetSysParamW, &inst, 0, 32, 0, recParam)), "JetSetSystemParameterW(Recovery)"); err != nil {
		return nil, err
	}
	if err := jetErr(int32(callProc(procJetInit, &inst)), "JetInit"); err != nil {
		return nil, fmt.Errorf("recovery failed (missing/mismatched logs?): %w", err)
	}
	var ses uintptr
	if err := jetErr(int32(callProc(procJetBeginSessionW, inst, &ses, 0, 0)), "JetBeginSessionW"); err != nil {
		_ = callProc(procJetTerm, inst, 0)
		return nil, err
	}
	dbName := filepath.Join(workDir, dbBase+".dat")
	wDB := wstr(dbName)
	if err := jetErr(int32(callProc(procJetAttachDatabaseW, ses, wDB, jetDbReadOnly)), "JetAttachDatabaseW"); err != nil {
		_ = callProc(procJetTerm, inst, 0)
		return nil, err
	}
	var db uintptr
	if err := jetErr(int32(callProc(procJetOpenDatabaseW, ses, wDB, 0, &db, jetDbReadOnly)), "JetOpenDatabaseW"); err != nil {
		_ = callProc(procJetTerm, inst, 0)
		return nil, err
	}
	return &jetSession{inst: inst, ses: ses, db: db, path: dbName}, nil
}

// Close tears the session down (best effort).
func (j *jetSession) Close() {
	if j == nil {
		return
	}
	_ = jetErr(int32(callProc(procJetDetachDatabaseW, j.ses, 0, 0)), "detach")
	_ = jetErr(int32(callProc(procJetEndSession, j.ses, 0, 0)), "end session")
	_ = jetErr(int32(callProc(procJetTerm, j.inst, 0)), "term")
}

// callProc invokes a LazyProc with type-safe argument conversion. Pointer
// arguments are converted inside the Call expression and kept alive until
// the call returns (uintptr conversion at call sites would race the GC).
func callProc(p *windows.LazyProc, args ...any) uintptr {
	vals := make([]uintptr, len(args))
	keep := make([]any, len(args))
	for i, a := range args {
		keep[i] = a
		switch t := a.(type) {
		case uintptr:
			vals[i] = t
		case *uint16:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *byte:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *uintptr:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *uint32:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *int32:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *uint64:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *[4]byte:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *[8]byte:
			vals[i] = uintptr(unsafe.Pointer(t))
		case *jetColumnDef:
			vals[i] = uintptr(unsafe.Pointer(t))
		case int:
			vals[i] = uintptr(t)
		case int32:
			vals[i] = uintptr(t)
		case uint32:
			vals[i] = uintptr(t)
		case uint64:
			vals[i] = uintptr(t)
		default:
			panic(fmt.Sprintf("callProc: unsupported argument type %T", a))
		}
	}
	r1, _, _ := p.Call(vals...)
	runtime.KeepAlive(keep)
	return r1
}

// wstr encodes a UTF-16 NUL-terminated string pointer.
func wstr(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

// astr encodes an ANSI NUL-terminated string pointer for ESE name
// parameters (WebCache table/column names are ASCII).
func astr(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}

// jetColumnDef mirrors JET_COLUMNDEF (28 bytes on x64).
type jetColumnDef struct {
	CbStruct uint32
	Columnid uint32
	Coltyp   uint32
	Country  uint16
	Langid   uint16
	Cp       uint16
	Filler   uint16
	CbMax    uint32
	Grbit    uint32
}

// openTable opens a table read-only.
func (j *jetSession) openTable(name string) (uintptr, error) {
	var tid uintptr
	err := jetErr(int32(callProc(procJetOpenTable, j.ses, j.db,
		astr(name), 0, 0, jetTableReadOnly, &tid)), "JetOpenTable "+name)
	return tid, err
}

// columnDef resolves one column's definition (id/type/codepage).
func (j *jetSession) columnDef(tid uintptr, col string) (*jetColumnDef, error) {
	var def jetColumnDef
	def.CbStruct = uint32(unsafe.Sizeof(def))
	err := jetErr(int32(callProc(procJetGetTableColInfo, j.ses, tid,
		wstr(col), &def, def.CbStruct, jetColInfo)), "JetGetTableColumnInfoW "+col)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

// readColumn retrieves one column value as raw bytes (nil when NULL/empty).
func (j *jetSession) readColumn(tid uintptr, colid uint32) ([]byte, error) {
	var actual uint32
	r1 := callProc(procJetRetrieveColumn, j.ses, tid, colid, 0, 0, &actual, 0, 0)
	if int32(r1) < 0 {
		return nil, jetErr(int32(r1), "JetRetrieveColumn size")
	}
	if actual == 0 {
		return nil, nil
	}
	buf := make([]byte, actual)
	r1 = callProc(procJetRetrieveColumn, j.ses, tid, colid, &buf[0], actual, &actual, 0, 0)
	if int32(r1) < 0 {
		return nil, jetErr(int32(r1), "JetRetrieveColumn read")
	}
	return buf[:actual], nil
}

// iterate walks a table from the first row, decoding requested columns.
func (j *jetSession) iterate(tid uintptr, cols map[string]*jetColumnDef, max int, fn func(rec map[string]any) error) error {
	err := jetErr(int32(callProc(procJetMove, j.ses, tid, jetMoveFirst, 0)), "JetMove first")
	if err != nil {
		return err
	}
	for i := 0; i < max; i++ {
		rec := map[string]any{}
		for name, def := range cols {
			raw, err := j.readColumn(tid, def.Columnid)
			if err != nil {
				continue
			}
			rec[name] = decodeESEValue(raw, def)
		}
		if err := fn(rec); err != nil {
			return err
		}
		r1 := callProc(procJetMove, j.ses, tid, jetMoveNext, 0)
		if int32(r1) < 0 {
			break // -1603 no current record = end of table
		}
	}
	return nil
}

// decodeESEValue converts raw column bytes per the column definition.
func decodeESEValue(raw []byte, def *jetColumnDef) any {
	if raw == nil {
		return nil
	}
	switch def.Coltyp {
	case jetColtypText, jetColtypLongText:
		if def.Cp == jetCPUnicode {
			return utf16LE(raw)
		}
		return string(raw)
	case jetColtypDateTime:
		if len(raw) >= 8 {
			days := math.Float64frombits(binary.LittleEndian.Uint64(raw[:8]))
			return oleTime(days)
		}
		return nil
	case jetColtypBinary, jetColtypLongBinary:
		if len(raw) == 8 {
			ft := int64(binary.LittleEndian.Uint64(raw))
			if ft > 1e15 && ft < 2650467744000000000 { // plausible FILETIME range
				secs := ft/1e7 - 11644473600
				if secs > 0 {
					return time.Unix(secs, 0).UTC().Format(time.RFC3339)
				}
			}
		}
		return fmt.Sprintf("%x", raw)
	default:
		if len(raw) <= 8 {
			return int64(binary.LittleEndian.Uint64(pad64(raw)))
		}
		return fmt.Sprintf("%x", raw)
	}
}

func pad64(b []byte) []byte {
	out := make([]byte, 8)
	copy(out, b)
	return out
}

// readHistory walks the WebCache History table.
func (j *jetSession) readHistory(max int) ([]record, error) {
	tid, err := j.openTable("History")
	if err != nil {
		return nil, err
	}
	defer func() { _ = jetErr(int32(callProc(procJetCloseTable, j.ses, tid)), "close") }()
	var out []record
	cols := map[string]*jetColumnDef{}
	for _, name := range []string{"Url", "Title", "LastVisited", "UsageCount"} {
		if def, err := j.columnDef(tid, name); err == nil {
			cols[name] = def
		}
	}
	if err := j.iterate(tid, cols, max, func(rec map[string]any) error {
		url, _ := rec["Url"].(string)
		if url == "" {
			return nil
		}
		out = append(out, record{
			Source: "ese", Kind: "history",
			URL:    url,
			Title:  strOf(rec["Title"]),
			Time:   strOf(rec["LastVisited"]),
			Visits: intOf(rec["UsageCount"]),
		})
		return nil
	}); err != nil {
		return out, err
	}
	return out, nil
}

// readDownloads walks the Downloads table.
func (j *jetSession) readDownloads(max int) ([]record, error) {
	tid, err := j.openTable("Downloads")
	if err != nil {
		return nil, err
	}
	defer func() { _ = jetErr(int32(callProc(procJetCloseTable, j.ses, tid)), "close") }()
	var out []record
	cols := map[string]*jetColumnDef{}
	for _, name := range []string{"URL", "FileName", "Path", "Downloads"} {
		if def, err := j.columnDef(tid, name); err == nil {
			cols[name] = def
		}
	}
	if err := j.iterate(tid, cols, max, func(rec map[string]any) error {
		url, _ := rec["URL"].(string)
		if url == "" {
			return nil
		}
		out = append(out, record{
			Source: "ese", Kind: "download",
			URL:   url,
			Path:  strOf(rec["Path"]),
			Title: strOf(rec["FileName"]),
			Time:  strOf(rec["Downloads"]),
		})
		return nil
	}); err != nil {
		return out, err
	}
	return out, nil
}

// readCache walks the CacheEntries table.
func (j *jetSession) readCache(max int) ([]record, error) {
	tid, err := j.openTable("CacheEntries")
	if err != nil {
		return nil, err
	}
	defer func() { _ = jetErr(int32(callProc(procJetCloseTable, j.ses, tid)), "close") }()
	var out []record
	cols := map[string]*jetColumnDef{}
	for _, name := range []string{"Url", "FileSize", "LastModified"} {
		if def, err := j.columnDef(tid, name); err == nil {
			cols[name] = def
		}
	}
	if err := j.iterate(tid, cols, max, func(rec map[string]any) error {
		url, _ := rec["Url"].(string)
		if url == "" {
			return nil
		}
		out = append(out, record{Source: "ese", Kind: "cache",
			URL: url, Size: intOf(rec["FileSize"]), Time: strOf(rec["LastModified"])})
		return nil
	}); err != nil {
		return out, err
	}
	return out, nil
}
