package parser

func ParseAddSource(raw string) (sourceID, title string) {
	inner := extractInner(raw)
	entry := getArray(inner, 0, 0)
	if entry == nil {
		return "", ""
	}
	idArr := getArray(entry, 0)
	if idArr != nil && len(idArr) > 0 {
		sourceID, _ = idArr[0].(string)
	}
	title = getString(entry, 1)
	return sourceID, title
}

func ParseSourceContent(raw string) (id, title string, wordCount int) {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return "", "", 0
	}
	idArr := getArray(innerArr, 0)
	if idArr != nil && len(idArr) > 0 {
		id, _ = idArr[0].(string)
	}
	title = getString(innerArr, 1)
	meta := getArray(innerArr, 2)
	if meta != nil && len(meta) > 1 {
		if wc, ok := meta[1].(float64); ok {
			wordCount = int(wc)
		}
	}
	return id, title, wordCount
}

func ParseSourceSummary(raw string) (sourceID, summary string) {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return "", ""
	}
	sourceID = getString(innerArr, 0, 0, 0, 0)
	if len(innerArr) > 1 {
		summary, _ = innerArr[1].(string)
	}
	return sourceID, summary
}
