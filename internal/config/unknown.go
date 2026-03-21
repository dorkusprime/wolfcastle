package config

import (
	"encoding/json"
	"fmt"
)

// checkUnknownFields detects JSON keys that don't correspond to any field
// in the Config struct. It works by round-tripping: unmarshal the raw JSON
// leniently into Config (unknown keys silently dropped), marshal back to a
// map, then diff the original keys against the round-tripped keys. Any key
// present in the original but absent after the round-trip is unknown.
func checkUnknownFields(raw []byte, tier string) []string {
	var have map[string]any
	if err := json.Unmarshal(raw, &have); err != nil {
		return nil // parse errors are handled elsewhere
	}

	// Lenient unmarshal: unknown fields are silently ignored.
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}

	// Round-trip the struct back to a map; only known fields survive.
	known, err := structToMap(&cfg)
	if err != nil {
		return nil
	}

	var warnings []string
	diffKeys(have, known, "", tier, &warnings)
	return warnings
}

// diffKeys recursively compares keys in have against known, collecting
// warnings for any key present in have but absent from known. The prefix
// parameter builds the dot-delimited path for nested fields.
func diffKeys(have, known map[string]any, prefix, tier string, warnings *[]string) {
	for k, v := range have {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		knownVal, ok := known[k]
		if !ok {
			*warnings = append(*warnings, fmt.Sprintf("config: unknown field %q in %s", path, tier))
			continue
		}
		// Recurse into nested maps.
		haveMap, haveIsMap := v.(map[string]any)
		knownMap, knownIsMap := knownVal.(map[string]any)
		if haveIsMap && knownIsMap {
			diffKeys(haveMap, knownMap, path, tier, warnings)
		}
	}
}
