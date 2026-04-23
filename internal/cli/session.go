package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

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

		sess, err := parseImportedSession(data)
		if err != nil {
			return err
		}

		path, err := session.Save(sess, "")
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Session imported to %s\n", path)
		return nil
	},
}

// parseImportedSession accepts either the wrapped StoredSession envelope
// (`{"version":N,"session":{...}}`, written by export-session) or a raw
// NotebookRpcSession (`{"at":"...","bl":"...",...}`, as produced by the
// TypeScript client's session.json).
//
// It's important to attempt the wrapped form FIRST: Go's json.Unmarshal
// ignores unknown top-level keys, so unmarshalling a wrapped payload into
// NotebookRpcSession succeeds silently with every field empty — the fallback
// never fires. Probing StoredSession first (and accepting it only when its
// embedded Session has an `at` token) avoids that trap.
func parseImportedSession(data []byte) (types.NotebookRpcSession, error) {
	var stored session.StoredSession
	if err := json.Unmarshal(data, &stored); err == nil && stored.Session.AT != "" {
		return stored.Session, nil
	}

	var sess types.NotebookRpcSession
	if err := json.Unmarshal(data, &sess); err != nil {
		return types.NotebookRpcSession{}, fmt.Errorf("invalid session JSON: %w", err)
	}
	if sess.AT == "" {
		return types.NotebookRpcSession{}, fmt.Errorf("session missing 'at' token")
	}
	return sess, nil
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

// criticalCookies are the Google account cookies most relevant for keeping a
// NotebookLM session alive. Order reflects display priority.
var criticalCookies = []string{
	"__Secure-1PSID", "__Secure-3PSID",
	"__Secure-1PSIDTS", "__Secure-3PSIDTS",
	"__Secure-1PSIDCC", "__Secure-3PSIDCC",
	"SID", "HSID", "SSID", "APISID", "SAPISID",
	"SIDCC", "NID",
}

var sessionStatusCmd = &cobra.Command{
	Use:   "session-status",
	Short: "Show cookie expirations and remaining validity for the saved session",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionPath, _ := cmd.Flags().GetString("session-path")
		all, _ := cmd.Flags().GetBool("all")

		sess, err := session.Load(sessionPath)
		if err != nil {
			return err
		}
		if sess == nil {
			return fmt.Errorf("no session found. Run export-session first")
		}

		printSessionStatus(os.Stdout, *sess, all, time.Now().UTC())
		return nil
	},
}

func printSessionStatus(out io.Writer, sess types.NotebookRpcSession, all bool, now time.Time) {

	fmt.Fprintf(out, "Tokens:\n")
	fmt.Fprintf(out, "  at   : %s\n", truncate(sess.AT, 24))
	fmt.Fprintf(out, "  bl   : %s\n", truncate(sess.BL, 24))
	fmt.Fprintf(out, "  fsid : %s\n", truncate(sess.FSID, 24))
	fmt.Fprintln(out)

	if len(sess.CookieJar) == 0 {
		fmt.Fprintln(out, "No structured cookies stored (legacy session). Re-run export-session to capture expiration data.")
		return
	}

	byName := make(map[string]types.SessionCookie, len(sess.CookieJar))
	for _, c := range sess.CookieJar {
		if _, ok := byName[c.Name]; !ok {
			byName[c.Name] = c
		}
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "COOKIE\tDOMAIN\tEXPIRES (UTC)\tREMAINING")

	shown := make(map[string]bool)
	anyMissing := false
	for _, name := range criticalCookies {
		c, ok := byName[name]
		if !ok {
			fmt.Fprintf(w, "%s\t-\t(not present)\t-\n", name)
			anyMissing = true
			continue
		}
		fmt.Fprintln(w, formatCookieRow(c, now))
		shown[name] = true
	}

	if all {
		// Stable ordering of remaining cookies by name.
		others := make([]types.SessionCookie, 0)
		for _, c := range sess.CookieJar {
			if !shown[c.Name] {
				others = append(others, c)
			}
		}
		sort.Slice(others, func(i, j int) bool { return others[i].Name < others[j].Name })
		for _, c := range others {
			fmt.Fprintln(w, formatCookieRow(c, now))
		}
	}
	w.Flush()

	if anyMissing {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Note: one or more critical cookies are missing. If refresh-session fails, re-run export-session.")
	}
	if !all {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Pass --all to list every cookie in the session.")
	}
}

func formatCookieRow(c types.SessionCookie, now time.Time) string {
	var expires, remaining string
	switch {
	case c.Session:
		expires = "(session)"
		remaining = "until browser exit"
	case c.Expires == nil:
		expires = "(unknown)"
		remaining = "legacy session"
	default:
		expires = c.Expires.Format("2006-01-02 15:04:05")
		d := c.Expires.Sub(now)
		if d <= 0 {
			remaining = "EXPIRED"
		} else {
			remaining = humanDuration(d)
		}
	}
	return strings.Join([]string{c.Name, c.Domain, expires, remaining}, "\t")
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) - days*24
	if days < 30 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	months := days / 30
	rem := days - months*30
	return fmt.Sprintf("~%dmo%dd", months, rem)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func init() {
	addTransportFlags(exportSessionCmd)
	exportSessionCmd.Flags().StringP("output", "o", "", "Output path for session file")
	refreshSessionCmd.Flags().String("session-path", "", "Session file path")
	sessionStatusCmd.Flags().String("session-path", "", "Session file path")
	sessionStatusCmd.Flags().Bool("all", false, "List every cookie, not just the critical ones")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
