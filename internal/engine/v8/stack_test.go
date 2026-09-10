//go:build windows && amd64

package v8

import (
	"context"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestNativeStackCaptureDoesNotInvokeJavaScriptHooks(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	capture := runtime.(engine.NativeStackCapture)
	if got := capture.CaptureNativeStack(8, true); len(got) != 0 {
		t.Fatalf("capture outside callback = %+v", got)
	}
	var frames, withoutSources []engine.NativeStackFrame
	if err := runtime.Set("captureProbe", runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		frames = capture.CaptureNativeStack(8, true)
		withoutSources = capture.CaptureNativeStack(1, false)
		if got := capture.CaptureNativeStack(0, true); len(got) != 0 {
			t.Errorf("zero limit returned %d frames", len(got))
		}
		return runtime.Value(nil), nil
	})); err != nil {
		t.Fatal(err)
	}
	const source = `Object.defineProperty(Error, 'prepareStackTrace', {
  get() { throw new Error('application stack hook must not run'); }
});
function outer() { return inner(); }
function inner() { captureProbe(); }
outer();`
	if _, err := runtime.Eval(context.Background(), source, "native-stack-test.js"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, frame := range frames {
		if frame.Function == "inner" {
			found = true
			if frame.URL != "native-stack-test.js" || frame.Line != 5 || frame.Column <= 0 || frame.Source != source {
				t.Fatalf("inner frame = %+v", frame)
			}
		}
	}
	if !found {
		t.Fatalf("missing caller in %+v", frames)
	}
	if len(withoutSources) != 1 || withoutSources[0].Source != "" {
		t.Fatalf("source-free bounded capture = %+v", withoutSources)
	}
}
