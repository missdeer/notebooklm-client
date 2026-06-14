package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/types"
)

var researchCmd = &cobra.Command{
	Use:   "research",
	Short: "Web/Deep research workflows",
}

var researchStartCmd = &cobra.Command{
	Use:   "start <notebook-id>",
	Short: "Start a web (fast) or deep research run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			query, _ := cmd.Flags().GetString("query")
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			modeStr, _ := cmd.Flags().GetString("mode")
			mode := types.ResearchFast
			if modeStr == "deep" {
				mode = types.ResearchDeep
			}
			researchID, artifactID, err := c.CreateWebSearch(ctx, args[0], query, mode)
			if err != nil {
				return err
			}
			fmt.Printf("researchID=%s\n", researchID)
			if artifactID != "" {
				fmt.Printf("artifactID=%s\n", artifactID)
			}
			return nil
		})
	},
}

var researchStatusCmd = &cobra.Command{
	Use:   "status <notebook-id>",
	Short: "Poll for research results (blocks until ready or timeout)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			results, report, err := c.PollResearchResults(ctx, args[0], 0)
			if err != nil {
				return err
			}
			out := struct {
				Results []types.ResearchResult `json:"results"`
				Report  string                 `json:"report,omitempty"`
			}{results, report}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		})
	},
}

var researchImportCmd = &cobra.Command{
	Use:   "import <notebook-id> <research-id>",
	Short: "Poll and import discovered sources into the notebook",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withClient(cmd, func(ctx context.Context, c *client.NotebookClient) error {
			results, report, err := c.PollResearchResults(ctx, args[0], 0)
			if err != nil {
				return err
			}
			if len(results) == 0 && report == "" {
				return fmt.Errorf("no research results to import (poll timed out)")
			}
			if err := c.ImportResearch(ctx, args[0], args[1], results, report); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Imported %d sources at %s\n", len(results), time.Now().Format(time.RFC3339))
			return nil
		})
	},
}

func init() {
	subs := []*cobra.Command{researchStartCmd, researchStatusCmd, researchImportCmd}
	for _, sc := range subs {
		researchCmd.AddCommand(sc)
		addTransportFlags(sc)
	}
	researchStartCmd.Flags().String("query", "", "Research query")
	researchStartCmd.Flags().String("mode", "fast", "Research mode: fast, deep")
}
