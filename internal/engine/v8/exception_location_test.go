//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"strings"
	"testing"
)

func TestCallbackErrorPreservesSourceWithoutReadingStack(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	ctx := context.Background()
	fn, err := runtime.Eval(ctx, `globalThis.stackReads=0;
(function sourceFailure(){
const error=new Error('callback sentinel');
Object.defineProperty(error,'stack',{get(){stackReads++;throw new Error('stack getter executed')}});
throw error;
})`, "callback-source.js")
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.Call(ctx, fn, nil)
	if err == nil || !strings.Contains(err.Error(), "callback sentinel") || !strings.Contains(err.Error(), "callback-source.js:") {
		t.Fatalf("callback diagnostic: %v", err)
	}
	reads, err := runtime.Eval(ctx, "stackReads", "check.js")
	if err != nil || reads.String() != "0" {
		t.Fatalf("diagnostics invoked application stack: %v %v", reads, err)
	}
}
