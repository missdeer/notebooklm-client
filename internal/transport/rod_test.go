package transport

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/launcher"
)

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
