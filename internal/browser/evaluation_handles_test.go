package browser

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

func persistentHandleCount(t *testing.T, runtime engine.Runtime) int {
	t.Helper()
	diagnostic, ok := runtime.(interface{ Diagnostics() (any, error) })
	if !ok {
		t.Fatal("V8 diagnostics unavailable")
	}
	value, err := diagnostic.Diagnostics()
	if err != nil {
		t.Fatal(err)
	}
	return value.(map[string]any)["persistent_handles"].(int)
}

func TestPageEvaluationReleasesExportedValuesBeforeClose(t *testing.T) {
	for cycle := 0; cycle < 3; cycle++ {
		p := newAsyncModulePage(t)
		navigateCapabilityFixture(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		runtime := p.Top.Realm.runtime
		// Let the fixture's initial ready tasks finish before measuring roots.
		if _, err := p.Evaluate(ctx, `Promise.resolve(0)`); err != nil {
			t.Fatal(err)
		}
		baseline := persistentHandleCount(t, runtime)
		for iteration := 0; iteration < 100; iteration++ {
			source := `({value:42})`
			if iteration%2 != 0 {
				source = `Promise.resolve({value:42})`
			}
			value, err := p.Evaluate(ctx, source)
			if err != nil || !reflect.DeepEqual(value, map[string]any{"value": float64(42)}) {
				t.Fatalf("cycle %d evaluation %d: %v %v", cycle, iteration, value, err)
			}
		}
		if growth := persistentHandleCount(t, runtime) - baseline; growth != 0 {
			t.Fatalf("cycle %d kept %d temporary evaluation roots before Page.Close", cycle, growth)
		}
		cancel()
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.(interface{ Diagnostics() (any, error) }).Diagnostics(); err == nil {
			t.Fatal("closed Page kept its isolate available")
		}
	}
}

func TestPageEvaluationTransfersNonJSONValueOwnership(t *testing.T) {
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runtime := p.Top.Realm.runtime
	owner := runtime.(engine.ValueReleaser)
	for _, source := range []string{
		`globalThis.savedFunction=()=>42;savedFunction`,
		`Promise.resolve(savedFunction)`,
	} {
		baseline := persistentHandleCount(t, runtime)
		exported, err := p.Evaluate(ctx, source)
		if err != nil {
			t.Fatal(err)
		}
		function, ok := exported.(engine.Value)
		if !ok {
			t.Fatalf("function export lost native ownership: %T", exported)
		}
		if growth := persistentHandleCount(t, runtime) - baseline; growth != 1 {
			t.Fatalf("function result owns %d roots, want its one exported handle", growth)
		}
		result, err := runtime.Call(ctx, function, nil)
		if err != nil || result.Export() != float64(42) {
			t.Fatalf("exported function no longer callable: %v %v", result, err)
		}
		owner.ReleaseValue(result)
		owner.ReleaseValue(function)
		if growth := persistentHandleCount(t, runtime) - baseline; growth != 0 {
			t.Fatalf("released function kept %d roots", growth)
		}
	}
	value, err := p.Evaluate(ctx, `savedFunction()`)
	if err != nil || value != float64(42) {
		t.Fatalf("releasing the embedding handle lost the JS reference: %v %v", value, err)
	}
}

func TestCompletedAndCanceledTimersReleaseCallbackRoots(t *testing.T) {
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runtime := p.Top.Realm.runtime
	if _, err := p.Evaluate(ctx, `Promise.resolve(0)`); err != nil {
		t.Fatal(err)
	}
	baseline := persistentHandleCount(t, runtime)
	for _, source := range []string{
		`new Promise(resolve=>{let completed=0;for(let i=0;i<256;i++){const payload=new Uint8Array(4096);payload[0]=1;setTimeout(()=>{completed+=payload[0];if(completed===256)resolve(completed)},0)}})`,
		`(()=>{for(let i=0;i<256;i++){const payload=new Uint8Array(4096);payload[0]=1;clearTimeout(setTimeout(()=>payload[0],60000))}return 256})()`,
		`new Promise(resolve=>{let count=0;globalThis.savedTimerCallback=()=>256;const id=setInterval(()=>{if(++count===2){clearInterval(id);resolve(savedTimerCallback())}},0)})`,
	} {
		value, err := p.Evaluate(ctx, source)
		if err != nil || value != float64(256) {
			t.Fatalf("timer result: %v %v", value, err)
		}
		if growth := persistentHandleCount(t, runtime) - baseline; growth != 0 {
			t.Fatalf("completed timer operations kept %d native roots", growth)
		}
		if len(p.Top.Realm.timers) != 0 {
			t.Fatalf("completed timer operations kept %d registrations", len(p.Top.Realm.timers))
		}
	}
	value, err := p.Evaluate(ctx, `savedTimerCallback()`)
	if err != nil || value != float64(256) {
		t.Fatalf("timer cleanup released a JS-owned function: %v %v", value, err)
	}
}

func TestListenerDispatchDoesNotRetainCheckpointRoots(t *testing.T) {
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runtime := p.Top.Realm.runtime
	if _, err := p.Evaluate(ctx, `Promise.resolve(0)`); err != nil {
		t.Fatal(err)
	}
	baseline := persistentHandleCount(t, runtime)
	for iteration := 0; iteration < 3; iteration++ {
		value, err := p.Evaluate(ctx, `(()=>{let count=0;const target=new EventTarget();for(let i=0;i<256;i++){const payload=new Uint8Array(4096);payload[0]=1;const callback=()=>count+=payload[0];target.addEventListener('probe',callback);target.dispatchEvent(new Event('probe'));target.removeEventListener('probe',callback)}return count})()`)
		if err != nil || value != float64(256) {
			t.Fatalf("listener result: %v %v", value, err)
		}
		if growth := persistentHandleCount(t, runtime) - baseline; growth != 0 {
			t.Fatalf("removed listeners kept %d native checkpoint roots", growth)
		}
	}
}
