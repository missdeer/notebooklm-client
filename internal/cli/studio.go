package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/missdeer/notebooklm-client/internal/api"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/spf13/cobra"
)

var studioCmd = &cobra.Command{
	Use:   "studio",
	Short: "Manage studio artifacts (status, download, delete, revise, export)",
}

var studioDownloadCmd = &cobra.Command{
	Use:   "download <notebook-id> <artifact-id>",
	Short: "Download an existing artifact without generating it again",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			format, _ := cmd.Flags().GetString("format")
			outputDir, _ := cmd.Flags().GetString("output")
			path, err := c.DownloadArtifact(ctx, args[0], args[1], outputDir, format)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		})
	},
}

var studioStatusCmd = &cobra.Command{
	Use:     "status <notebook-id>",
	Aliases: []string{"list"},
	Short:   "List Studio artifact IDs, types and generation status",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			artifacts, err := c.GetArtifacts(ctx, args[0])
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				if artifacts == nil {
					artifacts = []types.ArtifactInfo{}
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(artifacts)
			}
			return printStudioArtifacts(os.Stdout, artifacts)
		})
	},
}

func printStudioArtifacts(out io.Writer, artifacts []types.ArtifactInfo) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tTITLE")
	labels := map[int]string{1: "audio", 2: "report", 3: "video", 4: "quiz/flashcards", 5: "mind_map", 7: "infographic", 8: "slide_deck", 9: "data_table"}
	statuses := map[int]string{1: "in_progress", 2: "queued", 3: "completed", 4: "failed", 5: "suggested"}
	for _, a := range artifacts {
		label := labels[a.Type]
		if label == "" {
			label = fmt.Sprintf("type:%d", a.Type)
		}
		status := "unknown"
		if a.StatusCode != nil {
			if name, ok := statuses[*a.StatusCode]; ok {
				status = name
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, label, status, a.Title)
	}
	return w.Flush()
}

var studioDeleteCmd = &cobra.Command{
	Use:   "delete <artifact-id> [artifact-ids...]",
	Short: "Delete studio artifact(s) (irreversible)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			for _, id := range args {
				if err := c.DeleteArtifact(ctx, id); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to delete %s: %v\n", id, err)
				} else {
					fmt.Fprintf(os.Stderr, "Deleted %s\n", id)
				}
			}
			return nil
		})
	},
}

var studioReviseCmd = &cobra.Command{
	Use:   "revise <artifact-id>",
	Short: "Revise a slide deck (creates a NEW artifact with the changes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			specs, _ := cmd.Flags().GetStringArray("slide")
			if len(specs) == 0 {
				return fmt.Errorf("at least one --slide \"<index>:<instruction>\" is required")
			}
			revisions := make([]api.SlideRevision, 0, len(specs))
			for _, s := range specs {
				idx := strings.Index(s, ":")
				if idx <= 0 {
					return fmt.Errorf("invalid --slide format %q (want <index>:<instruction>)", s)
				}
				n, err := strconv.Atoi(strings.TrimSpace(s[:idx]))
				if err != nil {
					return fmt.Errorf("invalid slide index in %q: %w", s, err)
				}
				revisions = append(revisions, api.SlideRevision{
					Index:       n,
					Instruction: strings.TrimSpace(s[idx+1:]),
				})
			}
			newID, title, err := c.ReviseSlideDeck(ctx, args[0], revisions)
			if err != nil {
				return err
			}
			fmt.Printf("artifactID=%s\n", newID)
			if title != "" {
				fmt.Printf("title=%s\n", title)
			}
			return nil
		})
	},
}

var studioExportCmd = &cobra.Command{
	Use:   "export <notebook-id> <artifact-id>",
	Short: "Export an artifact to Google Docs or Sheets",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			format, _ := cmd.Flags().GetString("format")
			if format != "docs" && format != "sheets" {
				return fmt.Errorf("--format must be docs or sheets")
			}
			title, _ := cmd.Flags().GetString("title")
			url, err := c.ExportArtifact(ctx, args[0], args[1], title, format)
			if err != nil {
				return err
			}
			if url == "" {
				fmt.Fprintln(os.Stderr, "Export submitted but no URL returned")
				return nil
			}
			fmt.Println(url)
			return nil
		})
	},
}

var notebookDescribeCmd = &cobra.Command{
	Use:   "describe <notebook-id>",
	Short: "Show AI-generated notebook summary and suggested chat prompts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			s, err := c.GetNotebookSummary(ctx, args[0])
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(s)
			}
			fmt.Println(s.Summary)
			if len(s.Topics) > 0 {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Suggested prompts:")
				for _, t := range s.Topics {
					fmt.Fprintf(os.Stderr, "  - %s\n", t.Question)
				}
			}
			return nil
		})
	},
}

var sourceContentCmd = &cobra.Command{
	Use:   "content <source-id>",
	Short: "Print raw indexed text content of a source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			sc, err := c.GetSourceContent(ctx, args[0])
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sc)
			}
			if sc.Title != "" {
				fmt.Fprintf(os.Stderr, "# %s\n", sc.Title)
			}
			if sc.URL != "" {
				fmt.Fprintf(os.Stderr, "# %s\n", sc.URL)
			}
			fmt.Print(sc.Content)
			return nil
		})
	},
}

var chatConfigureCmd = &cobra.Command{
	Use:   "configure <notebook-id>",
	Short: "Configure chat goal/style and response length",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			goal, _ := cmd.Flags().GetString("goal")
			prompt, _ := cmd.Flags().GetString("custom-prompt")
			length, _ := cmd.Flags().GetString("response-length")
			if err := c.ConfigureChat(ctx, args[0], goal, prompt, length); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Chat configured for %s (goal=%s, length=%s)\n", args[0], goal, length)
			return nil
		})
	},
}

func init() {
	subs := []*cobra.Command{studioStatusCmd, studioDownloadCmd, studioDeleteCmd, studioReviseCmd, studioExportCmd}
	for _, sc := range subs {
		studioCmd.AddCommand(sc)
		addTransportFlags(sc)
	}
	studioStatusCmd.Flags().Bool("json", false, "Emit JSON")
	studioDownloadCmd.Flags().String("format", "auto", "Native format: auto, pptx, pdf, m4a, mp4, md, html, png, csv (no conversion)")
	studioDownloadCmd.Flags().StringP("output", "o", ".", "Output directory")
	studioReviseCmd.Flags().StringArray("slide", nil, "Per-slide instruction in <index>:<text> form (repeatable)")
	studioExportCmd.Flags().String("format", "docs", "Export target: docs, sheets")
	studioExportCmd.Flags().String("title", "", "Title for the new document")

	addTransportFlags(notebookDescribeCmd)
	notebookDescribeCmd.Flags().Bool("json", false, "Emit JSON")

	sourceCmd.AddCommand(sourceContentCmd)
	addTransportFlags(sourceContentCmd)
	sourceContentCmd.Flags().Bool("json", false, "Emit JSON (includes title, url, type)")

	chatCmd.AddCommand(chatConfigureCmd)
	addTransportFlags(chatConfigureCmd)
	chatConfigureCmd.Flags().String("goal", "default", "Chat goal: default, custom, learning_guide")
	chatConfigureCmd.Flags().String("custom-prompt", "", "Custom system prompt (required when --goal=custom)")
	chatConfigureCmd.Flags().String("response-length", "default", "Response length: default, longer, shorter")
}
