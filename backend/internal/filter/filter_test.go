package filter_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/filter"
)

// fixedNow is used as the modTime value for tests that don't exercise the
// modified-date predicate; its exact value doesn't matter for those tests.
// Tests for the modified-date predicate use daysAgo() instead so they remain
// correct when run against the real wall clock that rs.Decide queries.
var fixedNow = time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)

func daysAgo(n int) time.Time { return time.Now().AddDate(0, 0, -n) }

func decide(rs *filter.Ruleset, relPath string, size int64, modTime time.Time) filter.Action {
	return rs.Decide(relPath, size, modTime)
}

func TestDecide_NoRules_IncludesEverything(t *testing.T) {
	rs := &filter.Ruleset{}
	if got := decide(rs, "anything.txt", 1024, fixedNow); got != filter.Include {
		t.Errorf("got %v, want Include", got)
	}
}

func TestDecide_NilRuleset_IncludesEverything(t *testing.T) {
	var rs *filter.Ruleset
	if got := rs.Decide("anything.txt", 1024, fixedNow); got != filter.Include {
		t.Errorf("nil Ruleset must include, got %v", got)
	}
}

func TestDecide_Glob_Excludes(t *testing.T) {
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "**/node_modules/**"}},
	}
	cases := map[string]filter.Action{
		"src/index.ts":                       filter.Include,
		"node_modules/foo/index.js":          filter.Exclude,
		"apps/web/node_modules/bar/baz.json": filter.Exclude,
	}
	for path, want := range cases {
		if got := decide(rs, path, 100, fixedNow); got != want {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
}

func TestDecide_Extension_CaseInsensitive(t *testing.T) {
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeExtension, Action: filter.Exclude, Pattern: ".log"}},
	}
	// Matching with and without the leading dot in the pattern; case ignored on extension.
	for _, path := range []string{"app.log", "app.LOG", "deep/dir/app.Log"} {
		if got := decide(rs, path, 1, fixedNow); got != filter.Exclude {
			t.Errorf("%s: got %v, want Exclude", path, got)
		}
	}
	if got := decide(rs, "app.txt", 1, fixedNow); got != filter.Include {
		t.Errorf("app.txt: got %v, want Include", got)
	}

	rsNoDot := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeExtension, Action: filter.Exclude, Pattern: "tmp"}},
	}
	if got := decide(rsNoDot, "scratch.tmp", 1, fixedNow); got != filter.Exclude {
		t.Errorf("dotless pattern should match, got %v", got)
	}
}

func TestDecide_Size_AboveOnly(t *testing.T) {
	// Spec example: "exclude files larger than 4 GB".
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeSize, Action: filter.Exclude, SizeAboveBytes: 4 * 1024 * 1024 * 1024}},
	}
	if got := decide(rs, "small", 1024, fixedNow); got != filter.Include {
		t.Errorf("small file: got %v, want Include", got)
	}
	if got := decide(rs, "huge", 5*1024*1024*1024, fixedNow); got != filter.Exclude {
		t.Errorf("huge file: got %v, want Exclude", got)
	}
}

func TestDecide_Size_BelowOnly(t *testing.T) {
	// "Exclude tiny files (< 1KB)".
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeSize, Action: filter.Exclude, SizeBelowBytes: 1024}},
	}
	if got := decide(rs, "tiny", 100, fixedNow); got != filter.Exclude {
		t.Errorf("tiny file: got %v, want Exclude", got)
	}
	if got := decide(rs, "normal", 10*1024, fixedNow); got != filter.Include {
		t.Errorf("normal file: got %v, want Include", got)
	}
}

func TestDecide_Size_BothBounds(t *testing.T) {
	// Excludes files outside [1KB, 10KB] (both too small AND too big match).
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeSize, Action: filter.Exclude, SizeBelowBytes: 1024, SizeAboveBytes: 10240}},
	}
	if got := decide(rs, "tiny", 500, fixedNow); got != filter.Exclude {
		t.Errorf("tiny: got %v, want Exclude", got)
	}
	if got := decide(rs, "huge", 100000, fixedNow); got != filter.Exclude {
		t.Errorf("huge: got %v, want Exclude", got)
	}
	if got := decide(rs, "in range", 5000, fixedNow); got != filter.Include {
		t.Errorf("in range: got %v, want Include (no match)", got)
	}
}

func TestDecide_Modified_BeforeDaysAgo(t *testing.T) {
	// Spec example: "only include files modified in the last 30 days" →
	// exclude anything older than 30 days.
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeModified, Action: filter.Exclude, ModifiedBeforeDaysAgo: 30}},
	}
	if got := rs.Decide("ancient", 1, daysAgo(60)); got != filter.Exclude {
		t.Errorf("60d-old: got %v, want Exclude", got)
	}
	if got := rs.Decide("fresh", 1, daysAgo(7)); got != filter.Include {
		t.Errorf("7d-old: got %v, want Include", got)
	}
}

