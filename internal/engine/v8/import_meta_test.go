//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestModuleImportMetaURL(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	modules := runtime.(engine.ModuleRuntime)
	ctx := context.Background()
	loader := func(specifier, referrer string) (string, string, error) {
		base, err := url.Parse(referrer)
		if err != nil {
			return "", "", err
		}
		resolved, err := base.Parse(specifier)
		if err != nil {
			return "", "", err
		}
		return `export const meta=import.meta`, resolved.String(), nil
	}
	const entry = "https://example.test/modules/entry.js?version=1#fragment"
	value, err := modules.EvalModule(ctx, `
import {meta as dependency} from './dependency.js?version=2#part';
globalThis.entryMeta=import.meta;
globalThis.dependencyMeta=dependency;
globalThis.later=()=>import('./dynamic.js').then(m=>globalThis.dynamicMeta=m.meta);
const d=Object.getOwnPropertyDescriptor(import.meta,'url');
if(import.meta.url!==`+fmt.Sprintf("%q", entry)+` || import.meta!==import.meta ||
   Object.getPrototypeOf(import.meta)!==null || dependency===import.meta ||
   !d.writable || !d.enumerable || !d.configurable) throw Error('invalid metadata');
`, entry, loader)
	if err != nil {
		t.Fatal(err)
	}
	if _, settled, err := runtime.Await(value); err != nil || !settled {
		t.Fatalf("module: settled=%v err=%v", settled, err)
	}
	// A later entry replaces the host callback. Previously compiled modules
	// must still resolve metadata against their own identity.
	if _, err := modules.EvalModule(ctx, `globalThis.otherMeta=import.meta`, "https://example.test/other.js", loader); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Eval(ctx, `later()`, "later.js"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Eval(ctx, `[
entryMeta.url,
dependencyMeta.url,
dynamicMeta.url,
otherMeta.url,
(entryMeta.url='changed',entryMeta.url),
(delete entryMeta.url,!Object.hasOwn(entryMeta,'url'))
].join('|')`, "check.js")
	if err != nil {
		t.Fatal(err)
	}
	want := entry + "|https://example.test/modules/dependency.js?version=2#part|https://example.test/modules/dynamic.js|https://example.test/other.js|changed|true"
	if result.String() != want {
		t.Fatalf("metadata = %s, want %s", result.String(), want)
	}
}
