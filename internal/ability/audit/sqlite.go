// SPDX-License-Identifier: AGPL-3.0-only
package audit

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// Minimal read-only SQLite file scanner: header check, table b-tree walk,
// record decoding, overflow chains. Self-contained on purpose (no cgo, no
// SQL engine) — we only need specific known tables from browser History
// files. Write access, indexes, WAL and freelist are out of scope.

var (
	errNotSQLite      = errors.New("not a sqlite database")
	errUnsupportedEnc = errors.New("sqlite text encoding not UTF-8")
	errPageOutOfRange = errors.New("sqlite page out of range")
)

type sqliteFile struct {
	data     []byte
	pageSize int
	usable   int
}

func openSQLite(data []byte) (*sqliteFile, error) {
	if len(data) < 100 || string(data[:16]) != "SQLite format 3\x00" {
		return nil, errNotSQLite
	}
	ps := int(binary.BigEndian.Uint16(data[16:18]))
	if ps == 1 {
		ps = 65536
	}
	if ps < 512 || ps&(ps-1) != 0 {
		return nil, fmt.Errorf("%w: bad page size %d", errNotSQLite, ps)
	}
	reserved := int(data[20])
	if reserved >= ps {
		return nil, fmt.Errorf("%w: bad reserved bytes", errNotSQLite)
	}
	enc := binary.BigEndian.Uint32(data[56:60])
	if enc != 1 {
		return nil, errUnsupportedEnc
	}
	return &sqliteFile{data: data, pageSize: ps, usable: ps - reserved}, nil
}

func (s *sqliteFile) page(n uint32) ([]byte, error) {
	if n < 1 || int64(n)*int64(s.pageSize) > int64(len(s.data)) {
		return nil, errPageOutOfRange
	}
	off := int64(n-1) * int64(s.pageSize)
	return s.data[off : off+int64(s.pageSize)], nil
}

// varint decodes a SQLite varint; returns value and bytes consumed.
func sqliteVarint(b []byte) (int64, int) {
	var v uint64
	for i := 0; i < 8; i++ {
		if i >= len(b) {
			return int64(v), i
		}
		v = v<<7 | uint64(b[i]&0x7f)
		if b[i]&0x80 == 0 {
			return int64(v), i + 1
		}
	}
	if len(b) < 9 {
		return int64(v), len(b)
	}
	v = v<<8 | uint64(b[8])
	return int64(v), 9
}

// serialLen returns the on-disk byte length of a serial type value.
func serialLen(t int64) int {
	switch {
	case t == 0 || t == 8 || t == 9:
		return 0
	case t >= 1 && t <= 4:
		return int(t)
	case t == 5:
		return 6
	case t == 6 || t == 7:
		return 8
	case t >= 12:
		return int((t - 12) / 2)
	}
	return 0
}

// decodeRecord decodes one record payload into Go values (text as string,
// ints as int64, float as float64, blob as []byte, null as nil).
func decodeRecord(payload []byte) []any {
	hdrLen, n := sqliteVarint(payload)
	if hdrLen <= 0 || int(hdrLen) > len(payload) {
		return nil
	}
	var types []int64
	pos := n
	for pos < int(hdrLen) {
		t, m := sqliteVarint(payload[pos:])
		if m == 0 {
			break
		}
		types = append(types, t)
		pos += m
	}
	body := payload[hdrLen:]
	out := make([]any, 0, len(types))
	var cursor int
	for _, t := range types {
		l := serialLen(t)
		if cursor+l > len(body) {
			break
		}
		chunk := body[cursor : cursor+l]
		cursor += l
		switch {
		case t == 0:
			out = append(out, nil)
		case t >= 1 && t <= 6:
			out = append(out, signedBE(chunk))
		case t == 7:
			out = append(out, math.Float64frombits(binary.BigEndian.Uint64(chunk)))
		case t == 8:
			out = append(out, int64(0))
		case t == 9:
			out = append(out, int64(1))
		case t >= 12 && t%2 == 0:
			b := make([]byte, len(chunk))
			copy(b, chunk)
			out = append(out, b)
		default:
			out = append(out, string(chunk))
		}
	}
	return out
}

