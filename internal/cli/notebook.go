package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all notebooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			notebooks, err := c.ListNotebooks(ctx)
			if err != nil { return err }
			if len(notebooks) == 0 {
				fmt.Fprintln(os.Stderr, "No notebooks found")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tSOURCES")
			for _, nb := range notebooks {
				sc := "-"
				if nb.SourceCount != nil {
					sc = fmt.Sprintf("%d", *nb.SourceCount)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", nb.ID, nb.Title, sc)
			}
			return w.Flush()
		})
	},
}

var detailCmd = &cobra.Command{
	Use:   "detail <notebook-id>",
	Short: "Show notebook details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			title, sources, err := c.GetNotebookDetail(ctx, args[0])
			if err != nil { return err }
			fmt.Printf("Title: %s\n", title)
			fmt.Printf("Sources: %d\n", len(sources))
			for _, s := range sources {
				wc := 0
				if s.WordCount != nil {
					wc = *s.WordCount
				}
				fmt.Printf("  - %s  %s  (%d words)\n", s.ID, s.Title, wc)
			}
			return nil
		})
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <notebook-id> [notebook-ids...]",
	Short: "Delete notebook(s)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			for _, id := range args {
				if err := c.DeleteNotebook(ctx, id); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to delete %s: %v\n", id, err)
				} else {
					fmt.Fprintf(os.Stderr, "Deleted %s\n", id)
				}
			}
			return nil
		})
	},
}

var chatCmd = &cobra.Command{
	Use:   "chat <notebook-id>",
	Short: "Chat with a notebook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			question, _ := cmd.Flags().GetString("question")
			if question == "" {
				return fmt.Errorf("--question is required")
			}
			_, sources, err := c.GetNotebookDetail(ctx, args[0])
			if err != nil { return err }
			sourceIDs := make([]string, len(sources))
			for i, s := range sources {
				sourceIDs[i] = s.ID
			}
			text, _, err := c.SendChat(ctx, args[0], question, sourceIDs)
			if err != nil { return err }
			fmt.Println(text)
			return nil
		})
	},
}

func init() {
	for _, cmd := range []*cobra.Command{listCmd, detailCmd, deleteCmd, chatCmd} {
		addTransportFlags(cmd)
	}
	chatCmd.Flags().StringP("question", "q", "", "Question to ask")
}
