//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"testing"
)

func TestIsolatedWorldRuntimeMaterializesOnFirstEvaluation(t *testing.T) {
	serialBrowserTest(t)
	page := bootstrapSnapshotPage(t)
	realmID, err := page.IsolatedWorld(context.Background(), page.Top.ID, "demand-driven")
	if err != nil {
		t.Fatal(err)
	}
	world := page.Top.Realm.isolatedWorlds["demand-driven"]
	deferred, ok := world.runtime.(*deferredRuntime)
	if !ok || deferred.closed || world.ID != realmID {
		t.Fatalf("isolated world was materialized before use: %T", world.runtime)
	}
	debugger := NewDebugger(page)
	defer debugger.Close()
	result, err := debugger.Evaluate(context.Background(), page.Top.ID, realmID, "globalThis.marker=41;marker+1", DebuggerOptions{ReturnByValue: true})
	if err != nil || result["result"].(map[string]any)["value"] != float64(42) {
		t.Fatalf("first isolated-world evaluation: %#v %v", result, err)
	}
	if world.runtime == deferred {
		t.Fatal("first evaluation did not materialize the isolated world")
	}
}

func TestSemanticallyEmptyInitScript(t *testing.T) {
	for _, source := range []string{"", " \t\r\n", "// sourceURL=internal\n", "/* block */ // line\n"} {
		if !semanticallyEmptyScript(source) {
			t.Fatalf("empty script rejected: %q", source)
		}
	}
	for _, source := range []string{"0", "/* unterminated", "/", "// comment\ntrue"} {
		if semanticallyEmptyScript(source) {
			t.Fatalf("executable script accepted: %q", source)
		}
	}
}

func TestInitScriptWorldMaterializesOnlyForExecutableSource(t *testing.T) {
	serialBrowserTest(t)
	page := bootstrapSnapshotPage(t)
	ctx := context.Background()
	page.AddInitScriptWorld("// sourceURL=automation", "empty-init")
	page.AddInitScriptWorld("globalThis.initMarker=42", "executable-init")
	page.runInitScripts(ctx, page.Top.Realm)

	empty := page.Top.Realm.isolatedWorlds["empty-init"]
	if _, ok := empty.runtime.(*deferredRuntime); !ok {
		t.Fatalf("comment-only init script materialized its realm: %T", empty.runtime)
	}
	executable := page.Top.Realm.isolatedWorlds["executable-init"]
	if _, ok := executable.runtime.(*deferredRuntime); ok {
		t.Fatal("executable init script left its realm deferred")
	}
	debugger := NewDebugger(page)
	defer debugger.Close()
	result, err := debugger.Evaluate(ctx, page.Top.ID, executable.ID, "initMarker", DebuggerOptions{ReturnByValue: true})
	if err != nil || result["result"].(map[string]any)["value"] != float64(42) {
		t.Fatalf("executable init script result: %#v %v", result, err)
	}
}
