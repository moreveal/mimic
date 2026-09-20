package browser

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFrameReferenceArgumentsPreserveIdentity(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		value, err := page.Evaluate(ctx, `(()=>{
 const frame=document.createElement('iframe');document.body.appendChild(frame);const child=frame.contentWindow;
 const Return=child.eval('(function(value){return value})');
 const local={value:1};if(new Return(local)!==local)return 'returned object';
 if(new Return(document)!==document)return 'document identity';if(new Return(window)!==window)return 'window identity';
 const values=[local,local,23];const set=new child.Set(values);if(set.size!==2||!set.has(local)||set.values().next().value!==local)return 'iterator';
 let calls=0;const callable=function(value){calls++;return this===local&&value===local};
 const C=child.eval('(function(object,callback){object.value=17;this.ok=callback.call(object,object);this.saved=object})');
 const result=new C(local,callable);
 if(local.value!==17||!result.ok||calls!==1||result.saved!==local)return 'callback and mutation';
 let reads=0;const getter={get value(){reads++;return local}};
 const Read=child.eval('(function(object){return object.value})');
 if(new Read(getter)!==local||reads!==1)return 'getter';
 if(new Return(callable)!==callable||new Return(values)!==values)return 'callable array identity';
 let receiver;const Receiver=child.eval('(function(callback){this.returned=callback.call(this)})');
 const instance=new Receiver(function(){receiver=this;return local});if(receiver!==instance||instance.returned!==local)return 'callback receiver';
 return true;
 })()`)
		if err != nil || value != true {
			t.Fatalf("reference arguments: %v, %v", value, err)
		}
	})
}

func TestFrameReferenceArgumentsValidateSource(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, page *Page) {
		if _, err := page.Evaluate(context.Background(), `document.body.appendChild(document.createElement('iframe'))`); err != nil {
			t.Fatal(err)
		}
		source := page.Top.Realm
		var target *Realm
		for _, frame := range source.childFrames {
			target = frame.Realm
			break
		}
		if target == nil {
			t.Fatal("missing child realm")
		}
		raw := map[string]any{"kind": "reference", "type": "object", "frame": page.Top.ID, "realm": "stale", "handle": 1}
		if _, err := target.decodeFrameArgument(raw); err == nil || !strings.Contains(err.Error(), "no longer available") {
			t.Fatalf("stale source: %v", err)
		}
		raw["realm"] = source.ID
		if _, err := target.decodeFrameArgument(raw); err == nil || !strings.Contains(err.Error(), "no longer available") {
			t.Fatalf("missing handle: %v", err)
		}
		origin := source.origin
		source.origin = "https://other.example"
		_, err := target.decodeFrameArgument(raw)
		source.origin = origin
		if err == nil || !strings.Contains(err.Error(), "SecurityError") {
			t.Fatalf("cross-origin source: %v", err)
		}
	})
}
