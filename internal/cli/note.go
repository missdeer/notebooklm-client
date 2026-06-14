package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
)

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage notebook notes",
}

var noteListCmd = &cobra.Command{
	Use:   "list <notebook-id>",
	Short: "List notes in a notebook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			notes, err := c.ListNotes(ctx, args[0])
			if err != nil {
				return err
			}
			if len(notes) == 0 {
				fmt.Fprintln(os.Stderr, "No notes")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tCHARS")
			for _, n := range notes {
				title := n.Title
				if title == "" {
					title = "(untitled)"
				}
				fmt.Fprintf(w, "%s\t%s\t%d\n", n.ID, title, len(n.Content))
			}
			return w.Flush()
		})
	},
}

var noteCreateCmd = &cobra.Command{
	Use:   "create <notebook-id>",
	Short: "Create a new note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			title, _ := cmd.Flags().GetString("title")
			content, _ := cmd.Flags().GetString("content")
			id, err := c.CreateNote(ctx, args[0], title, content)
			if err != nil {
				return err
			}
			fmt.Println(id)
			return nil
		})
	},
}

var noteUpdateCmd = &cobra.Command{
	Use:   "update <notebook-id> <note-id>",
	Short: "Update note content/title",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			title, _ := cmd.Flags().GetString("title")
			content, _ := cmd.Flags().GetString("content")
			if title == "" && content == "" {
				return fmt.Errorf("at least one of --title or --content is required")
			}
			if err := c.UpdateNote(ctx, args[0], args[1], content, title); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Updated %s\n", args[1])
			return nil
		})
	},
}

var noteDeleteCmd = &cobra.Command{
	Use:   "delete <notebook-id> <note-id> [note-ids...]",
	Short: "Delete note(s)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			notebookID := args[0]
			for _, id := range args[1:] {
				if err := c.DeleteNote(ctx, notebookID, id); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to delete %s: %v\n", id, err)
				} else {
					fmt.Fprintf(os.Stderr, "Deleted %s\n", id)
				}
			}
			return nil
		})
	},
}

func init() {
	subs := []*cobra.Command{noteListCmd, noteCreateCmd, noteUpdateCmd, noteDeleteCmd}
	for _, sc := range subs {
		noteCmd.AddCommand(sc)
		addTransportFlags(sc)
	}
	noteCreateCmd.Flags().String("title", "", "Note title")
	noteCreateCmd.Flags().String("content", "", "Note content")
	noteUpdateCmd.Flags().String("title", "", "New title")
	noteUpdateCmd.Flags().String("content", "", "New content")
}
