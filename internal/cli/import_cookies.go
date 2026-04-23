package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/missdeer/notebooklm-client/internal/session"
	"github.com/missdeer/notebooklm-client/internal/transport"
	"github.com/missdeer/notebooklm-client/internal/types"
)

var importCookiesCmd = &cobra.Command{
	Use:   "import-cookies",
	Short: "Import cookies from a Netscape cookies.txt or from Firefox/Safari profile, then bootstrap tokens",
	Long: `Import Google session cookies without launching a browser.

Sources (pick any, or none for auto-scan):
  --file <path>        Netscape cookies.txt (curl/wget format; exported by most
                       browser extensions or by "yt-dlp --cookies-from-browser")
  --browser firefox    Firefox — read cookies.sqlite from every profile found
                       (use --profile <dir> to restrict to one)
  --browser safari     Safari — read Cookies.binarycookies (macOS only)

When neither --file nor --browser is given, every Firefox profile (and Safari
on macOS) in the default locations is scanned and merged. If the same cookie
appears in multiple sources, the version with the most recent timestamp wins
(Firefox lastAccessed / Safari creation date).

After extracting google.com-family cookies, the command immediately contacts
NotebookLM using those cookies to derive the short-lived tokens (at/bl/fsid),
then writes a complete session.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		file, _ := cmd.Flags().GetString("file")
		browser, _ := cmd.Flags().GetString("browser")
		profile, _ := cmd.Flags().GetString("profile")
		userAgent, _ := cmd.Flags().GetString("user-agent")

		if file != "" && browser != "" {
			return fmt.Errorf("--file and --browser are mutually exclusive")
		}

		snapshots, sources, err := gatherSnapshots(file, browser, profile)
		if err != nil {
			return err
		}
		if len(snapshots) == 0 {
			return fmt.Errorf("no cookies found from %s", joinSources(sources))
		}

		// Filter google-family on snapshots BEFORE reconciling so we don't
		// waste time on unrelated cookies.
		googleSnapshots := snapshots[:0]
		for _, s := range snapshots {
			if session.IsGoogleDomain(s.Cookie.Domain) {
				googleSnapshots = append(googleSnapshots, s)
			}
		}

		filtered := session.ReconcileSnapshots(googleSnapshots)
		if len(filtered) == 0 {
			return fmt.Errorf("no google.com cookies found across: %s", joinSources(sources))
		}

		// Report what happened.
		fmt.Fprintf(os.Stderr, "Scanned %s.\n", joinSources(sources))
		fmt.Fprintf(os.Stderr, "Kept %d google-domain cookies after reconciliation.\n", len(filtered))
		if len(sources) > 1 {
			winners := session.WinningSources(googleSnapshots)
			// Summarize per source.
			counts := make(map[string]int)
			for _, src := range winners {
				counts[src]++
			}
			for src, n := range counts {
				fmt.Fprintf(os.Stderr, "  %d cookies from %s\n", n, src)
			}
		}

		sess := types.NotebookRpcSession{
			CookieJar: filtered,
			Cookies:   session.FlattenCookies(filtered),
			UserAgent: userAgent,
		}

		fmt.Fprintln(os.Stderr, "Bootstrapping tokens via NotebookLM dashboard...")
		proxy := resolveProxy(cmd)
		httpClient := transport.NewProxyHTTPClient(proxy)
		refreshed, err := session.RefreshTokens(ctx, sess, httpClient, "")
		if err != nil {
			return fmt.Errorf("token bootstrap failed (cookies may be invalid or expired): %w", err)
		}

		savePath, err := session.Save(*refreshed, "")
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Session written to %s\n", savePath)
		return nil
	},
}

// gatherSnapshots resolves the source selection into one or more cookie
// batches (each with per-cookie timestamps). The second return value is a
// short label list for each source actually consulted (used for logging).
func gatherSnapshots(file, browser, profile string) ([]session.CookieSnapshot, []string, error) {
	switch {
	case file != "":
		f, err := os.Open(file)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()
		return readNetscapeAsSnapshots(f, file)

	case browser == "firefox":
		if profile != "" {
			snaps, err := session.ReadFirefoxSnapshots(profile)
			if err != nil {
				return nil, nil, err
			}
			return snaps, []string{"firefox(" + filepath.Base(profile) + ")"}, nil
		}
		return readAllFirefoxSnapshots()

	case browser == "safari":
		if runtime.GOOS != "darwin" {
			return nil, nil, fmt.Errorf("safari cookies can only be read on macOS")
		}
		return readSafariSnapshots(profile)

	case browser != "":
		return nil, nil, fmt.Errorf("unknown --browser value %q (expected firefox or safari)", browser)

	default:
		// Auto-discover everything.
		var all []session.CookieSnapshot
		var srcs []string

		ffSnaps, ffSrcs, err := readAllFirefoxSnapshots()
		if err == nil {
			all = append(all, ffSnaps...)
			srcs = append(srcs, ffSrcs...)
		}

		if runtime.GOOS == "darwin" {
			saSnaps, saSrcs, err := readSafariSnapshots("")
			if err == nil {
				all = append(all, saSnaps...)
				srcs = append(srcs, saSrcs...)
			}
		}

		if len(srcs) == 0 {
			return nil, nil, fmt.Errorf("no Firefox/Safari profiles found at default locations; pass --file or --browser with --profile")
		}
		return all, srcs, nil
	}
}

func readNetscapeAsSnapshots(r io.Reader, label string) ([]session.CookieSnapshot, []string, error) {
	cookies, err := session.ParseNetscape(r)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", label, err)
	}
	out := make([]session.CookieSnapshot, len(cookies))
	for i, c := range cookies {
		// Netscape files carry no per-cookie timestamp; leave LastSeen zero
		// so any source with real timestamps would win in a merge (but a
		// bare --file source never merges with other sources).
		out[i] = session.CookieSnapshot{Cookie: c, Source: label}
	}
	return out, []string{label}, nil
}

func readAllFirefoxSnapshots() ([]session.CookieSnapshot, []string, error) {
	profiles, err := session.AllFirefoxProfiles()
	if err != nil {
		return nil, nil, err
	}
	if len(profiles) == 0 {
		return nil, nil, nil
	}
	var all []session.CookieSnapshot
	var srcs []string
	for _, p := range profiles {
		snaps, err := session.ReadFirefoxSnapshots(p.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping Firefox profile %s: %v\n", p.Label, err)
			continue
		}
		all = append(all, snaps...)
		srcs = append(srcs, "firefox("+p.Label+")")
	}
	return all, srcs, nil
}

func readSafariSnapshots(pathOverride string) ([]session.CookieSnapshot, []string, error) {
	path := pathOverride
	if path == "" {
		def, err := session.DefaultSafariCookiesPath()
		if err != nil {
			return nil, nil, err
		}
		path = def
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read safari cookies: %w", err)
	}
	label := "safari(" + filepath.Base(path) + ")"
	snaps, err := session.ParseSafariSnapshots(data, label)
	if err != nil {
		return nil, nil, fmt.Errorf("parse safari cookies: %w", err)
	}
	return snaps, []string{label}, nil
}

func joinSources(srcs []string) string {
	switch len(srcs) {
	case 0:
		return "(none)"
	case 1:
		return srcs[0]
	default:
		return fmt.Sprintf("%d sources: %v", len(srcs), srcs)
	}
}

func init() {
	importCookiesCmd.Flags().String("file", "", "Path to Netscape cookies.txt")
	importCookiesCmd.Flags().String("browser", "", "Read directly from 'firefox' or 'safari'")
	importCookiesCmd.Flags().String("profile", "", "Browser profile directory (firefox) or cookies file (safari); auto-detected if omitted")
	importCookiesCmd.Flags().String("user-agent", "", "User-Agent to store in session (defaults to a recent Chrome UA)")
	// Proxy flags mirror those on addTransportFlags so that RefreshTokens can
	// route through a user-supplied proxy when NotebookLM is not directly
	// reachable.
	importCookiesCmd.Flags().String("proxy", "", "Proxy URL")
	importCookiesCmd.Flags().String("socks5-proxy", "", "SOCKS5 proxy address (e.g. 127.0.0.1:1080)")
	importCookiesCmd.Flags().String("http-proxy", "", "HTTP proxy address (e.g. 127.0.0.1:8080)")
	importCookiesCmd.Flags().String("https-proxy", "", "HTTPS proxy address (e.g. 127.0.0.1:8443)")
}
