package transport

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/launcher"
)

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
