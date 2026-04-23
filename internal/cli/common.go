package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/client"
	"github.com/missdeer/notebooklm-client/internal/types"
)

func addTransportFlags(cmd *cobra.Command) {
	cmd.Flags().String("transport", "auto", "Transport mode: auto, http, browser, curl")
	cmd.Flags().String("session-path", "", "Session file path")
	cmd.Flags().String("proxy", "", "Proxy URL")
	cmd.Flags().String("socks5-proxy", "", "SOCKS5 proxy address (e.g. 127.0.0.1:1080)")
	cmd.Flags().String("http-proxy", "", "HTTP proxy address (e.g. 127.0.0.1:8080)")
	cmd.Flags().String("https-proxy", "", "HTTPS proxy address (e.g. 127.0.0.1:8443)")
	cmd.Flags().String("profile", "", "Chrome profile directory")
	cmd.Flags().Bool("headless", false, "Run headless")
	cmd.Flags().String("chrome-path", "", "Chrome executable path")
}

func addSourceFlags(cmd *cobra.Command) {
	cmd.Flags().String("url", "", "Source URL")
	cmd.Flags().String("text", "", "Source text content")
	cmd.Flags().String("file", "", "Source file path")
	cmd.Flags().String("topic", "", "Research topic")
	cmd.Flags().String("research-mode", "fast", "Research mode: fast, deep")
}

func buildSource(cmd *cobra.Command) (types.SourceInput, error) {
	u, _ := cmd.Flags().GetString("url")
	text, _ := cmd.Flags().GetString("text")
	file, _ := cmd.Flags().GetString("file")
	topic, _ := cmd.Flags().GetString("topic")
	mode, _ := cmd.Flags().GetString("research-mode")

	count := 0
	if u != "" { count++ }
	if text != "" { count++ }
	if file != "" { count++ }
	if topic != "" { count++ }

	if count == 0 {
		return types.SourceInput{}, fmt.Errorf("one of --url, --text, --file, or --topic is required")
	}
	if count > 1 {
		return types.SourceInput{}, fmt.Errorf("only one of --url, --text, --file, or --topic may be specified")
	}

	switch {
	case u != "":
		return types.SourceInput{Type: types.SourceURL, URL: u}, nil
	case text != "":
		return types.SourceInput{Type: types.SourceText, Text: text}, nil
	case file != "":
		return types.SourceInput{Type: types.SourceFile, FilePath: file}, nil
	case topic != "":
		return types.SourceInput{Type: types.SourceResearch, Topic: topic, ResearchMode: types.ResearchMode(mode)}, nil
	}
	return types.SourceInput{}, fmt.Errorf("no source specified")
}

// normalizeProxyURL prepends the given scheme if the value has no scheme.
func normalizeProxyURL(value, scheme string) string {
	if strings.Contains(value, "://") {
		return value
	}
	return scheme + "://" + value
}

func resolveProxy(cmd *cobra.Command) string {
	// CLI flags take priority
	if p, _ := cmd.Flags().GetString("socks5-proxy"); p != "" {
		return normalizeProxyURL(p, "socks5")
	}
	if p, _ := cmd.Flags().GetString("http-proxy"); p != "" {
		return normalizeProxyURL(p, "http")
	}
	if p, _ := cmd.Flags().GetString("https-proxy"); p != "" {
		return normalizeProxyURL(p, "https")
	}
	if p, _ := cmd.Flags().GetString("proxy"); p != "" {
		return p
	}
	// Environment variables fallback
	for _, key := range []string{
		"SOCKS5_PROXY", "socks5_proxy",
		"HTTP_PROXY", "http_proxy",
		"HTTPS_PROXY", "https_proxy",
		"ALL_PROXY", "all_proxy",
	} {
		if p := os.Getenv(key); p != "" {
			return p
		}
	}
	return ""
}

func withClient(cmd *cobra.Command, fn func(context.Context, *client.NotebookClient) error) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	transport, _ := cmd.Flags().GetString("transport")
	sessionPath, _ := cmd.Flags().GetString("session-path")
	proxy := resolveProxy(cmd)

	c := client.New()
	if err := c.Connect(ctx, client.ConnectOptions{
		Transport:   client.TransportMode(transport),
		SessionPath: sessionPath,
		Proxy:       proxy,
	}); err != nil {
		return err
	}
	defer c.Disconnect()

	return fn(ctx, c)
}

func progressLogger(p types.WorkflowProgress) {
	log.Printf("[%s] %s", p.Status, p.Message)
}
