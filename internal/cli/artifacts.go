package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/types"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze source material with a question",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			question, _ := cmd.Flags().GetString("question")
			if question == "" { return fmt.Errorf("--question is required") }

			result, err := c.RunAnalyze(ctx, types.AnalyzeOptions{Source: source, Question: question}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.Answer)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a report from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			template, _ := cmd.Flags().GetString("template")
			instructions, _ := cmd.Flags().GetString("instructions")
			language, _ := cmd.Flags().GetString("language")

			result, err := c.RunReport(ctx, types.ReportOptions{
				Source: source, OutputDir: outputDir,
				Template: types.ReportTemplate(template), Instructions: instructions, Language: language,
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.MarkdownPath)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

var videoCmd = &cobra.Command{
	Use:   "video",
	Short: "Generate a video from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			format, _ := cmd.Flags().GetString("format")
			style, _ := cmd.Flags().GetString("style")
			instructions, _ := cmd.Flags().GetString("instructions")
			language, _ := cmd.Flags().GetString("language")

			result, err := c.RunVideo(ctx, types.VideoOptions{
				Source: source, OutputDir: outputDir,
				Format: types.VideoFormat(format), Style: types.VideoStyle(style),
				Instructions: instructions, Language: language,
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.VideoURL)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

var quizCmd = &cobra.Command{
	Use:   "quiz",
	Short: "Generate a quiz from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			instructions, _ := cmd.Flags().GetString("instructions")
			language, _ := cmd.Flags().GetString("language")
			quantity, _ := cmd.Flags().GetString("quantity")
			difficulty, _ := cmd.Flags().GetString("difficulty")

			result, err := c.RunQuiz(ctx, types.QuizOptions{
				Source: source, OutputDir: outputDir,
				Instructions: instructions, Language: language,
				Quantity: types.QuizQuantity(quantity), Difficulty: types.QuizDifficulty(difficulty),
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.HTMLPath)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

var flashcardsCmd = &cobra.Command{
	Use:   "flashcards",
	Short: "Generate flashcards from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			instructions, _ := cmd.Flags().GetString("instructions")
			language, _ := cmd.Flags().GetString("language")
			quantity, _ := cmd.Flags().GetString("quantity")
			difficulty, _ := cmd.Flags().GetString("difficulty")

			result, err := c.RunFlashcards(ctx, types.FlashcardsOptions{
				Source: source, OutputDir: outputDir,
				Instructions: instructions, Language: language,
				Quantity: types.QuizQuantity(quantity), Difficulty: types.QuizDifficulty(difficulty),
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.HTMLPath)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

var infographicCmd = &cobra.Command{
	Use:   "infographic",
	Short: "Generate an infographic from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			instructions, _ := cmd.Flags().GetString("instructions")
			language, _ := cmd.Flags().GetString("language")
			orientation, _ := cmd.Flags().GetString("orientation")
			detail, _ := cmd.Flags().GetString("detail")
			style, _ := cmd.Flags().GetString("style")

			result, err := c.RunInfographic(ctx, types.InfographicOptions{
				Source: source, OutputDir: outputDir,
				Instructions: instructions, Language: language,
				Orientation: types.InfographicOrientation(orientation),
				Detail: types.InfographicDetail(detail),
				Style: types.InfographicStyle(style),
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.ImagePath)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

var slidesCmd = &cobra.Command{
	Use:   "slides",
	Short: "Generate a slide deck from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			instructions, _ := cmd.Flags().GetString("instructions")
			language, _ := cmd.Flags().GetString("language")
			format, _ := cmd.Flags().GetString("format")
			length, _ := cmd.Flags().GetString("length")

			result, err := c.RunSlideDeck(ctx, types.SlideDeckOptions{
				Source: source, OutputDir: outputDir,
				Instructions: instructions, Language: language,
				Format: types.SlideDeckFormat(format), Length: types.SlideDeckLength(length),
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.PptxPath)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

var dataTableCmd = &cobra.Command{
	Use:   "data-table",
	Short: "Generate a data table from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			instructions, _ := cmd.Flags().GetString("instructions")
			language, _ := cmd.Flags().GetString("language")

			result, err := c.RunDataTable(ctx, types.DataTableOptions{
				Source: source, OutputDir: outputDir,
				Instructions: instructions, Language: language,
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.CsvPath)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

func init() {
	for _, cmd := range []*cobra.Command{analyzeCmd, reportCmd, videoCmd, quizCmd, flashcardsCmd, infographicCmd, slidesCmd, dataTableCmd} {
		addTransportFlags(cmd)
		addSourceFlags(cmd)
		cmd.Flags().StringP("output", "o", ".", "Output directory")
		cmd.Flags().String("instructions", "", "Custom instructions")
		cmd.Flags().String("language", "", "Output language")
	}
	analyzeCmd.Flags().StringP("question", "q", "", "Question to analyze")
	reportCmd.Flags().String("template", "", "Report template: briefing_doc, study_guide, blog_post, custom")
	videoCmd.Flags().String("format", "", "Video format: explainer, brief, cinematic")
	videoCmd.Flags().String("style", "", "Video style: auto, classic, whiteboard, kawaii, anime, watercolor, retro_print")
	quizCmd.Flags().String("quantity", "", "Quiz quantity: fewer, standard")
	quizCmd.Flags().String("difficulty", "", "Quiz difficulty: easy, medium, hard")
	flashcardsCmd.Flags().String("quantity", "", "Quantity: fewer, standard")
	flashcardsCmd.Flags().String("difficulty", "", "Difficulty: easy, medium, hard")
	infographicCmd.Flags().String("orientation", "", "Orientation: landscape, portrait, square")
	infographicCmd.Flags().String("detail", "", "Detail: concise, standard, detailed")
	infographicCmd.Flags().String("style", "", "Style: sketch_note, professional, bento_grid")
	slidesCmd.Flags().String("format", "", "Slide format: detailed, presenter")
	slidesCmd.Flags().String("length", "", "Slide length: default, short")
}
