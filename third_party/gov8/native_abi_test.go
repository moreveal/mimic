//go:build (windows || linux) && amd64

package gov8_test

import (
	"math"
	"testing"

	"github.com/maclof/gov8"
)

// Shares the process-wide V8 lifecycle with TestHTMLDDA. These mixed pointer /
// double signatures exposed Windows positional-XMM assumptions during the port.
func checkNativeNumberABI(t *testing.T, iso *gov8.Isolate, scope *gov8.Scope, ctx *gov8.Context) {
	for _, row := range []struct {
		value    float64
		expected string
	}{
		{1.25, "1.25"}, {-123.75, "-123.75"}, {math.Copysign(0, -1), "-0"},
		{math.Inf(1), "Infinity"}, {math.NaN(), "NaN"},
	} {
		number, err := scope.Number(row.value)
		if err != nil {
			t.Fatal(err)
		}
		advSeed(t, scope, ctx, "nativeNumber", number)
		callback, err := iso.NewFunction(scope, ctx, func(cs *gov8.CallbackScope, args gov8.FunctionCallbackArguments, rv gov8.ReturnValue) {
			if err := rv.SetFloat64(row.value); err != nil {
				t.Error(err)
			}
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		advSeed(t, scope, ctx, "nativeDouble", callback.Value)
		expression := "Object.is(nativeNumber," + row.expected + ") && Object.is(nativeDouble()," + row.expected + ")"
		if got := advEvalText(t, scope, ctx, expression); got != "true" {
			t.Fatalf("%s: %s", expression, got)
		}
	}
	date, err := scope.NewDate(ctx, 123456789.0)
	if err != nil {
		t.Fatal(err)
	}
	advSeed(t, scope, ctx, "nativeDate", date.Value)
	if got := advEvalText(t, scope, ctx, "nativeDate.getTime() === 123456789"); got != "true" {
		t.Fatal(got)
	}
	if err := iso.RunIdleTasks(0); err != nil {
		t.Fatal(err)
	}
}
