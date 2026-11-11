package api

import (
	"context"
	"fmt"
	"strings"
)

var ReportElementTypes = []any{1, 4, 8, 10, 14, 2, 3, 6}

func ReportConfig(wide bool) []any {
	types := append([]any{}, ReportElementTypes...)
	if wide {
		types = append(types, 7)
	}
	return []any{2, nil, nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}, []any{types}}
}
func CreateInteractiveReport(ctx context.Context, call RpcCaller, notebook, prompt, language string, sids []string) (any, error) {
	content := make([]any, 35)
	content[2] = 11
	sources := []any{}
	for _, id := range sids {
		sources = append(sources, []any{[]any{id}})
	}
	content[3] = sources
	content[34] = []any{nil, []any{prompt, language}}
	data, err := DecodedRPC(ctx, call, "R7cb6c", []any{ReportConfig(false), notebook, content}, notebook)
	if err != nil {
		return nil, err
	}
	id := StringAt(data, 0, 0)
	if id == "" {
		return nil, fmt.Errorf("interactive report returned no artifact ID")
	}
	return map[string]any{"artifact_id": id, "type": "interactive_report", "status": "queued"}, nil
}
func GetRawArtifact(ctx context.Context, call RpcCaller, notebook, id string) ([]any, error) {
	data, err := DecodedRPC(ctx, call, "v9rmvd", []any{id, ReportConfig(true)}, notebook)
	if err != nil {
		return nil, err
	}
	raw := ArrayAt(data, 0)
	if StringAt(raw, 0) != id {
		return nil, fmt.Errorf("artifact %s missing from response", id)
	}
	return raw, nil
}

type ReportElement struct {
	ID         string `json:"element_id"`
	BlockIndex int    `json:"block_index"`
	Flag       any    `json:"flag"`
}

