package jobs

import (
	"strings"
	"testing"

	"github.com/tidemarq/tidemarq/internal/filter"
)

func TestEncodeFilters_NilReturnsEmptyObject(t *testing.T) {
	got, err := encodeFilters(nil)
	if err != nil {
		t.Fatalf("encodeFilters(nil): %v", err)
	}
	if got != "{}" {
		t.Errorf("got %q, want %q", got, "{}")
	}
}

func TestEncodeFilters_ValidatesBeforeWriting(t *testing.T) {
	// A modified-date rule with no fields set must be rejected; persisting it
	// would cause every subsequent job run to fail at parse time.
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeModified, Action: filter.Exclude}},
	}
	_, err := encodeFilters(rs)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid filters") {
		t.Errorf("expected wrapped 'invalid filters' error, got %v", err)
	}
}

func TestParseFilters_EmptyStringAndEmptyObjectAreLegacy(t *testing.T) {
	// Both "" (column never written) and "{}" (column default) must parse to
	// nil — the engine's "no filtering" sentinel.
	for _, s := range []string{"", "{}"} {
		rs, err := parseFilters(s)
		if err != nil {
			t.Errorf("parseFilters(%q): unexpected error %v", s, err)
		}
		if rs != nil {
			t.Errorf("parseFilters(%q): got %+v, want nil", s, rs)
		}
	}
}

func TestParseFilters_RuleSetIsValidated(t *testing.T) {
	// Hand-crafted invalid blob (could only land in the DB via a manual edit
	// since the API validates at create/update). parseFilters refuses to
	// return a ruleset so the run aborts before mishandling files.
	const bad = `{"rules":[{"type":"glob","action":"exclude"}]}`
	rs, err := parseFilters(bad)
	if err == nil {
		t.Fatalf("expected validation error, got rs=%+v", rs)
	}
	if !strings.Contains(err.Error(), "invalid filters") {
		t.Errorf("expected wrapped 'invalid filters' error, got %v", err)
	}
}

func TestParseFilters_EmptyRulesetWithDefaultsCollapsesToNil(t *testing.T) {
	// A persisted ruleset with no rules and ExcludeHidden=false is identical
	// to no filtering. Collapsing it to nil saves the engine an indirection
	// (and the comparison in Decide()).
	rs, err := parseFilters(`{"rules":[]}`)
	if err != nil {
		t.Fatalf("parseFilters: %v", err)
	}
	if rs != nil {
		t.Errorf("expected nil for empty ruleset, got %+v", rs)
	}
}

func TestRoundtrip_EncodeThenParse(t *testing.T) {
	in := &filter.Ruleset{
		ExcludeHidden: true,
		Rules: []filter.Rule{
			{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "**/*.log"},
			{Type: filter.TypeSize, Action: filter.Exclude, SizeAboveBytes: 4 * 1024 * 1024 * 1024},
		},
	}
	blob, err := encodeFilters(in)
	if err != nil {
		t.Fatalf("encodeFilters: %v", err)
	}
	got, err := parseFilters(blob)
	if err != nil {
		t.Fatalf("parseFilters: %v", err)
	}
	if got == nil || got.ExcludeHidden != in.ExcludeHidden || len(got.Rules) != len(in.Rules) {
		t.Fatalf("roundtrip mismatch:\n got %+v\nwant %+v", got, in)
	}
	for i, r := range in.Rules {
		if got.Rules[i] != r {
			t.Errorf("rule %d differs:\n got %+v\nwant %+v", i, got.Rules[i], r)
		}
	}
}
