package av

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrQuarantineNotFound / ErrTargetExists are quarantine errors surfaced in
// receipts as E_QUAR_NOT_FOUND / E_TARGET_EXISTS.
var (
	ErrQuarantineNotFound = errors.New("quarantine item not found")
	ErrTargetExists       = errors.New("restore target already exists")
)

// Quarantine is the local quarantine store: detected files are moved out of
// their original location and renamed <id>.quar, with an <id>.json sidecar
// recording the original path and detection metadata.
type Quarantine struct {
	dir string
	mu  sync.Mutex
}

// QuarantineItem is one entry of the quarantine store (list result).
type QuarantineItem struct {
	ID            string `json:"id"`
	QuarantinedAt string `json:"quarantinedAt"`
	OriginalPath  string `json:"originalPath"`
	Sha256        string `json:"sha256"`
	Rule          string `json:"rule"`
	Severity      string `json:"severity"`
	Category      string `json:"category"`
	SizeBytes     int64  `json:"sizeBytes"`
}

// NewQuarantine prepares the store directory (created lazily on first use).
func NewQuarantine(dir string) *Quarantine {
	return &Quarantine{dir: dir}
}

// Dir returns the store directory path.
func (q *Quarantine) Dir() string { return q.dir }

// Quarantine moves src into the store and records metadata; returns the
// stored item with its id.
func (q *Quarantine) Quarantine(src, sha256Hex, rule, severity, category string) (*QuarantineItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := os.MkdirAll(q.dir, 0o700); err != nil {
		return nil, err
	}
	id := newQuarantineID()
	dst := filepath.Join(q.dir, id+".quar")
	if err := moveFile(src, dst); err != nil {
		return nil, err
	}
	size := int64(0)
	if st, err := os.Stat(dst); err == nil {
		size = st.Size()
	}
	item := &QuarantineItem{
		ID:            id,
		QuarantinedAt: time.Now().UTC().Format(time.RFC3339),
		OriginalPath:  src,
		Sha256:        sha256Hex,
		Rule:          rule,
		Severity:      severity,
		Category:      category,
		SizeBytes:     size,
	}
	meta, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(q.dir, id+".json"), meta, 0o600); err != nil {
		return nil, err
	}
	log.Printf("[av] quarantined %s -> %s (rule=%s)", src, dst, rule)
	return item, nil
}

// List returns all items; unreadable sidecars are skipped silently.
func (q *Quarantine) List() ([]QuarantineItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var items []QuarantineItem
	for _, ent := range entries {
		if !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(q.dir, ent.Name()))
		if err != nil {
			continue
		}
		var it QuarantineItem
		if json.Unmarshal(raw, &it) == nil && it.ID != "" {
			items = append(items, it)
		}
	}
	return items, nil
}

// Restore moves a quarantined file back to its recorded original path.
func (q *Quarantine) Restore(id string) (*QuarantineItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	it, err := q.get(id)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(it.OriginalPath), 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(it.OriginalPath); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrTargetExists, it.OriginalPath)
	}
	if err := moveFile(q.fileOf(it), it.OriginalPath); err != nil {
		return nil, err
	}
	if err := os.Remove(q.metaOf(it)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	log.Printf("[av] restored %s -> %s", it.ID, it.OriginalPath)
	return it, nil
}

// Delete removes a quarantined file and its metadata permanently.
func (q *Quarantine) Delete(id string) (*QuarantineItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	it, err := q.get(id)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(q.fileOf(it)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err := os.Remove(q.metaOf(it)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	log.Printf("[av] deleted quarantine item %s", it.ID)
	return it, nil
}

// get loads one sidecar meta. The id is server-supplied, so it is strictly
// validated ([0-9A-Za-z-]) to prevent path traversal.
func (q *Quarantine) get(id string) (*QuarantineItem, error) {
	if !validID(id) {
		return nil, ErrQuarantineNotFound
	}
	raw, err := os.ReadFile(q.metaPath(id))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrQuarantineNotFound
		}
		return nil, err
	}
	var it QuarantineItem
	if err := json.Unmarshal(raw, &it); err != nil || it.ID == "" {
		return nil, ErrQuarantineNotFound
	}
	return &it, nil
}

func (q *Quarantine) metaPath(id string) string {
	return filepath.Join(q.dir, id+".json")
}

func (q *Quarantine) fileOf(it *QuarantineItem) string {
	return filepath.Join(q.dir, it.ID+".quar")
}

func (q *Quarantine) metaOf(it *QuarantineItem) string {
	return filepath.Join(q.dir, it.ID+".json")
}

// validID allows only [0-9A-Za-z-] ids.
func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') || r == '-'
		if !ok {
			return false
		}
	}
	return true
}

// newQuarantineID: UTC timestamp + 4 random bytes (hex), filesystem-safe.
func newQuarantineID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b)
}

// moveFile renames when possible, falls back to copy+remove across volumes.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return os.Remove(src)
}