func ReportElements(raw []any) []ReportElement {
	out := []ReportElement{}
	for i, b := range ArrayAt(raw, 34, 0, 0) {
		if id := StringAt(b, 12, 0); id != "" {
			out = append(out, ReportElement{id, i, At(b, 12, 1)})
		}
	}
	return out
}
func firstText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	for _, x := range ArrayAt(v) {
		if s := firstText(x); s != "" {
			return s
		}
	}
	return ""
}
func RenderReport(raw []any) string {
	var out strings.Builder
	for _, b := range ArrayAt(raw, 34, 0, 0) {
		if id := StringAt(b, 12, 0); id != "" {
			fmt.Fprintf(&out, "\n[Embedded element: %s]\n\n", id)
			continue
		}
		if At(b, 0) == nil && At(b, 1) == nil {
			if title := firstText(At(b, 2)); title != "" {
				fmt.Fprintf(&out, "\n## %s\n\n", title)
			}
			continue
		}
		var text strings.Builder
		for _, group := range ArrayAt(b, 2) {
			for _, span := range ArrayAt(group) {
				s := StringAt(span, 2, 0)
				if At(span, 2, 1, 0) == true {
					s = "**" + s + "**"
				}
				text.WriteString(s)
			}
		}
		if text.Len() > 0 {
			fmt.Fprintln(&out, text.String())
			fmt.Fprintln(&out)
		}
	}
	return out.String()
}
func StartReportElement(ctx context.Context, call RpcCaller, notebook, id, prompt, language string, sids []string, settings map[string]int) (any, error) {
	raw, err := GetRawArtifact(ctx, call, notebook, id)
	if err != nil {
		return nil, err
	}
	if len(sids) == 0 {
		for _, v := range ArrayAt(raw, 3) {
			if s := StringAt(v, 0, 0); s != "" {
				sids = append(sids, s)
			}
		}
	}
	if len(sids) == 0 {
		return nil, fmt.Errorf("element %s has no source scope", id)
	}
	sources := []any{}
	for _, sid := range sids {
		sources = append(sources, []any{[]any{sid}})
	}
	update, mask, err := BuildElementUpdate(raw, sources, prompt, language, settings)
	if err != nil {
		return nil, err
	}
	if _, err := DecodedRPC(ctx, call, "rc3d8d", []any{update, []any{mask}, nil, ReportConfig(true)}, notebook); err != nil {
		return nil, err
	}
	data, err := DecodedRPC(ctx, call, "Rytqqe", []any{ReportConfig(false), id}, notebook)
	if err != nil {
		return nil, fmt.Errorf("element generation outcome unknown; check studio status before retrying: %w", err)
	}
	if StringAt(data, 0, 0) != id {
		return nil, fmt.Errorf("element generation acknowledgement missing; check studio status before retrying")
	}
	return map[string]any{"artifact_id": id, "status": "queued"}, nil
}
func BuildElementUpdate(raw []any, sources []any, prompt, lang string, settings map[string]int) ([]any, []string, error) {
	value := func(k string, d int) int {
		if v, ok := settings[k]; ok {
			return v
		}
		return d
	}
	var p any = prompt
	if prompt == "" {
		p = nil
	}
	update := make([]any, 10)
	update[0] = StringAt(raw, 0)
	update[3] = sources
	var mask []string
	switch At(raw, 2) {
	case float64(1):
		update = append(update[:6], []any{nil, []any{p, 2, nil, nil, lang, nil, value("audio_format", 2)}})
		mask = []string{"audio_overview.generation_options.episode_focus", "audio_overview.generation_options.episode_length", "audio_overview.generation_options.language_code", "audio_overview.generation_options.show_format", "sources"}
	case float64(3):
		format := value("video_format", 3)
		opts := []any{nil, lang, p, nil, format}
		mask = []string{"explainer_video.generation_options.language_code", "explainer_video.generation_options.video_focus", "explainer_video.generation_options.template_format", "sources"}
		if format != 4 {
			opts = append(opts, 1)
			mask = append(mask[:3], "explainer_video.generation_options.video_overview_style", "sources")
		}
		update = append(update[:8], []any{nil, nil, opts})
	case float64(7):
		update = make([]any, 15)
		update[0] = StringAt(raw, 0)
		update[3] = sources
		update[14] = []any{[]any{p, lang, nil, value("orientation", 1), value("detail_level", 2), value("infographic_style", 1)}}
		mask = []string{"infographic.generation_options.user_steering_prompt", "infographic.generation_options.language_code", "infographic.generation_options.aspect_ratio", "infographic.generation_options.information_density", "infographic.generation_options.style", "sources"}
	case float64(8):
		update = make([]any, 17)
		update[0] = StringAt(raw, 0)
		update[3] = sources
		update[16] = []any{[]any{p, lang, value("slide_format", 1), value("slide_length", 3)}}
		mask = []string{"slides.generation_options.user_steering_prompt", "slides.generation_options.language_code", "slides.generation_options.deck_type", "slides.generation_options.length", "sources"}
	case float64(4):
		sub := At(raw, 9, 1, 0)
		opts := []any{sub, nil, p, lang}
		mask = []string{"app.generation_options.free_text_steering_prompt", "app.generation_options.language_code", "sources"}
		if sub == float64(2) {
			opts = append(opts, nil, nil, nil, []any{value("amount", 2), value("difficulty", 2)})
			mask = []string{mask[0], mask[1], "app.generation_options.quiz_generation_options.question_quantity", "app.generation_options.quiz_generation_options.quiz_difficulty", "sources"}
		}
		if sub == float64(1) {
			opts = append(opts, nil, nil, []any{value("amount", 1), value("difficulty", 2)})
			mask = []string{mask[0], mask[1], "app.generation_options.flashcards_generation_options.card_quantity", "app.generation_options.flashcards_generation_options.flashcards_difficulty", "sources"}
		}
		update[9] = []any{nil, opts}
	default:
		return nil, nil, fmt.Errorf("unsupported report element type %v", At(raw, 2))
	}
	return update, mask, nil
}
