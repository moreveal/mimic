package gojaengine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEvalHonorsContextCancellationAndRuntimeRemainsUsable(t *testing.T) {
	runtime := Factory{}.New()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := runtime.Eval(ctx, `for (;;) {}`, "infinite.js"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	value, err := runtime.Eval(context.Background(), `6 * 7`, "after.js")
	if err != nil || value.Export() != int64(42) {
		t.Fatalf("runtime unusable after interruption: value=%v err=%v", value, err)
	}
}