// signedBE decodes big-endian two's-complement integers of 1/2/3/4/6/8 bytes.
func signedBE(b []byte) int64 {
	if len(b) == 0 {
		return 0
	}
	var v int64
	if b[0]&0x80 != 0 {
		v = -1
	}
	for _, c := range b {
		v = v<<8 | int64(c)
	}
	return v
}

// overflowChain follows the overflow page list, appending payloads.
func (s *sqliteFile) overflowChain(first uint32, need int) []byte {
	out := make([]byte, 0, need)
	next := first
	seen := map[uint32]bool{}
	for next != 0 && len(out) < need && !seen[next] {
		seen[next] = true
		pg, err := s.page(next)
		if err != nil {
			break
		}
		nextPage := binary.BigEndian.Uint32(pg[0:4])
		chunk := pg[4:int(s.usable)]
		take := need - len(out)
		if take > len(chunk) {
			take = len(chunk)
		}
		out = append(out, chunk[:take]...)
		next = nextPage
	}
	return out
}

// tablePayload returns the full payload bytes of a table-leaf cell,
// reassembling overflow pages when the payload spills.
func (s *sqliteFile) tableCellPayload(pg []byte, cellOff int, hdrSize int) ([]byte, int64, error) {
	p := pg[cellOff:hdrSize]
	payloadLen, n1 := sqliteVarint(p)
	rowid, n2 := sqliteVarint(p[n1:])
	body := p[n1+n2:]
	maxLocal := s.usable - 35
	if int(payloadLen) <= maxLocal {
		if int(payloadLen) > len(body) {
			return nil, 0, fmt.Errorf("cell payload exceeds page")
		}
		return body[:payloadLen], rowid, nil
	}
	minLocal := ((s.usable-12)*32)/255 - 23
	local := minLocal + (int(payloadLen)-minLocal)%(s.usable-4)
	if local > maxLocal {
		local = minLocal
	}
	if local+4 > len(body) {
		return nil, 0, fmt.Errorf("overflow cell exceeds page")
	}
	first := binary.BigEndian.Uint32(body[local : local+4])
	full := make([]byte, 0, payloadLen)
	full = append(full, body[:local]...)
	full = append(full, s.overflowChain(first, int(payloadLen)-local)...)
	return full, rowid, nil
}

// walkTable visits every (rowid, record) row of a table b-tree.
func (s *sqliteFile) walkTable(root uint32, fn func(rowid int64, rec []any) error) error {
	seen := map[uint32]bool{}
	var walk func(pgno uint32) error
	walk = func(pgno uint32) error {
		if seen[pgno] {
			return nil
		}
		seen[pgno] = true
		pg, err := s.page(pgno)
		if err != nil {
			return err
		}
		hdr := 0
		if pgno == 1 {
			hdr = 100 // file header occupies page 1
		}
		typ := pg[hdr]
		cellCount := int(binary.BigEndian.Uint16(pg[hdr+3 : hdr+5]))
		switch typ {
		case 0x0d: // leaf table
			cellPtrStart := hdr + 8
			for i := 0; i < cellCount; i++ {
				off := int(binary.BigEndian.Uint16(pg[cellPtrStart+2*i : cellPtrStart+2*i+2]))
				if off < hdr+8 || off >= s.usable {
					continue
				}
				payload, rowid, err := s.tableCellPayload(pg, off, s.usable)
				if err != nil {
					continue
				}
				if err := fn(rowid, decodeRecord(payload)); err != nil {
					return err
				}
			}
		case 0x05: // interior table
			cellPtrStart := hdr + 12
			for i := 0; i < cellCount; i++ {
				off := int(binary.BigEndian.Uint16(pg[cellPtrStart+2*i : cellPtrStart+2*i+2]))
				if off < hdr+12 || off >= s.usable {
					continue
				}
				child := binary.BigEndian.Uint32(pg[off : off+4])
				if err := walk(child); err != nil {
					return err
				}
			}
			right := binary.BigEndian.Uint32(pg[hdr+8 : hdr+12])
			return walk(right)
		default:
			return fmt.Errorf("unexpected b-tree page type 0x%02x on page %d", typ, pgno)
		}
		return nil
	}
	return walk(root)
}

