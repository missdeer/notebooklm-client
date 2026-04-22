package parser

import (
	"fmt"
	"regexp"

	"github.com/missdeer/notebooklm-client/internal/types"
)

var uuidPrefixRe = regexp.MustCompile(`^[0-9a-f]{8}-`)

func ParseCreateNotebook(raw string) (string, error) {
	inner := extractInner(raw)
	id := getString(inner, 2)
	if id == "" {
		return "", fmt.Errorf("failed to parse notebook ID from create response")
	}
	return id, nil
}

func ParseListNotebooks(raw string) []types.NotebookInfo {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return nil
	}

	entries := innerArr
	if len(innerArr) > 0 {
		if first, ok := innerArr[0].([]any); ok {
			entries = first
		}
	}

	var notebooks []types.NotebookInfo
	for _, entry := range entries {
		arr, ok := entry.([]any)
		if !ok {
			continue
		}
		title, _ := arr[0].(string)
		id := ""
		if len(arr) > 2 {
			id, _ = arr[2].(string)
		}
		if id == "" || !uuidPrefixRe.MatchString(id) {
			continue
		}
		nb := types.NotebookInfo{ID: id, Title: title}
		if len(arr) > 1 {
			if srcArr, ok := arr[1].([]any); ok {
				n := len(srcArr)
				nb.SourceCount = &n
			}
		}
		notebooks = append(notebooks, nb)
	}
	return notebooks
}

func ParseNotebookDetail(raw string) (string, []types.SourceInfo) {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return "", nil
	}

	entry := innerArr
	if len(innerArr) > 0 {
		if first, ok := innerArr[0].([]any); ok {
			entry = first
		}
	}

	title, _ := entry[0].(string)
	var sources []types.SourceInfo

	if len(entry) > 1 {
		if sourcesArr, ok := entry[1].([]any); ok {
			for _, srcEntry := range sourcesArr {
				src, ok := srcEntry.([]any)
				if !ok {
					continue
				}
				var id string
				if first, ok := src[0].([]any); ok && len(first) > 0 {
					id, _ = first[0].(string)
				}
				if id == "" {
					continue
				}
				srcTitle := ""
				if len(src) > 1 {
					srcTitle, _ = src[1].(string)
				}
				si := types.SourceInfo{ID: id, Title: srcTitle}
				if len(src) > 2 {
					if meta, ok := src[2].([]any); ok {
						if len(meta) > 1 {
							if wc, ok := meta[1].(float64); ok {
								n := int(wc)
								si.WordCount = &n
							}
						}
						if len(meta) > 7 {
							if urlArr, ok := meta[7].([]any); ok && len(urlArr) > 0 {
								si.URL, _ = urlArr[0].(string)
							} else if urlStr, ok := meta[7].(string); ok {
								si.URL = urlStr
							}
						}
					}
				}
				sources = append(sources, si)
			}
		}
	}
	return title, sources
}
