package parser

import (
	"encoding/json"
	"regexp"
	"strings"
)

var safetyPrefixRe = regexp.MustCompile(`^\s*\)]\}'\s*\n?`)
var digitLineRe = regexp.MustCompile(`^\d+$`)

func StripSafetyPrefix(raw string) string {
	return strings.TrimSpace(safetyPrefixRe.ReplaceAllString(raw, ""))
}

func ExtractJSONChunks(body string) [][]any {
	var chunks [][]any
	lines := strings.Split(body, "\n")

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		if digitLineRe.MatchString(line) {
			// Try single-line JSON on next line
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if nextLine != "" {
					var parsed []any
					if err := json.Unmarshal([]byte(nextLine), &parsed); err == nil {
						chunks = append(chunks, parsed)
						i += 2
						continue
					}
				}
			}

			// Multi-line fallback: accumulate up to byte length
			length := 0
			for _, c := range line {
				length = length*10 + int(c-'0')
			}
			var jsonStr strings.Builder
			j := i + 1
			for j < len(lines) && jsonStr.Len() < length {
				if jsonStr.Len() > 0 {
					jsonStr.WriteByte('\n')
				}
				jsonStr.WriteString(lines[j])
				j++
			}
			if s := strings.TrimSpace(jsonStr.String()); s != "" {
				var parsed []any
				if err := json.Unmarshal([]byte(s), &parsed); err == nil {
					chunks = append(chunks, parsed)
				}
			}
			i = j
		} else {
			var parsed []any
			if err := json.Unmarshal([]byte(line), &parsed); err == nil {
				chunks = append(chunks, parsed)
			}
			i++
		}
	}
	return chunks
}

func ParseEnvelopes(raw string) [][]any {
	stripped := StripSafetyPrefix(raw)
	chunks := ExtractJSONChunks(stripped)
	var results [][]any

	for _, chunk := range chunks {
		for _, item := range chunk {
			env, ok := item.([]any)
			if !ok || len(env) < 3 {
				continue
			}
			if tag, ok := env[0].(string); !ok || tag != "wrb.fr" {
				continue
			}
			innerStr, ok := env[2].(string)
			if !ok {
				continue
			}
			var parsed []any
			if err := json.Unmarshal([]byte(innerStr), &parsed); err == nil {
				results = append(results, parsed)
			}
		}
	}
	return results
}
