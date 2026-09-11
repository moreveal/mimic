package gojaengine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBootstrapProgramsPreserveConcurrentRealmIsolation(t *testing.T) {
	const source = `var privateState=[];globalThis.append=value=>{privateState.push(value);return privateState.join(',')};globalThis.object={};return 17;`
	for i := 0; i < 16; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			r := Factory{}.New().(*runtime)
			defer r.Close()
			v, err := r.EvalBootstrap(context.Background(), source, "shared-bootstrap.js")
			if err != nil || v.Export() != int64(17) {
				t.Fatalf("bootstrap: %v %v", v, err)
			}
			v, err = r.Eval(context.Background(), fmt.Sprintf(`append(%d)+':'+typeof privateState`, i), "shared-bootstrap.js")
			if err != nil || v.Export() != fmt.Sprintf("%d:undefined", i) {
				t.Fatalf("realm isolation: %v %v", v, err)
			}
			if _, err = r.EvalBootstrap(context.Background(), source, "shared-bootstrap.js"); err != nil {
				t.Fatal(err)
			}
			v, err = r.Eval(context.Background(), `append('fresh')`, "read.js")
			if err != nil || v.Export() != "fresh" {
				t.Fatalf("fresh bindings: %v %v", v, err)
			}
		})
	}
}

func TestBootstrapCancellationAndOrdinaryEval(t *testing.T) {
	r := Factory{}.New().(*runtime)
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := r.EvalBootstrap(ctx, `for(;;){}`, "bootstrap.js"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := r.EvalBootstrap(context.Background(), `return )`, "invalid.js"); err == nil {
		t.Fatal("syntax error lost")
	}
	v, err := r.Eval(context.Background(), `var ordinary=21;ordinary*2`, "mimic:webapi-surface")
	if err != nil || v.Export() != int64(42) {
		t.Fatalf("ordinary eval: %v %v", v, err)
	}
	v, err = r.Eval(context.Background(), `ordinary`, "read.js")
	if err != nil || v.Export() != int64(21) {
		t.Fatalf("ordinary global binding: %v %v", v, err)
	}
}

func TestBootstrapProgramRetentionIsBounded(t *testing.T) {
	for i := 0; i < bootstrapProgramLimit*2; i++ {
		if _, err := compileBootstrap(fmt.Sprintf("return %d;", i), "bounded.js"); err != nil {
			t.Fatal(err)
		}
	}
	bootstrapPrograms.Lock()
	n := len(bootstrapPrograms.entries)
	bootstrapPrograms.Unlock()
	if n > bootstrapProgramLimit {
		t.Fatalf("retained %d programs", n)
	}
}
