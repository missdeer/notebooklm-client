package parser

import "github.com/missdeer/notebooklm-client/internal/types"

func ParseAccountInfo(raw string) types.AccountInfo {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return types.AccountInfo{}
	}

	entry := innerArr
	if len(innerArr) > 0 {
		if first, ok := innerArr[0].([]any); ok {
			if len(innerArr) < 2 || innerArr[1] == nil {
				entry = first
			}
		}
	}

	info := types.AccountInfo{}
	if limits := getArray(entry, 1); limits != nil {
		if len(limits) > 0 {
			if v, ok := limits[0].(float64); ok {
				info.PlanType = int(v)
			}
		}
		if len(limits) > 1 {
			if v, ok := limits[1].(float64); ok {
				info.NotebookLimit = int(v)
			}
		}
		if len(limits) > 2 {
			if v, ok := limits[2].(float64); ok {
				info.SourceLimit = int(v)
			}
		}
		if len(limits) > 3 {
			if v, ok := limits[3].(float64); ok {
				info.SourceWordLimit = int(v)
			}
		}
	}
	if flags := getArray(entry, 4); flags != nil && len(flags) > 0 {
		info.IsPlus, _ = flags[0].(bool)
	}
	return info
}
