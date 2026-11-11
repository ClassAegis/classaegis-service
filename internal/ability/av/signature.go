package av

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// SignatureDB is the local signature database (JSON file). Rules:
//   - hashes: exact file SHA-256 matches
//   - patterns: byte patterns (hex-encoded in JSON) matched as a substring
//     of the first patternScanLimit bytes of a file
type SignatureDB struct {
	Version  int           `json:"version"`
	Updated  string        `json:"updated"`
	Source   string        `json:"source"`
	Hashes   []HashRule    `json:"hashes"`
	Patterns []PatternRule `json:"patterns"`

	// decoded at load time; not serialized
	patternBytes [][]byte `json:"-"`
}

// HashRule matches a file by SHA-256.
type HashRule struct {
	Sha256   string `json:"sha256"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Category string `json:"category"`
}

// PatternRule matches a byte pattern inside the scanned prefix of a file.
type PatternRule struct {
	Pattern  string `json:"pattern"` // hex-encoded bytes
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Category string `json:"category"`
}

const (
	maxRules         = 100000  // load guard
	patternScanLimit = 1 << 20 // 1MB prefix for pattern matching
)

// LoadSignatureDB reads and validates a signature DB file. Invalid entries
// are dropped with a warning; structural problems are errors.
func LoadSignatureDB(path string) (*SignatureDB, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	db, err := ParseSignatureDB(raw)
	if err != nil {
		return nil, err
	}
	db.patternBytes = decodePatterns(db.Patterns)
	return db, nil
}

// ParseSignatureDB validates JSON bytes into a SignatureDB.
func ParseSignatureDB(raw []byte) (*SignatureDB, error) {
	var db SignatureDB
	if err := json.Unmarshal(raw, &db); err != nil {
		return nil, fmt.Errorf("invalid signature db JSON: %w", err)
	}
	if db.Version < 1 {
		return nil, errors.New("invalid signature db: version must be >= 1")
	}
	if len(db.Hashes)+len(db.Patterns) > maxRules {
		return nil, errors.New("invalid signature db: too many rules")
	}
	hashRe := regexp.MustCompile(`^[0-9a-f]{64}$`)
	kept := db.Hashes[:0]
	for _, h := range db.Hashes {
		if !hashRe.MatchString(strings.ToLower(h.Sha256)) {
			log.Printf("[av] dropping hash rule with bad sha256: %q", h.Name)
			continue
		}
		kept = append(kept, HashRule{
			Sha256:   strings.ToLower(h.Sha256),
			Name:     h.Name,
			Severity: normalizeSeverity(h.Severity),
			Category: h.Category,
		})
	}
	db.Hashes = kept
	return &db, nil
}

func decodePatterns(rules []PatternRule) [][]byte {
	var out [][]byte
	for _, p := range rules {
		b, err := hex.DecodeString(p.Pattern)
		if err != nil || len(b) == 0 {
			log.Printf("[av] dropping pattern rule with bad hex: %q", p.Name)
			continue
		}
		out = append(out, b)
	}
	return out
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(s) {
	case "high", "medium", "low":
		return strings.ToLower(s)
	}
	return "medium"
}

// EmptySignatureDB returns an empty in-memory DB (heuristics still work).
func EmptySignatureDB() *SignatureDB {
	return &SignatureDB{Version: 0}
}

// Save writes the DB back to path atomically (temp file + rename).
func (db *SignatureDB) Save(path string) error {
	raw, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LookupHash returns the matching hash rule, if any.
func (db *SignatureDB) LookupHash(sha256Hex string) (HashRule, bool) {
	for _, h := range db.Hashes {
		if h.Sha256 == sha256Hex {
			return h, true
		}
	}
	return HashRule{}, false
}

// LookupPattern returns the matching pattern rule, if any.
func (db *SignatureDB) LookupPattern(content []byte) (PatternRule, bool) {
	for i, p := range db.patternBytes {
		if bytes.Contains(content, p) {
			return db.Patterns[i], true
		}
	}
	return PatternRule{}, false
}
