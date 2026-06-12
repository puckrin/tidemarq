package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/tidemarq/tidemarq/internal/filter"
)

// encodeFilters serialises a Ruleset to the JSON string stored in jobs.filters_json.
// nil → "{}" (the column default). The ruleset is validated before encoding so an
// invalid ruleset never reaches the database.
func encodeFilters(rs *filter.Ruleset) (string, error) {
	if rs == nil {
		return "{}", nil
	}
	if err := rs.Validate(); err != nil {
		return "", fmt.Errorf("invalid filters: %w", err)
	}
	b, err := json.Marshal(rs)
	if err != nil {
		return "", fmt.Errorf("encoding filters: %w", err)
	}
	return string(b), nil
}

// parseFilters deserialises the persisted filters_json blob. An empty string or
// "{}" returns (nil, nil) — equivalent to "no filtering", matching the
// pre-feature behaviour. Any other parse or validation failure is returned so
// the caller can refuse to run rather than silently sync the wrong files.
func parseFilters(s string) (*filter.Ruleset, error) {
	if s == "" || s == "{}" {
		return nil, nil
	}
	var rs filter.Ruleset
	if err := json.Unmarshal([]byte(s), &rs); err != nil {
		return nil, fmt.Errorf("decoding filters: %w", err)
	}
	if err := rs.Validate(); err != nil {
		return nil, fmt.Errorf("invalid filters: %w", err)
	}
	if !rs.ExcludeHidden && len(rs.Rules) == 0 {
		return nil, nil
	}
	return &rs, nil
}
