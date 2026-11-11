package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/spf13/cobra"
)

var slidesCmd = &cobra.Command{
	Use:   "slides",
	Short: "Generate a slide deck from new source material or an existing notebook",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := slideDeckOptions(cmd)
		if err != nil {
			return err
		}
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			result, err := c.RunSlideDeck(ctx, opts, progressLogger)
			if err != nil {
				return err
			}
			fmt.Println(result.PptxPath)
			if result.PdfPath != "" {
				fmt.Println(result.PdfPath)
			}
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

func slideDeckOptions(cmd *cobra.Command) (types.SlideDeckOptions, error) {
	var opts types.SlideDeckOptions
	opts.NotebookID, _ = cmd.Flags().GetString("notebook-id")
	opts.NotebookID = strings.TrimSpace(opts.NotebookID)
	if cmd.Flags().Changed("notebook-id") && opts.NotebookID == "" {
		return opts, fmt.Errorf("--notebook-id must not be empty")
	}
	if opts.NotebookID != "" {
		for _, name := range []string{"url", "text", "file", "topic"} {
			if cmd.Flags().Changed(name) {
				return opts, fmt.Errorf("--notebook-id cannot be combined with --%s", name)
			}
		}
	} else {
		if cmd.Flags().Changed("source-ids") {
			return opts, fmt.Errorf("--source-ids requires --notebook-id")
		}
		var err error
		opts.Source, err = buildSource(cmd)
		if err != nil {
			return opts, err
		}
	}
	if cmd.Flags().Changed("source-ids") {
		ids, _ := cmd.Flags().GetString("source-ids")
		for _, id := range strings.Split(ids, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				return opts, fmt.Errorf("--source-ids must contain non-empty comma-separated IDs")
			}
			opts.SourceIDs = append(opts.SourceIDs, id)
		}
	}
	opts.OutputDir, _ = cmd.Flags().GetString("output")
	opts.Instructions, _ = cmd.Flags().GetString("instructions")
	opts.Language, _ = cmd.Flags().GetString("language")
	format, _ := cmd.Flags().GetString("format")
	length, _ := cmd.Flags().GetString("length")
	opts.Format, opts.Length = types.SlideDeckFormat(format), types.SlideDeckLength(length)
	return opts, nil
}

func init() {
	slidesCmd.Flags().String("notebook-id", "", "Existing notebook ID (uses all sources unless --source-ids is set)")
	slidesCmd.Flags().String("source-ids", "", "Comma-separated existing source IDs (requires --notebook-id)")
	slidesCmd.Flags().String("format", "", "Slide format: detailed, presenter")
	slidesCmd.Flags().String("length", "", "Slide length: default, short")
}
