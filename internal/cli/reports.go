package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
)

type reportPlanItem struct {
	ElementID string            `json:"element_id"`
	Prompt    string            `json:"steering_prompt"`
	Settings  map[string]string `json:"settings"`
}

func manageReport(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	raw, err := api.GetRawArtifact(ctx, c.CallBatchExecute, nb, in.val("artifact_id"))
	if err != nil {
		return nil, err
	}
	if api.At(raw, 2) != float64(11) {
		return nil, fmt.Errorf("artifact is not an interactive report")
	}
	elements := api.ReportElements(raw)
	lang := in.val("language")
	if lang == "" {
		lang = api.StringAt(raw, 34, 1, 1)
	}
	if lang == "" {
		lang = "en"
	}
	switch in.schema.Canonical {
	case "report get":
		markdown := api.RenderReport(raw)
		if markdown == "" {
			return nil, fmt.Errorf("report has no generated content yet")
		}
		if output := in.val("output"); output != "" {
			return output, writeOutput(output, []byte(markdown))
		}
		if flagBool(in.cmd, "json") {
			return map[string]any{"markdown": markdown, "elements": elements, "artifact_id": in.val("artifact_id")}, nil
		}
		return markdown, nil
	case "report elements":
		out := []any{}
		waitIDs := flagStrings(in.cmd, "wait")
		for _, el := range elements {
			var meta []any
			check := func() (bool, error) {
				var e error
				meta, e = api.GetRawArtifact(ctx, c.CallBatchExecute, nb, el.ID)
				if e != nil {
					return false, e
				}
				if api.At(meta, 4) == float64(4) {
					return false, fmt.Errorf("element %s generation failed", el.ID)
				}
				return api.At(meta, 4) == float64(3), nil
			}
			if containsString(waitIDs, el.ID) {
				if err := waitUntil(ctx, time.Duration(flagInt(in.cmd, "timeout", 600))*time.Second, check); err != nil {
					return nil, err
				}
			} else {
				_, err = check()
				if err != nil {
					return nil, err
				}
			}
			item := map[string]any{"element_id": el.ID, "type": artifactKind(meta), "title": api.StringAt(meta, 1), "status": api.At(meta, 4), "description": api.StringAt(meta, 35, 1)}
			if flagBool(in.cmd, "content") {
				item["content"] = meta
			}
			out = append(out, item)
		}
		return out, nil
	}
	plan := []reportPlanItem{}
	if in.schema.Canonical == "report element create-batch" {
		data, err := os.ReadFile(in.val("plan"))
		if err != nil {
			return nil, err
		}
		if len(data) > 256<<10 {
			return nil, fmt.Errorf("plan exceeds 256 KiB")
		}
		if err := json.Unmarshal(data, &plan); err != nil {
			return nil, err
		}
	} else {
		item := reportPlanItem{ElementID: in.val("id"), Prompt: in.val("prompt"), Settings: map[string]string{}}
		for _, s := range flagStrings(in.cmd, "setting") {
			k, v, ok := strings.Cut(s, "=")
			if !ok {
				return nil, fmt.Errorf("setting must be name=value")
			}
			item.Settings[k] = v
		}
		if item.ElementID == "" {
			for _, el := range elements {
				meta, e := api.GetRawArtifact(ctx, c.CallBatchExecute, nb, el.ID)
				if e != nil {
					return nil, e
				}
				if artifactKind(meta) == in.val("type") {
					item.ElementID = el.ID
					break
				}
			}
		}
		plan = append(plan, item)
	}
	// Validate the full plan before any generation starts.
	type prepared struct {
		item     reportPlanItem
		sources  []string
		settings map[string]int
	}
	ready := []prepared{}
	seen := map[string]bool{}
	for _, item := range plan {
		belongs := false
		for _, el := range elements {
			belongs = belongs || el.ID == item.ElementID
		}
		if !belongs || seen[item.ElementID] {
			return nil, fmt.Errorf("element %s missing from report or duplicated", item.ElementID)
		}
		seen[item.ElementID] = true
		meta, e := api.GetRawArtifact(ctx, c.CallBatchExecute, nb, item.ElementID)
		if e != nil {
			return nil, e
		}
		if api.At(meta, 4) != float64(5) {
			return nil, fmt.Errorf("element %s is not suggested; check status before generating", item.ElementID)
		}
		settings, e := elementSettings(item.Settings)
		if e == nil {
			e = validateElementSettings(artifactKind(meta), item.Settings)
		}
		if e != nil {
			return nil, e
		}
		sids := []string{}
		for _, s := range api.ArrayAt(raw, 3) {
			if id := api.StringAt(s, 0, 0); id != "" {
				sids = append(sids, id)
			}
		}
		if len(sids) == 0 {
			return nil, fmt.Errorf("report has no source scope")
		}
		if item.Prompt == "" {
			item.Prompt = api.StringAt(meta, 35, 1)
		}
		ready = append(ready, prepared{item, sids, settings})
	}
	out := []any{}
	for _, p := range ready {
		v, e := api.StartReportElement(ctx, c.CallBatchExecute, nb, p.item.ElementID, p.item.Prompt, lang, p.sources, p.settings)
		if e != nil {
			_ = emitResult(in.cmd, out)
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

func containsString(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

func elementSettings(named map[string]string) (map[string]int, error) {
	tables := map[string]map[string]int{"difficulty": {"easy": 1, "medium": 2, "hard": 3}, "question_amount": {"fewer": 1, "standard": 2, "more": 3}, "card_amount": {"fewer": 1, "standard": 2, "more": 3}, "orientation": {"landscape": 1, "portrait": 2, "square": 3}, "detail_level": {"concise": 1, "standard": 2, "detailed": 3}, "infographic_style": {"auto_select": 1, "auto": 1, "sketch_note": 2, "professional": 3, "bento_grid": 4, "editorial": 5, "instructional": 6, "bricks": 7, "clay": 8, "anime": 9, "kawaii": 10, "scientific": 11}, "slide_format": {"detailed_deck": 1, "presenter_slides": 2}, "slide_length": {"short": 2, "default": 3}, "audio_format": {"deep_dive": 1, "brief": 2, "critique": 3, "debate": 4}, "video_format": {"explainer": 1, "short": 4, "cinematic": 3}}
	out := map[string]int{}
	for k, v := range named {
		code, ok := tables[k][v]
		if !ok {
			return nil, fmt.Errorf("invalid element setting %s=%s", k, v)
		}
		key := k
		if k == "question_amount" || k == "card_amount" {
			key = "amount"
		}
		out[key] = code
	}
	return out, nil
}

func validateElementSettings(kind string, named map[string]string) error {
	allowed := map[string][]string{"quiz": {"question_amount", "difficulty"}, "flashcards": {"card_amount", "difficulty"}, "infographic": {"orientation", "detail_level", "infographic_style"}, "slide_deck": {"slide_format", "slide_length"}, "audio": {"audio_format"}, "video": {"video_format"}, "mind_map": {}}
	keys, ok := allowed[kind]
	if !ok {
		return fmt.Errorf("unsupported element kind %s", kind)
	}
	for k := range named {
		if !containsString(keys, k) {
			return fmt.Errorf("setting %s does not apply to %s", k, kind)
		}
	}
	return nil
}
