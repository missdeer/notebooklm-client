package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

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

var sourceListCmd = &cobra.Command{
	Use:   "list <notebook-id>",
	Short: "List sources in a notebook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			_, sources, err := c.GetNotebookDetail(ctx, args[0])
			if err != nil {
				return err
			}
			if len(sources) == 0 {
				fmt.Fprintln(os.Stderr, "No sources")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tWORDS")
			for _, s := range sources {
				wc := "-"
				if s.WordCount != nil {
					wc = fmt.Sprintf("%d", *s.WordCount)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.ID, s.Title, wc)
			}
			return w.Flush()
		})
	},
}

var sourceDeleteCmd = &cobra.Command{
	Use:   "delete <source-id> [source-ids...]",
	Short: "Delete source(s) from any notebook",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			for _, id := range args {
				if err := c.DeleteSource(ctx, id); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to delete %s: %v\n", id, err)
				} else {
					fmt.Fprintf(os.Stderr, "Deleted %s\n", id)
				}
			}
			return nil
		})
	},
}

var sourceRenameCmd = &cobra.Command{
	Use:   "rename <notebook-id> <source-id> <new-title>",
	Short: "Rename a source",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			if err := c.RenameSource(ctx, args[0], args[1], args[2]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Renamed %s\n", args[1])
			return nil
		})
	},
}

var sourceRefreshCmd = &cobra.Command{
	Use:   "refresh <notebook-id> <source-id>",
	Short: "Refresh source data (re-fetch URL/Drive content)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			if err := c.RefreshSource(ctx, args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Refresh requested for %s\n", args[1])
			return nil
		})
	},
}

var sourceSummaryCmd = &cobra.Command{
	Use:   "summary <source-id>",
	Short: "Print AI-generated summary of a source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			summary, err := c.GetSourceSummary(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Println(summary)
			return nil
		})
	},
}

func init() {
	subs := []*cobra.Command{sourceAddCmd, sourceListCmd, sourceDeleteCmd, sourceRenameCmd, sourceRefreshCmd, sourceSummaryCmd}
	for _, sc := range subs {
		sourceCmd.AddCommand(sc)
		addTransportFlags(sc)
	}
	sourceAddCmd.Flags().String("url", "", "URL to add")
	sourceAddCmd.Flags().String("text", "", "Text content to add")
	sourceAddCmd.Flags().String("file", "", "File to upload")
	sourceAddCmd.Flags().String("title", "", "Title for text source")
}
