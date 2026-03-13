package config

// DeepMerge merges src into dst recursively.
// Objects are deep-merged. Arrays in src replace dst entirely.
// Null values in src delete the key from dst.
func DeepMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}
	for k, sv := range src {
		// Null deletion
		if sv == nil {
			delete(dst, k)
			continue
		}
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		// Both are maps: recurse
		dMap, dIsMap := dv.(map[string]any)
		sMap, sIsMap := sv.(map[string]any)
		if dIsMap && sIsMap {
			dst[k] = DeepMerge(dMap, sMap)
			continue
		}
		// Otherwise src wins (including arrays)
		dst[k] = sv
	}
	return dst
}
