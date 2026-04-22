package parser

import "github.com/missdeer/notebooklm-client/internal/types"

func ParseStudioConfig(raw string) types.StudioConfig {
	inner := extractInner(raw)
	innerArr, ok := inner.([]any)
	if !ok {
		return types.StudioConfig{}
	}

	sections := innerArr
	if len(innerArr) > 0 {
		if first, ok := innerArr[0].([]any); ok {
			sections = first
		}
	}

	result := types.StudioConfig{}
	if len(sections) > 0 {
		result.AudioTypes = parseTypedSection(sections[0])
	}
	if len(sections) > 1 {
		result.ExplainerTypes = parseTypedSection(sections[1])
	}
	if len(sections) > 2 {
		result.SlideTypes = parseTypedSection(sections[2])
	}
	if len(sections) > 3 {
		result.DocTypes = parseDocSection(sections[3])
	}
	return result
}

func parseTypedSection(section any) []types.StudioAudioType {
	sArr, ok := section.([]any)
	if !ok || len(sArr) == 0 {
		return nil
	}
	items, ok := sArr[0].([]any)
	if !ok {
		return nil
	}
	var result []types.StudioAudioType
	for _, item := range items {
		arr, ok := item.([]any)
		if !ok {
			continue
		}
		t := types.StudioAudioType{}
		if len(arr) > 0 {
			if id, ok := arr[0].(float64); ok {
				t.ID = int(id)
			}
		}
		if len(arr) > 1 {
			t.Name, _ = arr[1].(string)
		}
		if len(arr) > 2 {
			t.Description, _ = arr[2].(string)
		}
		result = append(result, t)
	}
	return result
}

func parseDocSection(section any) []types.StudioDocType {
	sArr, ok := section.([]any)
	if !ok || len(sArr) == 0 {
		return nil
	}
	items, ok := sArr[0].([]any)
	if !ok {
		return nil
	}
	var result []types.StudioDocType
	for _, item := range items {
		arr, ok := item.([]any)
		if !ok {
			continue
		}
		t := types.StudioDocType{}
		if len(arr) > 0 {
			t.Name, _ = arr[0].(string)
		}
		if len(arr) > 1 {
			t.Description, _ = arr[1].(string)
		}
		result = append(result, t)
	}
	return result
}
