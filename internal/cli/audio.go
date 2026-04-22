package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/types"
)

var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Generate an audio podcast from source material",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			source, err := buildSource(cmd)
			if err != nil { return err }
			outputDir, _ := cmd.Flags().GetString("output")
			if outputDir == "" { outputDir = "." }
			language, _ := cmd.Flags().GetString("language")
			instructions, _ := cmd.Flags().GetString("instructions")
			format, _ := cmd.Flags().GetString("format")
			length, _ := cmd.Flags().GetString("length")

			result, err := c.RunAudioOverview(ctx, types.AudioOverviewOptions{
				Source: source, OutputDir: outputDir,
				Language: types.AudioLanguage(language), Instructions: instructions,
				Format: types.AudioStyleFormat(format), Length: types.AudioLength(length),
			}, progressLogger)
			if err != nil { return err }
			fmt.Println(result.AudioPath)
			fmt.Fprintln(os.Stderr, "Notebook:", result.NotebookURL)
			return nil
		})
	},
}

func init() {
	addTransportFlags(audioCmd)
	addSourceFlags(audioCmd)
	audioCmd.Flags().StringP("output", "o", ".", "Output directory")
	audioCmd.Flags().StringP("language", "l", "", "Audio language")
	audioCmd.Flags().String("instructions", "", "Custom instructions")
	audioCmd.Flags().String("format", "", "Audio format: deep_dive, brief, critique, debate")
	audioCmd.Flags().String("length", "", "Audio length: short, default, long")
}
