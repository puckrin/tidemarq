// Package filter implements the §3.3 file-filtering rules a sync job can
// define to include or exclude files from a run. Rules are evaluated in order;
// the first matching rule wins. With no matching rule the file is included.
// Hidden files (any path segment beginning with '.') are excluded as a final
// step when ExcludeHidden is set and no earlier rule decided the file.
package filter

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// Action is the verdict a matching rule applies to a file.
type Action string

const (
	Include Action = "include"
	Exclude Action = "exclude"
)

// RuleType identifies which predicate fields on Rule are meaningful.
type RuleType string

const (
	TypeGlob      RuleType = "glob"      // Pattern: doublestar glob (supports **)
	TypeExtension RuleType = "extension" // Pattern: ".log" or "log"
	TypeSize      RuleType = "size"      // MinSize/MaxSize in bytes (inclusive)
	TypeModified  RuleType = "modified"  // OlderThanDays / NewerThanDays
)

// Rule is one entry in a Ruleset. The Type field selects which Pattern/Size/
// Modified fields are interpreted; the others are ignored.
//
// Size and modified predicates are framed as "matches files outside the
// allowed window" so the spec example reads naturally:
//
//   exclude files larger than 4 GB
//       → {Type: size, Action: exclude, SizeAboveBytes: 4*GiB}
//   only include files modified in the last 30 days
//       → {Type: modified, Action: exclude, ModifiedBeforeDaysAgo: 30}
type Rule struct {
	Type   RuleType `json:"type"`
	Action Action   `json:"action"`

	// Glob, Extension.
	Pattern string `json:"pattern,omitempty"`

	// Size rules. Each field is independent; at least one must be > 0.
	// SizeAboveBytes: rule matches files larger than this ("too big").
	// SizeBelowBytes: rule matches files smaller than this ("too small").
	// Both set: rule matches files outside the [Below, Above] window.
	SizeAboveBytes int64 `json:"size_above_bytes,omitempty"`
	SizeBelowBytes int64 `json:"size_below_bytes,omitempty"`

	// Modified rules. Each field is independent; at least one must be > 0.
	// ModifiedBeforeDaysAgo: rule matches files older than N days ("too old").
	// ModifiedWithinDays:    rule matches files newer than N days ("too fresh").
	ModifiedBeforeDaysAgo int `json:"modified_before_days_ago,omitempty"`
	ModifiedWithinDays    int `json:"modified_within_days,omitempty"`
}

// Ruleset is the set of filters attached to a job.
//
// The zero value (no rules, ExcludeHidden=false) includes every file, matching
// the pre-feature behaviour. New jobs created via the wizard default to
// ExcludeHidden=true; legacy jobs persisted with an empty filters_json blob
// keep their previous behaviour automatically.
type Ruleset struct {
	ExcludeHidden bool   `json:"exclude_hidden"`
	Rules         []Rule `json:"rules,omitempty"`
}

// Validate returns an error if any rule is structurally invalid. Callers
// should run this at API boundaries (e.g. job create/update) so the engine
// never encounters a malformed rule mid-run.
func (rs *Ruleset) Validate() error {
	if rs == nil {
		return nil
	}
	for i, r := range rs.Rules {
		if err := r.validate(); err != nil {
			return fmt.Errorf("rule %d: %w", i, err)
		}
	}
	return nil
}

func (r Rule) validate() error {
	switch r.Action {
	case Include, Exclude:
	default:
		return fmt.Errorf("invalid action %q", r.Action)
	}
	switch r.Type {
	case TypeGlob:
		if r.Pattern == "" {
			return fmt.Errorf("glob rule requires pattern")
		}
		if !doublestar.ValidatePattern(r.Pattern) {
			return fmt.Errorf("invalid glob pattern %q", r.Pattern)
		}
	case TypeExtension:
		if r.Pattern == "" {
			return fmt.Errorf("extension rule requires pattern")
		}
	case TypeSize:
		if r.SizeAboveBytes == 0 && r.SizeBelowBytes == 0 {
			return fmt.Errorf("size rule requires size_above_bytes or size_below_bytes")
		}
		if r.SizeAboveBytes < 0 || r.SizeBelowBytes < 0 {
			return fmt.Errorf("size bounds must be non-negative")
		}
		if r.SizeAboveBytes > 0 && r.SizeBelowBytes > 0 && r.SizeBelowBytes > r.SizeAboveBytes {
			return fmt.Errorf("size_below_bytes %d exceeds size_above_bytes %d (window is empty)", r.SizeBelowBytes, r.SizeAboveBytes)
		}
	case TypeModified:
		if r.ModifiedBeforeDaysAgo == 0 && r.ModifiedWithinDays == 0 {
			return fmt.Errorf("modified rule requires modified_before_days_ago or modified_within_days")
		}
		if r.ModifiedBeforeDaysAgo < 0 || r.ModifiedWithinDays < 0 {
			return fmt.Errorf("days must be non-negative")
		}
	default:
		return fmt.Errorf("unknown rule type %q", r.Type)
	}
	return nil
}

// Decide returns the final action for a file. relPath is forward-slash
// separated and rooted at the job's source directory. size and modTime are
// the file's current metadata.
//
// Order of evaluation:
//  1. Walk the rules top-to-bottom; first match wins.
//  2. If no rule matched and ExcludeHidden is set, exclude the file if any
//     path segment begins with '.'.
//  3. Otherwise, include.
//
// The hidden check runs after the rules, not before, so a user can write an
// explicit `include glob .env` rule to override ExcludeHidden for a specific
// file.
func (rs *Ruleset) Decide(relPath string, size int64, modTime time.Time) Action {
	return rs.decideAt(relPath, size, modTime, time.Now())
}

// decideAt is the testable form of Decide with an injected "now" so unit
// tests can pin the clock for OlderThanDays/NewerThanDays predicates.
func (rs *Ruleset) decideAt(relPath string, size int64, modTime, now time.Time) Action {
	if rs == nil {
		return Include
	}
	for _, r := range rs.Rules {
		if r.matches(relPath, size, modTime, now) {
			return r.Action
		}
	}
	if rs.ExcludeHidden && hasHiddenSegment(relPath) {
		return Exclude
	}
	return Include
}

func (r Rule) matches(relPath string, size int64, modTime, now time.Time) bool {
	switch r.Type {
	case TypeGlob:
		// Match (not PathMatch) — relPath is always forward-slash, regardless
		// of host OS, and PathMatch on Windows would interpret `/` literally.
		ok, err := doublestar.Match(r.Pattern, relPath)
		return err == nil && ok
	case TypeExtension:
		want := strings.ToLower(strings.TrimPrefix(r.Pattern, "."))
		got := strings.ToLower(strings.TrimPrefix(path.Ext(relPath), "."))
		return want != "" && want == got
	case TypeSize:
		if r.SizeAboveBytes > 0 && size > r.SizeAboveBytes {
			return true
		}
		if r.SizeBelowBytes > 0 && size < r.SizeBelowBytes {
			return true
		}
		return false
	case TypeModified:
		age := now.Sub(modTime)
		if r.ModifiedBeforeDaysAgo > 0 && age > time.Duration(r.ModifiedBeforeDaysAgo)*24*time.Hour {
			return true
		}
		if r.ModifiedWithinDays > 0 && age < time.Duration(r.ModifiedWithinDays)*24*time.Hour {
			return true
		}
		return false
	}
	return false
}

func hasHiddenSegment(relPath string) bool {
	for _, seg := range strings.Split(relPath, "/") {
		if strings.HasPrefix(seg, ".") && seg != "." && seg != ".." {
			return true
		}
	}
	return false
}
