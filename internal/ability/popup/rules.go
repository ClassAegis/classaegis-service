// Package popup implements the popup/ad blocking ability (弹窗/广告拦截):
// top-level window interception driven by process/title/class rules, an
// event buffer for cloud aggregation, and rule updates from the server.
// The process is loopback-only; cloud collaboration is pull-based events +
// server-pushed rules via core.
package popup

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

// RuleSet is the popup interception rule set (JSON file / inline payload).
type RuleSet struct {
	Version   int    `json:"version"`
	Updated   string `json:"updated"`
	Source    string `json:"source"`
	Block     []Rule `json:"block"`
	Whitelist []Rule `json:"whitelist"`
}

// Rule matches a window on up to three dimensions; an empty dimension is a
// wildcard. At least one dimension must be set.
type Rule struct {
	Process string `json:"process,omitempty"` // executable name, e.g. SomeAd.exe
	Title   string `json:"title,omitempty"`   // window title glob, * and ?
	Class   string `json:"class,omitempty"`   // window class glob
	Action  string `json:"action,omitempty"`  // close (default) | hide
	Note    string `json:"note,omitempty"`    // human remark
}

const maxRules = 20000

// ParseRuleSet validates JSON bytes into a RuleSet. Malformed single rules
// are dropped with a warning; structural problems are errors.
func ParseRuleSet(raw []byte) (*RuleSet, error) {
	var rs RuleSet
	if err := json.Unmarshal(raw, &rs); err != nil {
		return nil, fmt.Errorf("invalid rule set JSON: %w", err)
	}
	if rs.Version < 1 {
		return nil, errors.New("invalid rule set: version must be >= 1")
	}
	if len(rs.Block)+len(rs.Whitelist) > maxRules {
		return nil, errors.New("invalid rule set: too many rules")
	}
	rs.Block = validRules(rs.Block, "block")
	rs.Whitelist = validRules(rs.Whitelist, "whitelist")
	return &rs, nil
}

func validRules(in []Rule, kind string) []Rule {
	kept := in[:0]
	for _, r := range in {
		if strings.TrimSpace(r.Process) == "" && strings.TrimSpace(r.Title) == "" && strings.TrimSpace(r.Class) == "" {
			log.Printf("[popup] dropping empty %s rule (%s)", kind, r.Note)
			continue
		}
		if r.Action != "" && r.Action != "close" && r.Action != "hide" {
			log.Printf("[popup] dropping %s rule with bad action %q (%s)", kind, r.Action, r.Note)
			continue
		}
		kept = append(kept, r)
	}
	return kept
}

// LoadRuleSet reads and validates a rule set file.
func LoadRuleSet(path string) (*RuleSet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseRuleSet(raw)
}

// Save writes the rule set atomically (temp file + rename).
func (rs *RuleSet) Save(path string) error {
	raw, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// EmptyRuleSet returns an empty in-memory rule set.
func EmptyRuleSet() *RuleSet {
	return &RuleSet{Version: 0}
}

// MatchBlock returns the first matching block rule, if any. Whitelist is
// checked by MatchWhitelist (it wins over block at the call site).
func (rs *RuleSet) MatchBlock(w WindowInfo) (Rule, bool) {
	return matchRules(rs.Block, w)
}

// MatchWhitelist reports whether any whitelist rule matches the window.
func (rs *RuleSet) MatchWhitelist(w WindowInfo) bool {
	_, ok := matchRules(rs.Whitelist, w)
	return ok
}

func matchRules(rules []Rule, w WindowInfo) (Rule, bool) {
	proc := strings.ToLower(w.Process)
	title := strings.ToLower(w.Title)
	cls := strings.ToLower(w.Class)
	for _, r := range rules {
		if ruleHit(r, proc, title, cls) {
			return r, true
		}
	}
	return Rule{}, false
}

func ruleHit(r Rule, proc, title, cls string) bool {
	if r.Process != "" && !globMatch(strings.ToLower(r.Process), proc) {
		return false
	}
	if r.Title != "" && !globMatch(strings.ToLower(r.Title), title) {
		return false
	}
	if r.Class != "" && !globMatch(strings.ToLower(r.Class), cls) {
		return false
	}
	return true
}

// globMatch is a case-insensitive wildcard matcher supporting * and ?.
func globMatch(pattern, s string) bool {
	p, t := []rune(strings.ToLower(pattern)), []rune(strings.ToLower(s))
	star, pIdx, tIdx, match := -1, 0, 0, 0
	for tIdx < len(t) {
		if pIdx < len(p) && (p[pIdx] == '?' || p[pIdx] == t[tIdx]) {
			pIdx++
			tIdx++
		} else if pIdx < len(p) && p[pIdx] == '*' {
			star = pIdx
			match = tIdx
			pIdx++
		} else if star >= 0 {
			pIdx = star + 1
			match++
			tIdx = match
		} else {
			return false
		}
	}
	for pIdx < len(p) && p[pIdx] == '*' {
		pIdx++
	}
	return pIdx == len(p)
}
