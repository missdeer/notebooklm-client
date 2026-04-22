package parser

func get(data any, path ...int) any {
	current := data
	for _, idx := range path {
		arr, ok := current.([]any)
		if !ok || idx < 0 || idx >= len(arr) {
			return nil
		}
		current = arr[idx]
	}
	return current
}

func getString(data any, path ...int) string {
	val := get(data, path...)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

func getFloat(data any, path ...int) (float64, bool) {
	val := get(data, path...)
	if f, ok := val.(float64); ok {
		return f, true
	}
	return 0, false
}

func getArray(data any, path ...int) []any {
	val := get(data, path...)
	if arr, ok := val.([]any); ok {
		return arr
	}
	return nil
}

func extractInner(raw string) any {
	envelopes := ParseEnvelopes(raw)
	if len(envelopes) > 0 {
		return envelopes[0]
	}
	return nil
}

func extractAllInner(raw string) []any {
	envelopes := ParseEnvelopes(raw)
	result := make([]any, len(envelopes))
	for i, e := range envelopes {
		result[i] = e
	}
	return result
}
