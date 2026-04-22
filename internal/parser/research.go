package parser

import "github.com/missdeer/notebooklm-client/internal/types"

type ResearchParseResult struct {
	Status  int
	Results []types.ResearchResult
	Report  string
}

func ParseResearchResults(raw string) ResearchParseResult {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return ResearchParseResult{}
	}

	outerList := getArray(innerArr, 0)
	if outerList == nil {
		return ResearchParseResult{}
	}

	wrapper, ok := outerList[0].([]any)
	if !ok {
		wrapper = outerList
	}

	var entryArr []any
	if len(wrapper) > 0 {
		if _, ok := wrapper[0].(string); ok {
			entryArr = wrapper
		} else if inner, ok := wrapper[0].([]any); ok && len(inner) > 0 {
			if _, ok := inner[0].(string); ok {
				entryArr = inner
			}
		}
	}
	if entryArr == nil {
		return ResearchParseResult{}
	}

	taskInfo := getArray(entryArr, 1)
	if taskInfo == nil {
		return ResearchParseResult{}
	}

	statusCode := 0
	if len(taskInfo) > 4 {
		if v, ok := taskInfo[4].(float64); ok {
			statusCode = int(v)
		}
	}
	if statusCode == 0 && len(taskInfo) > 2 {
		if v, ok := taskInfo[2].(float64); ok {
			statusCode = int(v)
		}
	}

	isCompleted := statusCode == 2 || statusCode == 6
	status := statusCode
	if isCompleted {
		status = 2
	}

	result := ResearchParseResult{Status: status}

	sourcesAndSummary := getArray(taskInfo, 3)
	if sourcesAndSummary == nil {
		return result
	}

	var sourceItems []any
	if len(sourcesAndSummary) > 0 {
		if first, ok := sourcesAndSummary[0].([]any); ok {
			if len(first) > 0 {
				if _, ok := first[0].([]any); ok {
					sourceItems = first
				} else {
					sourceItems = sourcesAndSummary
				}
			} else {
				sourceItems = sourcesAndSummary
			}
		} else {
			sourceItems = sourcesAndSummary
		}
	}

	for _, item := range sourceItems {
		arr, ok := item.([]any)
		if !ok {
			continue
		}

		// Deep research report: [null, [title, markdown], null, 3, ...]
		if len(arr) > 1 && arr[0] == nil {
			if pair, ok := arr[1].([]any); ok && len(pair) >= 2 {
				if _, tok := pair[0].(string); tok {
					if md, mok := pair[1].(string); mok && result.Report == "" {
						result.Report = md
						continue
					}
				}
			}
			// Legacy report: [null, title, null, type, ..., [chunks]]
			if len(arr) > 6 {
				if _, ok := arr[1].(string); ok {
					if chunks, ok := arr[6].([]any); ok && result.Report == "" {
						var parts []string
						for _, c := range chunks {
							if s, ok := c.(string); ok {
								parts = append(parts, s)
							}
						}
						if len(parts) > 0 {
							joined := ""
							for i, p := range parts {
								if i > 0 {
									joined += "\n\n"
								}
								joined += p
							}
							result.Report = joined
						}
					}
				}
			}
			continue
		}

		// URL source: [url, title, desc, type]
		u, _ := arr[0].(string)
		if u == "" {
			continue
		}
		t := ""
		if len(arr) > 1 {
			t, _ = arr[1].(string)
		}
		d := ""
		if len(arr) > 2 {
			d, _ = arr[2].(string)
		}
		result.Results = append(result.Results, types.ResearchResult{URL: u, Title: t, Description: d})
	}

	return result
}
