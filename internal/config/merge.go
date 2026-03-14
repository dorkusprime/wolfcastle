package config

// DeepMerge merges src into dst recursively, returning a new map.
// The original dst is not mutated (clone-before-merge).
// Objects are deep-merged. Arrays in src replace dst entirely.
// Null values in src delete the key from dst.
func DeepMerge(dst, src map[string]any) map[string]any {
	result := cloneMap(dst)
	for k, sv := range src {
		// Null deletion
		if sv == nil {
			delete(result, k)
			continue
		}
		dv, exists := result[k]
		if !exists {
			result[k] = cloneValue(sv)
			continue
		}
		// Both are maps: recurse
		dMap, dIsMap := dv.(map[string]any)
		sMap, sIsMap := sv.(map[string]any)
		if dIsMap && sIsMap {
			result[k] = DeepMerge(dMap, sMap)
			continue
		}
		// Otherwise src wins (including arrays)
		result[k] = cloneValue(sv)
	}
	return result
}

// cloneMap returns a shallow-value deep copy of in, recursively cloning
// nested maps and slices so the result shares no mutable state with in.
func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneValue(v)
	}
	return out
}

// cloneValue recursively copies maps and slices, returning scalars as-is.
func cloneValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneValue(item)
		}
		return out
	default:
		return typed
	}
}
