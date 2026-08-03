package capability

// getFloatConfig extracts a float64 value from a config map with a default fallback.
// Handles float64, float32, int, int64 (Go's encoding/json defaults to float64
// for numbers, but some paths may produce int).
func getFloatConfig(cfg map[string]any, key string, defaultVal float64) float64 {
	if cfg == nil {
		return defaultVal
	}
	v, ok := cfg[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return defaultVal
	}
}
