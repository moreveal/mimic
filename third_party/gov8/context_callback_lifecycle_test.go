//go:build (windows || linux) && amd64

package gov8

import (
	"testing"
)

func CheckContextCloseReleasesDirectFunctionCallbacks(t *testing.T, iso *Isolate) {
	count := func() int {
		hostCallbackRegistry.mu.Lock()
		defer hostCallbackRegistry.mu.Unlock()
		n := 0
		for _, entry := range hostCallbackRegistry.entries {
			if entry.iso == iso {
				n++
			}
		}
		return n
	}
	baseline := count()
	for i := 0; i < 32; i++ {
		scope, err := iso.NewScope()
		if err != nil {
			t.Fatal(err)
		}
		ctx, err := iso.NewContext()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = iso.NewFunction(scope, ctx, func(*CallbackScope, FunctionCallbackArguments, ReturnValue) {}, nil); err != nil {
			t.Fatal(err)
		}
		if got := count(); got != baseline+1 {
			t.Fatalf("registered callbacks: got %d, want %d", got, baseline+1)
		}
		if err = ctx.Close(); err != nil {
			t.Fatal(err)
		}
		if got := count(); got != baseline {
			t.Fatalf("callbacks retained after context close: got %d, want %d", got, baseline)
		}
		if err = scope.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
