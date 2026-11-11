package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
)

// CaptureCDPSession reads an existing browser without closing the user's browser.
func CaptureCDPSession(ctx context.Context, endpoint string) (*types.NotebookRpcSession, error) {
	control := endpoint
	if !strings.HasPrefix(control, "ws://") && !strings.HasPrefix(control, "wss://") {
		req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(endpoint, "/")+"/json/version", nil)
		if err != nil {
			return nil, err
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("CDP endpoint HTTP %d", resp.StatusCode)
		}
		var info struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
			return nil, err
		}
		control = info.WebSocketDebuggerURL
		if control == "" {
			return nil, fmt.Errorf("CDP endpoint did not expose a websocket URL")
		}
	}
	browser := rod.New().ControlURL(control).Context(ctx)
	if err := browser.Connect(); err != nil {
		return nil, err
	}
	pages, err := browser.Pages()
	if err != nil {
		return nil, err
	}
	var page *rod.Page
	created := false
	for _, p := range pages {
		info, err := p.Info()
		if err != nil {
			return nil, err
		}
		if rpc.NotebookHost(info.URL) != "" {
			page = p
			break
		}
	}
	if page == nil {
		page, err = browser.Page(proto.TargetCreateTarget{URL: rpc.DashboardURL})
		if err != nil {
			return nil, err
		}
		created = true
	}
	if created {
		defer page.Close()
	}
	t := NewRodTransport(RodTransportOptions{})
	t.browser = browser
	t.page = page
	if err := t.captureSession(ctx, page, 30*time.Second); err != nil {
		return nil, err
	}
	sess := t.GetSession()
	return &sess, nil
}
