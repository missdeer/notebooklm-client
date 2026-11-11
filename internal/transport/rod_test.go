package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher"
)

// A local CDP stub exercises Rod's page cache and contexts without a Google
// account or browser process.
type loginCDP struct {
	attachments int
	events      chan *cdp.Event
}

func (c *loginCDP) Event() <-chan *cdp.Event { return c.events }

func (c *loginCDP) Call(ctx context.Context, sessionID, method string, params interface{}) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch method {
	case "Target.attachToTarget":
		c.attachments++
		return fmt.Appendf(nil, `{"sessionId":"session-%d"}`, c.attachments), nil
	case "Runtime.evaluate":
		return []byte(`{"result":{"type":"object","objectId":"window"}}`), nil
	case "Runtime.callFunctionOn":
		return []byte(`{"result":{"type":"object","value":{"at":"token","bl":"build","baseHost":"notebook.google.com"}}}`), nil
	default:
		return []byte(`{}`), nil
	}
}

func TestWaitForTokensReconnectsDetachedPage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cdpClient := &loginCDP{events: make(chan *cdp.Event)}
	defer close(cdpClient.events)
	browser := rod.New().Context(ctx).NoDefaultDevice().Client(cdpClient)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	page, err := browser.PageFromTarget("login-tab")
	if err != nil {
		t.Fatal(err)
	}
	detached, detach := page.WithCancel()
	detach()

	transport := NewRodTransport(RodTransportOptions{})
	tokens, recovered, err := transport.waitForTokens(ctx, detached, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AT != "token" || recovered.TargetID != page.TargetID || cdpClient.attachments != 2 {
		t.Fatal("did not reconnect to the same login tab and extract tokens")
	}
	if recovered.GetContext().Err() != nil || recovered.Browser().GetContext().Err() != nil {
		t.Fatal("polling deadline leaked into recovered page lifetime")
	}
	requestCtx, cancelRequest := context.WithCancel(ctx)
	cancelRequest()
	_, _, err = transport.waitForTokens(requestCtx, recovered, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("request cancellation must stop polling: %v", err)
	}

	cancel()
	_, _, err = transport.waitForTokens(ctx, recovered, time.Second)
	if !errors.Is(err, context.Canceled) || cdpClient.attachments != 2 {
		t.Fatalf("command cancellation must stop polling without reconnecting: %v", err)
	}
}

// Execute the actual extraction script with page globals to cover both WIZ
// and inline bootstrap JSON without requiring an authenticated browser.
func TestSessionTokenExtraction(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable for browser JavaScript regression test")
	}
	for _, tc := range []struct{ name, host, html, wiz, want string }{
		{"old host", "notebooklm.google.com", "", `{"SNlM0e":"token","cfb2h":"labs-tailwind-old"}`, "token"},
		{"rebrand", "notebook.google.com", "", `{"SNlM0e":"token","cfb2h":"gemini-notebook-new"}`, "token"},
		{"HTML fallback", "notebook.google.com", `<script>{"SNlM0e" : "token\u003d", "cfb2h":"new-build","FdrFJe":"123"}</script>`, `{}`, "token="},
		{"login page", "accounts.google.com", "", `{"SNlM0e":"login-token"}`, ""},
		{"spoof host", "evil-notebooklm.google.com", "", `{"SNlM0e":"token"}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, _ := json.Marshal(tc.host)
			html, _ := json.Marshal(tc.html)
			script := fmt.Sprintf(`global.location={hostname:%s}; global.window={WIZ_global_data:%s}; global.document={documentElement:{outerHTML:%s,lang:'zh-CN'}}; console.log(JSON.stringify((%s)()));`, host, tc.wiz, html, sessionTokensScript)
			out, err := exec.Command(node, "-e", script).CombinedOutput()
			if err != nil {
				t.Fatalf("script failed: %v: %s", err, out)
			}
			var result struct {
				AT       string
				BaseHost string
				Language string
			}
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal(err)
			}
			if result.AT != tc.want {
				t.Fatalf("unexpected token extraction result")
			}
			if tc.want != "" && (result.BaseHost != tc.host || result.Language != "zh") {
				t.Fatal("host or language not extracted")
			}
		})
	}
}

func TestApplyAntiDetectionFlags(t *testing.T) {
	l := applyAntiDetectionFlags(launcher.New())

	if l.Has("enable-automation") {
		t.Fatal("enable-automation should be removed")
	}
	if l.Has("rod-leakless") {
		t.Fatal("rod-leakless should be disabled")
	}
	if got := l.Get("disable-blink-features"); got != "AutomationControlled" {
		t.Fatalf("disable-blink-features = %q, want AutomationControlled", got)
	}
	if got := l.Get("lang"); got != "en-US" {
		t.Fatalf("lang = %q, want en-US", got)
	}

	disableFeatures, ok := l.GetFlags("disable-features")
	if !ok {
		t.Fatal("disable-features should be present")
	}
	found := false
	for _, feature := range disableFeatures {
		if feature == "InfiniteSessionRestore" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("disable-features = %v, want InfiniteSessionRestore", disableFeatures)
	}
}

func TestAntiDetectionScriptFormats(t *testing.T) {
	cfgJSON := `{"hardwareConcurrency":8,"deviceMemory":8,"webglVendor":"Google Inc.","webglRenderer":"ANGLE","languages":["en-US","en"],"platform":"Win32","canvasNoiseSeed":1,"canvasNoisePixels":8,"canvasNoiseDelta":3}`
	script := fmt.Sprintf(antiDetectionScript, cfgJSON)

	for _, want := range []string{
		"Navigator.prototype, 'webdriver'",
		"Navigator.prototype, 'plugins'",
		"HTMLCanvasElement.prototype.toDataURL",
		"WebGLRenderingContext",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("formatted script missing %q", want)
		}
	}
	if strings.Contains(script, "%%") {
		t.Fatal("formatted script still contains escaped percent markers")
	}
}
