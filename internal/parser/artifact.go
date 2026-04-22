package parser

import (
	"strings"

	"github.com/missdeer/notebooklm-client/internal/types"
)

func ParseGenerateArtifact(raw string) (artifactID, title string) {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return "", ""
	}
	entry := innerArr
	if len(innerArr) > 0 {
		if first, ok := innerArr[0].([]any); ok {
			entry = first
		}
	}
	if len(entry) > 0 {
		artifactID, _ = entry[0].(string)
	}
	if len(entry) > 1 {
		title, _ = entry[1].(string)
	}
	return artifactID, title
}

func ParseArtifacts(raw string) []types.ArtifactInfo {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return nil
	}

	entries := innerArr
	if len(entries) == 1 {
		if first, ok := entries[0].([]any); ok {
			entries = first
		}
	}

	var artifacts []types.ArtifactInfo
	for _, rawEntry := range entries {
		arr, ok := rawEntry.([]any)
		if !ok {
			continue
		}
		entry := arr
		if len(arr) == 1 {
			if first, ok := arr[0].([]any); ok {
				entry = first
			}
		}

		id := ""
		if len(entry) > 0 {
			id, _ = entry[0].(string)
		}
		if id == "" {
			continue
		}
		title := ""
		if len(entry) > 1 {
			title, _ = entry[1].(string)
		}
		artType := 0
		if len(entry) > 2 {
			if t, ok := entry[2].(float64); ok {
				artType = int(t)
			}
		}

		artifact := types.ArtifactInfo{ID: id, Title: title, Type: artType}

		if srcIDs := getArray(entry, 3); srcIDs != nil {
			for _, sid := range srcIDs {
				sidArr, ok := sid.([]any)
				if !ok {
					continue
				}
				if inner, ok := sidArr[0].([]any); ok && len(inner) > 0 {
					if s, ok := inner[0].(string); ok {
						artifact.SourceIDs = append(artifact.SourceIDs, s)
					}
				} else if s, ok := sidArr[0].(string); ok {
					artifact.SourceIDs = append(artifact.SourceIDs, s)
				}
			}
		}

		media := findMediaURLs(entry, 0)
		if media.Download != "" {
			artifact.DownloadURL = media.Download
		}
		if media.Stream != "" {
			artifact.StreamURL = media.Stream
		}
		if media.HLS != "" {
			artifact.HlsURL = media.HLS
		}
		if media.Dash != "" {
			artifact.DashURL = media.Dash
		}
		if media.DurationSeconds > 0 {
			ds := media.DurationSeconds
			artifact.DurationSeconds = &ds
		}
		if media.DurationNanos > 0 {
			dn := media.DurationNanos
			artifact.DurationNanos = &dn
		}

		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

type mediaURLs struct {
	Download        string
	Stream          string
	HLS             string
	Dash            string
	DurationSeconds int
	DurationNanos   int
}

func findMediaURLs(data any, depth int) mediaURLs {
	if depth > 12 || data == nil {
		return mediaURLs{}
	}

	arr, ok := data.([]any)
	if !ok {
		return mediaURLs{}
	}

	// Duration pair: [seconds, nanos]
	if len(arr) == 2 {
		s, sok := arr[0].(float64)
		n, nok := arr[1].(float64)
		if sok && nok && s > 10 && s < 100000 && n > 1000000 {
			return mediaURLs{DurationSeconds: int(s), DurationNanos: int(n)}
		}
	}

	// Media URL variants array
	if len(arr) >= 2 {
		if first, ok := arr[0].([]any); ok && len(first) > 0 {
			if urlStr, ok := first[0].(string); ok && strings.Contains(urlStr, "googleusercontent.com/notebooklm/") {
				result := mediaURLs{}
				for _, variant := range arr {
					vArr, ok := variant.([]any)
					if !ok || len(vArr) < 1 {
						continue
					}
					u, ok := vArr[0].(string)
					if !ok {
						continue
					}
					typeCode := 0
					if len(vArr) > 1 {
						if tc, ok := vArr[1].(float64); ok {
							typeCode = int(tc)
						}
					}
					switch {
					case strings.Contains(u, "=m140-dv") || typeCode == 4:
						result.Download = u
					case strings.Contains(u, "=m140") || typeCode == 1:
						result.Stream = u
					case strings.Contains(u, "=mm,hls") || typeCode == 2:
						result.HLS = u
					case strings.Contains(u, "=mm,dash") || typeCode == 3:
						result.Dash = u
					}
				}
				return result
			}
		}
	}

	// Recurse
	merged := mediaURLs{}
	for _, item := range arr {
		found := findMediaURLs(item, depth+1)
		if found.Download != "" {
			merged.Download = found.Download
		}
		if found.Stream != "" {
			merged.Stream = found.Stream
		}
		if found.HLS != "" {
			merged.HLS = found.HLS
		}
		if found.Dash != "" {
			merged.Dash = found.Dash
		}
		if found.DurationSeconds > 0 {
			merged.DurationSeconds = found.DurationSeconds
		}
		if found.DurationNanos > 0 {
			merged.DurationNanos = found.DurationNanos
		}
	}
	return merged
}

func FindArtifactDownloadURL(raw string, artifactID string) string {
	artifacts := ParseArtifacts(raw)
	for _, a := range artifacts {
		if a.ID == artifactID && a.DownloadURL != "" {
			return a.DownloadURL
		}
	}
	return ""
}
