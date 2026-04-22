package parser

func ParseChatStream(raw string) (text, threadID, responseID string) {
	inners := extractAllInner(raw)

	for _, inner := range inners {
		innerArr, ok := inner.([]any)
		if !ok {
			continue
		}
		payload := innerArr
		if len(innerArr) > 0 {
			if first, ok := innerArr[0].([]any); ok {
				payload = first
			}
		}
		if len(payload) > 0 {
			if t, ok := payload[0].(string); ok && t != "" {
				text = t
			}
		}
		meta := getArray(payload, 2)
		if meta != nil {
			if len(meta) > 0 {
				if tid, ok := meta[0].(string); ok && tid != "" {
					threadID = tid
				}
			}
			if len(meta) > 1 {
				if rid, ok := meta[1].(string); ok && rid != "" {
					responseID = rid
				}
			}
		}
	}
	return text, threadID, responseID
}
