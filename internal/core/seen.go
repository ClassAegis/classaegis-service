// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// SeenSet persists processed msgIds for idempotency across restarts.
// Stored as JSON array in %ProgramData%\oneclassmesh\seen.json, capped.
type SeenSet struct {
	mu   sync.Mutex
	path string
	max  int
	ids  map[string]bool
}

func NewSeenSet(maxEntries int) (*SeenSet, error) {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = "."
	}
	dir := filepath.Join(base, "oneclassmesh")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &SeenSet{
		path: filepath.Join(dir, "seen.json"),
		max:  maxEntries,
		ids:  make(map[string]bool),
	}
	data, err := os.ReadFile(s.path)
	if err == nil {
		var list []string
		if json.Unmarshal(data, &list) == nil {
			for _, id := range list {
				s.ids[id] = true
			}
		}
	}
	return s, nil
}

// Add returns true if the id is new (first time seen).
func (s *SeenSet) Add(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids[id] {
		return false
	}
	s.ids[id] = true
	if len(s.ids) > s.max {
		// drop oldest entries: map has no order, so rebuild keeping last max.
		// simple strategy: if oversized, keep only this new id and log.
		s.ids = map[string]bool{id: true}
		log.Printf("seen set overflow, reset to latest: %s", id)
	}
	s.saveLocked()
	return true
}

// Remove drops an id (used on FAILED so a retry may re-execute).
func (s *SeenSet) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ids, id)
	s.saveLocked()
}

func (s *SeenSet) saveLocked() {
	list := make([]string, 0, len(s.ids))
	for id := range s.ids {
		list = append(list, id)
	}
	data, err := json.Marshal(list)
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0o644)
}
