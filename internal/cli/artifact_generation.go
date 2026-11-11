package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/parser"
	"github.com/missdeer/notebooklm-client/internal/payload"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func createArtifact(ctx context.Context, c *client.NotebookClient, in commandInput, nb string) (any, error) {
	ids, err := notebookSources(ctx, c, nb, in.val("source_ids"))
	if err != nil {
		return nil, err
	}
	kind := strings.TrimSuffix(in.schema.Canonical, " create")
	prompt := in.val("focus", "prompt", "instructions", "description")
	lang := in.val("language")
	if lang == "" {
		lang = "en"
	}
	if kind == "mindmap" {
		return api.CreateMindMap(ctx, c.CallBatchExecute, nb, in.val("title"), ids)
	}
	if kind == "report" && in.val("format") == "Interactive" {
		if t := in.val("template"); t != "" && t != "learning_overview" {
			return nil, fmt.Errorf("unsupported interactive report template %s", t)
		}
		return api.CreateInteractiveReport(ctx, c.CallBatchExecute, nb, prompt, lang, ids)
	}
	opts, err := artifactOptions(kind, in, prompt, lang)
	if err != nil {
		return nil, err
	}
	nested, simple := []any{}, []any{}
	for _, sid := range ids {
		nested = append(nested, []any{[]any{sid}})
		simple = append(simple, []any{sid})
	}
	content := payload.BuildArtifactPayload(nested, simple, opts)
	if kind == "slides" && in.val("length") == "default" {
		content[16].([]any)[0].([]any)[3] = 3
	}
	if kind == "quiz" {
		count := flagInt(in.cmd, "count", 2)
		difficulty := flagInt(in.cmd, "difficulty", 2)
		if count < 1 || difficulty < 1 || difficulty > 5 {
			return nil, fmt.Errorf("quiz count must be positive and difficulty between 1 and 5")
		}
		content[9].([]any)[1].([]any)[7] = []any{count, difficulty}
	}
	raw, err := c.CallBatchExecute(ctx, rpc.GenerateArtifact, []any{[]any{2}, nb, content}, "/notebook/"+nb)
	id, title := parser.ParseGenerateArtifact(raw)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("artifact generation returned no ID")
	}
	return map[string]string{"artifact_id": id, "title": title, "notebook_id": nb, "status": "queued"}, nil
}

func artifactOptions(kind string, in commandInput, prompt, lang string) (types.ArtifactOption, error) {
	format, length := in.val("format"), in.val("length")
	quantity := in.val("quantity", "amount", "count")
	if quantity == "less" {
		quantity = "fewer"
	}
	switch kind {
	case "audio":
		if format != "" && payload.AudioFormatCode[types.AudioStyleFormat(format)] == 0 {
			return nil, fmt.Errorf("invalid audio format %s", format)
		}
		if length != "" && payload.AudioLengthCode[types.AudioLength(length)] == 0 {
			return nil, fmt.Errorf("invalid audio length %s", length)
		}
		return types.AudioArtifactOptions{Instructions: prompt, Language: lang, Format: types.AudioStyleFormat(format), Length: types.AudioLength(length)}, nil
	case "video":
		if format != "" && payload.VideoFormatCode[types.VideoFormat(format)] == 0 {
			return nil, fmt.Errorf("invalid video format %s", format)
		}
		if style := in.val("style"); style != "" && payload.VideoStyleCode[types.VideoStyle(style)] == 0 {
			return nil, fmt.Errorf("invalid video style %s", style)
		}
		return types.VideoArtifactOptions{Instructions: prompt, StylePrompt: in.val("style_prompt"), Language: lang, Format: types.VideoFormat(format), Style: types.VideoStyle(in.val("style"))}, nil
	case "quiz":
		return types.QuizArtifactOptions{Instructions: prompt, Language: lang, Quantity: types.QuizQuantity(quantity), Difficulty: types.QuizDifficulty(in.val("difficulty"))}, nil
	case "flashcards":
		return types.FlashcardsArtifactOptions{Instructions: prompt, Language: lang, Quantity: types.QuizQuantity(quantity), Difficulty: types.QuizDifficulty(in.val("difficulty"))}, nil
	case "report":
		if format == "Create Your Own" && strings.TrimSpace(prompt) == "" {
			return nil, fmt.Errorf("custom report requires --prompt")
		}
		template := map[string]types.ReportTemplate{"Briefing Doc": "briefing_doc", "Study Guide": "study_guide", "Blog Post": "blog_post", "Create Your Own": "custom", "": "briefing_doc"}[format]
		if template == "" {
			return nil, fmt.Errorf("unsupported report format %s", format)
		}
		return types.ReportArtifactOptions{Template: template, Instructions: prompt, Language: lang}, nil
	case "slides":
		if format == "detailed_deck" {
			format = "detailed"
		}
		if format == "presenter_slides" {
			format = "presenter"
		}
		return types.SlideDeckArtifactOptions{Instructions: prompt, Language: lang, Format: types.SlideDeckFormat(format), Length: types.SlideDeckLength(length)}, nil
	case "infographic":
		if style := in.val("style"); style != "" && payload.InfographicStyleCode[types.InfographicStyle(style)] == 0 {
			return nil, fmt.Errorf("invalid infographic style %s", style)
		}
		return types.InfographicArtifactOptions{Instructions: prompt, Language: lang, Orientation: types.InfographicOrientation(in.val("orientation")), Detail: types.InfographicDetail(in.val("detail", "detail_level")), Style: types.InfographicStyle(in.val("style"))}, nil
	case "data-table":
		return types.DataTableArtifactOptions{Instructions: prompt, Language: lang}, nil
	}
	return nil, fmt.Errorf("unsupported artifact kind %s", kind)
}

func reviseSlides(ctx context.Context, c *client.NotebookClient, in commandInput) (any, error) {
	specs := flagStrings(in.cmd, "slide")
	if len(specs) == 0 {
		specs = flagStrings(in.cmd, "revision")
	}
	revisions := []api.SlideRevision{}
	for _, s := range specs {
		index, instruction, ok := strings.Cut(s, ":")
		if !ok {
			parts := strings.SplitN(strings.TrimSpace(s), " ", 2)
			if len(parts) == 2 {
				index, instruction, ok = parts[0], parts[1], true
			}
		}
		n, err := strconv.Atoi(index)
		if !ok || err != nil || n < 1 || strings.TrimSpace(instruction) == "" {
			return nil, fmt.Errorf("invalid revision %q; expected index:instruction", s)
		}
		revisions = append(revisions, api.SlideRevision{Index: n - 1, Instruction: instruction})
	}
	if len(revisions) == 0 {
		return nil, fmt.Errorf("at least one --slide index:instruction is required")
	}
	id, title, err := c.ReviseSlideDeck(ctx, in.val("artifact_id"), revisions)
	return map[string]string{"artifact_id": id, "title": title}, err
}
