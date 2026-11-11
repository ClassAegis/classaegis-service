package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOpenSQLiteFixture: the generated fixture exercises schema parsing,
// row walking and a >4KB overflow-chain URL reassembly.
func TestOpenSQLiteFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "chrome_history.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := openSQLite(raw)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	rows, err := db.selectRows("urls", 100)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	// overflow URL must be reassembled intact
	var longURL string
	for _, r := range rows {
		if u, _ := r["url"].(string); len(u) > 1000 {
			longURL = u
		}
	}
	wantLen := len("https://example.com/search?q=") + 9000 + len("&end=1")
	if len(longURL) != wantLen {
		t.Fatalf("overflow url length wrong: got %d want %d", len(longURL), wantLen)
	}
	if !strings.HasPrefix(longURL, "https://example.com/search?q=") || !strings.HasSuffix(longURL, "&end=1") {
		t.Fatalf("overflow url content wrong: %.40s...", longURL)
	}
	// schema mapping: id stored as NULL aliasing rowid
	if _, ok := rows[0]["url"].(string); !ok {
		t.Fatalf("schema mapping broken, row0=%v", rows[0])
	}
	us, _ := rows[0]["last_visit_time"].(int64)
	if got := chromiumTime(us); got != "2024-01-17T21:20:00Z" {
		t.Fatalf("chromium time conversion: %s", got)
	}
}

// TestChromiumTime: WebKit epoch conversion edge cases.
func TestChromiumTime(t *testing.T) {
	if got := chromiumTime(0); got != "" {
		t.Fatalf("zero input should stay empty, got %q", got)
	}
	if got := chromiumTime(13350000000000000); got != "2024-01-17T21:20:00Z" {
		t.Fatalf("got %q", got)
	}
}

// TestOleTime: OLE automation date conversion.
func TestOleTime(t *testing.T) {
	if got := oleTime(0); got != "" {
		t.Fatalf("zero should stay empty, got %q", got)
	}
	if got := oleTime(45000.5); got != "2023-03-15T12:00:00Z" {
		t.Fatalf("got %q", got)
	}
}

// TestUtf16LE: ESE unicode text decoding.
func TestUtf16LE(t *testing.T) {
	if got := utf16LE([]byte{'h', 0, 'i', 0}); got != "hi" {
		t.Fatalf("got %q", got)
	}
}

// TestStrIntHelpers: value coercion helpers.
func TestStrIntHelpers(t *testing.T) {
	if strOf(nil) != "" || strOf(" x ") != "x" {
		t.Fatal("strOf wrong")
	}
	if intOf(int64(5)) != 5 || intOf(nil) != 0 {
		t.Fatal("intOf wrong")
	}
}

// TestParseSourceLimit: defaults and caps.
func TestParseSourceLimit(t *testing.T) {
	e := New(Options{})
	src, limit := e.parseSourceLimit(map[string]any{})
	if src != "all" || limit != 200 {
		t.Fatalf("defaults wrong: %s %d", src, limit)
	}
	src, limit = e.parseSourceLimit(map[string]any{"source": "edge", "limit": json.Number("5000")})
	if src != "edge" || limit != 200 {
		t.Fatalf("cap should clamp to MaxRecords: %s %d", src, limit)
	}
}
