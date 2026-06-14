package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Manage notebook sharing",
}

var shareStatusCmd = &cobra.Command{
	Use:   "status <notebook-id>",
	Short: "Show sharing settings and collaborators",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			status, err := c.GetShareStatus(ctx, args[0])
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(status)
		})
	},
}

var sharePublicCmd = &cobra.Command{
	Use:   "public <notebook-id>",
	Short: "Enable or disable public link sharing",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			enable, _ := cmd.Flags().GetBool("enable")
			disable, _ := cmd.Flags().GetBool("disable")
			if enable == disable {
				return fmt.Errorf("exactly one of --enable or --disable is required")
			}
			if err := c.ShareNotebookPublic(ctx, args[0], enable); err != nil {
				return err
			}
			if enable {
				fmt.Fprintf(os.Stderr, "Public link enabled for %s\n", args[0])
			} else {
				fmt.Fprintf(os.Stderr, "Public link disabled for %s\n", args[0])
			}
			return nil
		})
	},
}

var shareInviteCmd = &cobra.Command{
	Use:   "invite <notebook-id>",
	Short: "Invite a collaborator by email",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			email, _ := cmd.Flags().GetString("email")
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			permission, _ := cmd.Flags().GetString("permission")
			message, _ := cmd.Flags().GetString("message")
			notify, _ := cmd.Flags().GetBool("notify")
			if err := c.ShareNotebookWithUser(ctx, args[0], email, permission, notify, message); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Invited %s (%s)\n", email, permission)
			return nil
		})
	},
}

func init() {
	subs := []*cobra.Command{shareStatusCmd, sharePublicCmd, shareInviteCmd}
	for _, sc := range subs {
		shareCmd.AddCommand(sc)
		addTransportFlags(sc)
	}
	sharePublicCmd.Flags().Bool("enable", false, "Enable public link")
	sharePublicCmd.Flags().Bool("disable", false, "Disable public link")
	shareInviteCmd.Flags().String("email", "", "Collaborator email")
	shareInviteCmd.Flags().String("permission", "viewer", "Permission: viewer, editor")
	shareInviteCmd.Flags().String("message", "", "Optional invite message")
	shareInviteCmd.Flags().Bool("notify", true, "Send email notification")
}
