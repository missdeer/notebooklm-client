package payload

import "github.com/missdeer/notebooklm-client/internal/types"

var AudioFormatCode = map[types.AudioStyleFormat]int{
	"deep_dive": 1, "brief": 2, "critique": 3, "debate": 4,
}

var AudioLengthCode = map[types.AudioLength]int{
	"short": 1, "default": 2, "long": 3,
}

var VideoFormatCode = map[types.VideoFormat]int{
	"explainer": 1, "brief": 2, "cinematic": 3,
}

var VideoStyleCode = map[types.VideoStyle]int{
	"auto": 1, "classic": 3, "whiteboard": 4, "kawaii": 5,
	"anime": 6, "watercolor": 7, "retro_print": 8,
}

var QuizQuantityCode = map[types.QuizQuantity]int{
	"fewer": 1, "standard": 2,
}

var QuizDifficultyCode = map[types.QuizDifficulty]int{
	"easy": 1, "medium": 2, "hard": 3,
}

var InfographicOrientationCode = map[types.InfographicOrientation]int{
	"landscape": 1, "portrait": 2, "square": 3,
}

var InfographicDetailCode = map[types.InfographicDetail]int{
	"concise": 1, "standard": 2, "detailed": 3,
}

var InfographicStyleCode = map[types.InfographicStyle]int{
	"sketch_note": 2, "professional": 3, "bento_grid": 4,
}

var SlideFormatCode = map[types.SlideDeckFormat]int{
	"detailed": 1, "presenter": 2,
}

var SlideLengthCode = map[types.SlideDeckLength]int{
	"default": 1, "short": 2,
}

type ReportTemplateInfo struct {
	Title       string
	Description string
	Prompt      string
}

var ReportTemplates = map[types.ReportTemplate]ReportTemplateInfo{
	"briefing_doc": {
		Title:       "Briefing Doc",
		Description: "Key insights and important quotes",
		Prompt:      "Create a comprehensive briefing document that includes an Executive Summary, detailed analysis of key themes, important quotes with context, and actionable insights.",
	},
	"study_guide": {
		Title:       "Study Guide",
		Description: "Short-answer quiz, essay questions, glossary",
		Prompt:      "Create a comprehensive study guide that includes key concepts, short-answer practice questions, essay prompts for deeper exploration, and a glossary of important terms.",
	},
	"blog_post": {
		Title:       "Blog Post",
		Description: "Insightful takeaways in readable article format",
		Prompt:      "Write an engaging blog post that presents the key insights in an accessible, reader-friendly format. Include an attention-grabbing introduction, well-organized sections, and a compelling conclusion with takeaways.",
	},
	"custom": {
		Title:       "Custom Report",
		Description: "Custom format",
		Prompt:      "Create a report based on the provided sources.",
	},
}

func nilOrInt(m map[types.AudioStyleFormat]int, k types.AudioStyleFormat) any {
	if k == "" {
		return nil
	}
	if v, ok := m[k]; ok {
		return v
	}
	return nil
}

func intOrNil[K ~string](m map[K]int, k K) any {
	if k == "" {
		return nil
	}
	if v, ok := m[k]; ok {
		return v
	}
	return nil
}

func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func BuildAudioPayload(sidsTriple, sidsDouble any, opts types.AudioArtifactOptions) []any {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	formatCode := intOrNil(AudioFormatCode, opts.Format)
	if formatCode == nil {
		formatCode = AudioFormatCode["deep_dive"]
	}

	return []any{
		nil, nil, 1, sidsTriple, nil, nil,
		[]any{nil, []any{strOrNil(opts.Instructions), intOrNil(AudioLengthCode, opts.Length), nil, sidsDouble, lang, nil, formatCode}},
	}
}

