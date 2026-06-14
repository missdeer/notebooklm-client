package parser

import (
	"strings"

	"github.com/missdeer/notebooklm-client/internal/types"
)

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

// chunkDetail mirrors a single entry in the retrieval-chunks tree ([4][3]).
type chunkDetail struct {
	sourceID       string
	relevance      float64
	hasRelevance   bool
	excerpt        string
}

// flattenExcerptTree concatenates strings out of an arbitrarily nested array.
func flattenExcerptTree(node any) string {
	switch n := node.(type) {
	case string:
		return n
	case []any:
		var b strings.Builder
		for _, child := range n {
			b.WriteString(flattenExcerptTree(child))
		}
		return b.String()
	}
	return ""
}

func buildChunkMap(citationTree []any) map[string]chunkDetail {
	out := make(map[string]chunkDetail)
	for _, cite := range citationTree {
		arr, ok := cite.([]any)
		if !ok {
			continue
		}
		chunkID := getString(arr, 0, 0)
		if chunkID == "" {
			continue
		}
		meta := getArray(arr, 1)
		if meta == nil {
			continue
		}
		d := chunkDetail{}
		if len(meta) > 2 {
			if f, ok := meta[2].(float64); ok {
				d.relevance = f
				d.hasRelevance = true
			}
		}
		d.excerpt = flattenExcerptTree(get(meta, 4))
		d.sourceID = getString(meta, 5, 0, 0, 0)
		out[chunkID] = d
	}
	return out
}

// ParseChatWithCitations joins inline citation refs ([4][0][1]) with the
// retrieval-chunks tree ([4][3]) emitted by the chat stream RPC.
func ParseChatWithCitations(raw string) types.ChatWithCitationsResult {
	text, threadID, responseID := ParseChatStream(raw)
	result := types.ChatWithCitationsResult{
		Text:       text,
		ThreadID:   threadID,
		ResponseID: responseID,
	}

	inners := extractAllInner(raw)
	var lastInlineRefs []any
	var lastChunkMap map[string]chunkDetail

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
		answerData := getArray(payload, 4, 0)
		if answerData == nil || len(answerData) < 2 {
			continue
		}
		refs, ok := answerData[1].([]any)
		if !ok {
			continue
		}
		lastInlineRefs = refs
		if citationTree := getArray(payload, 4, 3); citationTree != nil {
			lastChunkMap = buildChunkMap(citationTree)
		} else {
			lastChunkMap = map[string]chunkDetail{}
		}
	}

	if lastInlineRefs == nil {
		return result
	}

	citations := make([]types.ChatCitation, 0, len(lastInlineRefs))
	for i, ref := range lastInlineRefs {
		refArr, ok := ref.([]any)
		if !ok {
			continue
		}
		chunkID := getString(refArr, 0, 0)
		if chunkID == "" {
			continue
		}
		refMeta := getArray(refArr, 1)
		cite := types.ChatCitation{
			Index:   i + 1,
			ChunkID: chunkID,
		}
		if refMeta != nil {
			if len(refMeta) > 1 {
				if f, ok := refMeta[1].(float64); ok {
					cite.CharStart = int(f)
					cite.HasCharRange = true
				}
			}
			if len(refMeta) > 2 {
				if f, ok := refMeta[2].(float64); ok {
					cite.CharEnd = int(f)
					cite.HasCharRange = true
				}
			}
		}
		if lastChunkMap != nil {
			if d, ok := lastChunkMap[chunkID]; ok {
				cite.SourceID = d.sourceID
				cite.Excerpt = d.excerpt
				if d.hasRelevance {
					cite.Relevance = d.relevance
					cite.HasRelevance = true
				}
			}
		}
		citations = append(citations, cite)
	}
	result.Citations = citations
	return result
}