// tableRootAndCols finds a table's root page and its column names (parsed
// from the CREATE TABLE statement stored in sqlite_master).
func (s *sqliteFile) tableRootAndCols(table string) (uint32, []string, error) {
	var found uint32
	var cols []string
	err := s.walkTable(1, func(rowid int64, rec []any) error {
		if len(rec) >= 5 {
			if t, _ := rec[0].(string); t == "table" {
				if n, _ := rec[1].(string); strings.EqualFold(n, table) {
					switch v := rec[3].(type) {
					case int64:
						found = uint32(v)
					}
					if sql, _ := rec[4].(string); sql != "" {
						cols = parseCreateColumns(sql)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	if found == 0 {
		return 0, nil, fmt.Errorf("table %q not found", table)
	}
	return found, cols, nil
}

// parseCreateColumns extracts column names from a CREATE TABLE statement.
// Constraints (PRIMARY/UNIQUE/CHECK/FOREIGN/CONSTRAINT clauses) are skipped.
func parseCreateColumns(sql string) []string {
	open := strings.Index(sql, "(")
	if open < 0 {
		return nil
	}
	close_ := strings.LastIndex(sql, ")")
	if close_ <= open {
		return nil
	}
	inner := sql[open+1 : close_]
	var cols []string
	depth := 0
	cur := strings.Builder{}
	var parts []string
	for _, r := range inner {
		switch {
		case r == '(':
			depth++
			cur.WriteRune(r)
		case r == ')':
			depth--
			cur.WriteRune(r)
		case r == ',' && depth == 0:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		parts = append(parts, cur.String())
	}
	constraintKeywords := map[string]bool{
		"primary": true, "unique": true, "check": true, "foreign": true, "constraint": true,
	}
	for _, part := range parts {
		f := strings.Fields(strings.TrimSpace(part))
		if len(f) == 0 {
			continue
		}
		first := strings.Trim(f[0], "`\"'[]")
		if constraintKeywords[strings.ToLower(first)] {
			continue
		}
		cols = append(cols, first)
	}
	return cols
}

// selectRows walks a table and returns rows as maps keyed by schema column
// names (order from CREATE TABLE), capped at limit rows. Values keep their
// decoded Go types.
func (s *sqliteFile) selectRows(table string, limit int) ([]map[string]any, error) {
	root, cols, err := s.tableRootAndCols(table)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	err = s.walkTable(root, func(rowid int64, rec []any) error {
		if len(rows) >= limit {
			return errStopWalk
		}
		m := map[string]any{"rowid": rowid}
		for i, c := range cols {
			if i < len(rec) {
				m[c] = rec[i]
			}
		}
		rows = append(rows, m)
		return nil
	})
	if err != nil && err != errStopWalk {
		return nil, err
	}
	return rows, nil
}

var errStopWalk = errors.New("stop walk")

// chromiumTime converts a WebKit timestamp (microseconds since 1601-01-01)
// to RFC3339; zero input stays zero.
func chromiumTime(us int64) string {
	if us == 0 {
		return ""
	}
	secs := us/1e6 - 11644473600
	if secs < 0 {
		return ""
	}
	return time.Unix(secs, 0).UTC().Format(time.RFC3339)
}

// utf16LE decodes UTF-16LE bytes (ESE Unicode text columns).
func utf16LE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	r := utf16.Decode(u)
	for i, c := range r {
		if c == utf8.RuneError {
			r = r[:i]
			break
		}
	}
	return string(r)
}

// oleTime converts an OLE automation date (days since 1899-12-30) to
// RFC3339; ESE DateTime columns store this double.
func oleTime(days float64) string {
	if days <= 0 {
		return ""
	}
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	t := base.Add(time.Duration(days * float64(24*time.Hour)))
	return t.UTC().Format(time.RFC3339)
}