func BuildReportPayload(sidsTriple, sidsDouble any, opts types.ReportArtifactOptions) []any {
	template := opts.Template
	if template == "" {
		template = "briefing_doc"
	}
	tmpl := ReportTemplates[template]
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}

	var prompt string
	if template == "custom" {
		prompt = opts.Instructions
		if prompt == "" {
			prompt = tmpl.Prompt
		}
	} else {
		if opts.Instructions != "" {
			prompt = tmpl.Prompt + "\n\n" + opts.Instructions
		} else {
			prompt = tmpl.Prompt
		}
	}

	return []any{
		nil, nil, 2, sidsTriple, nil, nil, nil,
		[]any{nil, []any{tmpl.Title, tmpl.Description, nil, sidsDouble, lang, prompt, nil, true}},
	}
}

func BuildVideoPayload(sidsTriple, sidsDouble any, opts types.VideoArtifactOptions) []any {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	formatCode := intOrNil(VideoFormatCode, opts.Format)
	var styleCode any
	if opts.Format != "cinematic" {
		styleCode = intOrNil(VideoStyleCode, opts.Style)
	}

	return []any{
		nil, nil, 3, sidsTriple, nil, nil, nil, nil,
		[]any{nil, nil, []any{sidsDouble, lang, strOrNil(opts.Instructions), nil, formatCode, styleCode}},
	}
}

func BuildQuizPayload(sidsTriple any, opts types.QuizArtifactOptions) []any {
	return []any{
		nil, nil, 4, sidsTriple, nil, nil, nil, nil, nil,
		[]any{nil, []any{2, nil, strOrNil(opts.Instructions), strOrNil(opts.Language), nil, nil, nil,
			[]any{intOrNil(QuizQuantityCode, opts.Quantity), intOrNil(QuizDifficultyCode, opts.Difficulty)}}},
	}
}

func BuildFlashcardsPayload(sidsTriple any, opts types.FlashcardsArtifactOptions) []any {
	return []any{
		nil, nil, 4, sidsTriple, nil, nil, nil, nil, nil,
		[]any{nil, []any{1, nil, strOrNil(opts.Instructions), strOrNil(opts.Language), nil, nil,
			[]any{intOrNil(QuizDifficultyCode, opts.Difficulty), intOrNil(QuizQuantityCode, opts.Quantity)}}},
	}
}

func BuildInfographicPayload(sidsTriple any, opts types.InfographicArtifactOptions) []any {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	return []any{
		nil, nil, 7, sidsTriple,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		[]any{[]any{strOrNil(opts.Instructions), lang, nil,
			intOrNil(InfographicOrientationCode, opts.Orientation),
			intOrNil(InfographicDetailCode, opts.Detail),
			intOrNil(InfographicStyleCode, opts.Style)}},
	}
}

func BuildSlideDeckPayload(sidsTriple any, opts types.SlideDeckArtifactOptions) []any {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	return []any{
		nil, nil, 8, sidsTriple,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		[]any{[]any{strOrNil(opts.Instructions), lang,
			intOrNil(SlideFormatCode, opts.Format),
			intOrNil(SlideLengthCode, opts.Length)}},
	}
}

func BuildDataTablePayload(sidsTriple any, opts types.DataTableArtifactOptions) []any {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	return []any{
		nil, nil, 9, sidsTriple,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		[]any{nil, []any{strOrNil(opts.Instructions), lang}},
	}
}

func BuildArtifactPayload(sidsTriple, sidsDouble any, opts types.ArtifactOption) []any {
	switch o := opts.(type) {
	case types.AudioArtifactOptions:
		return BuildAudioPayload(sidsTriple, sidsDouble, o)
	case types.ReportArtifactOptions:
		return BuildReportPayload(sidsTriple, sidsDouble, o)
	case types.VideoArtifactOptions:
		return BuildVideoPayload(sidsTriple, sidsDouble, o)
	case types.QuizArtifactOptions:
		return BuildQuizPayload(sidsTriple, o)
	case types.FlashcardsArtifactOptions:
		return BuildFlashcardsPayload(sidsTriple, o)
	case types.InfographicArtifactOptions:
		return BuildInfographicPayload(sidsTriple, o)
	case types.SlideDeckArtifactOptions:
		return BuildSlideDeckPayload(sidsTriple, o)
	case types.DataTableArtifactOptions:
		return BuildDataTablePayload(sidsTriple, o)
	default:
		return nil
	}
}
