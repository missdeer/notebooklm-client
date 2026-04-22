package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
)

var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Manage notebook sources",
}

var sourceAddCmd = &cobra.Command{
	Use:   "add <notebook-id>",
	Short: "Add a source to an existing notebook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			notebookID := args[0]
			u, _ := cmd.Flags().GetString("url")
			text, _ := cmd.Flags().GetString("text")
			file, _ := cmd.Flags().GetString("file")
			title, _ := cmd.Flags().GetString("title")

			switch {
			case u != "":
				id, t, err := c.AddURLSource(ctx, notebookID, u)
				if err != nil { return err }
				fmt.Fprintf(os.Stderr, "Added URL source: %s (%s)\n", t, id)
			case file != "":
				id, t, err := c.AddFileSource(ctx, notebookID, file)
				if err != nil { return err }
				fmt.Fprintf(os.Stderr, "Added file source: %s (%s)\n", t, id)
			case text != "":
				if title == "" { title = "Pasted Text" }
				id, t, err := c.AddTextSource(ctx, notebookID, title, text)
				if err != nil { return err }
				fmt.Fprintf(os.Stderr, "Added text source: %s (%s)\n", t, id)
			default:
				return fmt.Errorf("one of --url, --text, or --file is required")
			}
			return nil
		})
	},
}

func init() {
	sourceCmd.AddCommand(sourceAddCmd)
	addTransportFlags(sourceAddCmd)
	sourceAddCmd.Flags().String("url", "", "URL to add")
	sourceAddCmd.Flags().String("text", "", "Text content to add")
	sourceAddCmd.Flags().String("file", "", "File to upload")
	sourceAddCmd.Flags().String("title", "", "Title for text source")
}