func TestDecide_Modified_WithinDays(t *testing.T) {
	// "Exclude files modified in the last 7 days" (e.g. avoid syncing
	// half-written files from an upstream that's still actively being edited).
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeModified, Action: filter.Exclude, ModifiedWithinDays: 7}},
	}
	if got := rs.Decide("fresh", 1, daysAgo(3)); got != filter.Exclude {
		t.Errorf("3d-old: got %v, want Exclude", got)
	}
	if got := rs.Decide("ancient", 1, daysAgo(60)); got != filter.Include {
		t.Errorf("60d-old: got %v, want Include", got)
	}
}

func TestDecide_FirstMatchWins(t *testing.T) {
	// include .env first → beats the generic exclude-hidden rule below.
	rs := &filter.Ruleset{
		ExcludeHidden: true,
		Rules: []filter.Rule{
			{Type: filter.TypeGlob, Action: filter.Include, Pattern: ".env"},
			{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: ".*"},
		},
	}
	if got := decide(rs, ".env", 100, fixedNow); got != filter.Include {
		t.Errorf(".env: got %v, want Include (rule 0 wins)", got)
	}
	if got := decide(rs, ".git/config", 100, fixedNow); got != filter.Exclude {
		t.Errorf(".git/config: got %v, want Exclude (rule 1)", got)
	}
}

func TestDecide_ExcludeHidden_AppliesWhenNoRuleMatches(t *testing.T) {
	rs := &filter.Ruleset{ExcludeHidden: true}
	if got := decide(rs, ".env", 100, fixedNow); got != filter.Exclude {
		t.Errorf("hidden path: got %v, want Exclude", got)
	}
	if got := decide(rs, "src/.cache/x", 100, fixedNow); got != filter.Exclude {
		t.Errorf("hidden segment in middle of path: got %v, want Exclude", got)
	}
	if got := decide(rs, "src/main.ts", 100, fixedNow); got != filter.Include {
		t.Errorf("non-hidden: got %v, want Include", got)
	}
}

func TestDecide_ExcludeHidden_OverriddenByExplicitIncludeRule(t *testing.T) {
	// The user explicitly includes .env even though hidden files are off.
	rs := &filter.Ruleset{
		ExcludeHidden: true,
		Rules: []filter.Rule{
			{Type: filter.TypeGlob, Action: filter.Include, Pattern: ".env"},
		},
	}
	if got := decide(rs, ".env", 100, fixedNow); got != filter.Include {
		t.Errorf("explicit include must beat ExcludeHidden, got %v", got)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		rs      filter.Ruleset
		wantErr string
	}{
		"empty is fine": {
			rs:      filter.Ruleset{},
			wantErr: "",
		},
		"valid glob": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "**/*.log"}}},
			wantErr: "",
		},
		"glob without pattern": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: filter.TypeGlob, Action: filter.Exclude}}},
			wantErr: "glob rule requires pattern",
		},
		"invalid glob pattern": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "[unclosed"}}},
			wantErr: "invalid glob pattern",
		},
		"size with no bounds": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: filter.TypeSize, Action: filter.Exclude}}},
			wantErr: "size rule requires size_above_bytes or size_below_bytes",
		},
		"size empty window": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: filter.TypeSize, Action: filter.Exclude, SizeBelowBytes: 1000, SizeAboveBytes: 100}}},
			wantErr: "window is empty",
		},
		"modified with no days": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: filter.TypeModified, Action: filter.Exclude}}},
			wantErr: "modified rule requires",
		},
		"bad action": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: filter.TypeGlob, Action: "skip", Pattern: "*"}}},
			wantErr: "invalid action",
		},
		"unknown type": {
			rs:      filter.Ruleset{Rules: []filter.Rule{{Type: "regex", Action: filter.Include, Pattern: ".+"}}},
			wantErr: "unknown rule type",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.rs.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRuleset_JSON_RoundTrip(t *testing.T) {
	rs := filter.Ruleset{
		ExcludeHidden: true,
		Rules: []filter.Rule{
			{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "**/*.log"},
			{Type: filter.TypeSize, Action: filter.Exclude, SizeAboveBytes: 4 * 1024 * 1024 * 1024},
			{Type: filter.TypeModified, Action: filter.Exclude, ModifiedBeforeDaysAgo: 30},
		},
	}
	blob, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got filter.Ruleset
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ExcludeHidden != rs.ExcludeHidden || len(got.Rules) != len(rs.Rules) {
		t.Fatalf("roundtrip mismatch:\n got %+v\nwant %+v", got, rs)
	}
	for i := range rs.Rules {
		if got.Rules[i] != rs.Rules[i] {
			t.Errorf("rule %d differs:\n got %+v\nwant %+v", i, got.Rules[i], rs.Rules[i])
		}
	}
}

func TestRuleset_EmptyJSON_IsLegacyBehaviour(t *testing.T) {
	// Existing jobs persisted with an empty blob must keep their pre-feature
	// behaviour (include everything, no hidden-file filtering).
	var rs filter.Ruleset
	if err := json.Unmarshal([]byte(`{}`), &rs); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if rs.ExcludeHidden {
		t.Error("empty JSON should yield ExcludeHidden=false")
	}
	if got := rs.Decide(".env", 100, fixedNow); got != filter.Include {
		t.Errorf("legacy default should include hidden files, got %v", got)
	}
}
