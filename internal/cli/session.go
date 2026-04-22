package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/types"
)

var exportSessionCmd = &cobra.Command{
	Use:   "export-session",
	Short: "Launch browser to log in and export session",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		proxy := resolveProxy(cmd)
		profileDir, _ := cmd.Flags().GetString("profile")
		headless, _ := cmd.Flags().GetBool("headless")
		chromePath, _ := cmd.Flags().GetString("chrome-path")
		output, _ := cmd.Flags().GetString("output")

		c := client.New()
		if err := c.Connect(ctx, client.ConnectOptions{
			Transport:  client.TransportBrowser,
			Proxy:      proxy,
			ProfileDir: profileDir,
			Headless:   headless,
			ChromePath: chromePath,
		}); err != nil {
			return err
		}
		defer c.Disconnect()

		path, err := c.ExportSession(output)
		if err != nil {
			return err
		}
		fmt.Println(path)
		fmt.Fprintln(os.Stderr, "Session exported. You can now use --transport http")
		return nil
	},
}

var importSessionCmd = &cobra.Command{
	Use:   "import-session <json-file-or-string>",
	Short: "Import a session from JSON file or inline string",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		var data []byte
		if _, err := os.Stat(input); err == nil {
			data, err = os.ReadFile(input)
			if err != nil {
				return err
			}
		} else {
			data = []byte(input)
		}

		var sess types.NotebookRpcSession
		if err := json.Unmarshal(data, &sess); err != nil {
			var stored session.StoredSession
			if err2 := json.Unmarshal(data, &stored); err2 != nil {
				return fmt.Errorf("invalid session JSON: %w", err)
			}
			sess = stored.Session
		}

		if sess.AT == "" {
			return fmt.Errorf("session missing 'at' token")
		}

		path, err := session.Save(sess, "")
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Session imported to %s\n", path)
		return nil
	},
}

var refreshSessionCmd = &cobra.Command{
	Use:   "refresh-session",
	Short: "Refresh short-lived tokens using long-lived cookies",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionPath, _ := cmd.Flags().GetString("session-path")
		sess, err := session.Load(sessionPath)
		if err != nil || sess == nil {
			return fmt.Errorf("no session found. Run export-session first")
		}
		refreshed, err := session.RefreshTokens(cmd.Context(), *sess, nil, sessionPath)
		if err != nil {
			return err
		}
		log.Printf("Tokens refreshed (at=%s...)", refreshed.AT[:min(20, len(refreshed.AT))])
		return nil
	},
}

func init() {
	addTransportFlags(exportSessionCmd)
	exportSessionCmd.Flags().StringP("output", "o", "", "Output path for session file")
	refreshSessionCmd.Flags().String("session-path", "", "Session file path")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
