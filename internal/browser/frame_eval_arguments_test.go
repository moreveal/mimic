package browser

import (
	"context"
	"testing"
)

// Frozen Chrome 152 returns non-string eval inputs unchanged, including boxed
// strings. TrustedScript uses its internal source without invoking toString.
func TestFrameEvalArgumentsPreserveIdentityAndTrustedSource(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		value, err := page.Evaluate(context.Background(), `(()=>{
 const frame=document.createElement('iframe');document.body.appendChild(frame);const child=frame.contentWindow;
 let reads=0;const object={toString(){reads++;throw Error('coerced')}};
 const values=[undefined,null,true,13,Symbol('s'),object,()=>42,new String('17'),document,window];
 if(!values.every(value=>child.eval(value)===value))return 'identity';
 if(child.eval()!==undefined)return 'omitted';
 const policy=trustedTypes.createPolicy('eval-arguments',{createScript:source=>source});
 const source=policy.createScript('globalThis.evalBrand=41;evalBrand+1');
 Object.defineProperty(source,'toString',{value(){reads++;throw Error('trusted coercion')}});
 if(child.eval(source)!==42||child.eval('evalBrand')!==41)return 'trusted source';
 if(reads!==0)return 'coercion';
 return true;
 })()`)
		if err != nil || value != true {
			t.Fatalf("frame eval arguments: %v, %v", value, err)
		}
	})
}

func TestFrameEvalNonStringStillChecksOrigin(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		_, err := page.Evaluate(context.Background(), `const frame=document.createElement('iframe');document.body.appendChild(frame);globalThis.savedEval=frame.contentWindow.eval`)
		if err != nil {
			t.Fatal(err)
		}
		for _, frame := range page.Top.Realm.childFrames {
			frame.Realm.origin = "https://other.example"
		}
		value, err := page.Evaluate(context.Background(), `(()=>{try{savedEval({});return false}catch(error){return String(error).includes('SecurityError')}})()`)
		if err != nil || value != true {
			t.Fatalf("frame eval access: %v, %v", value, err)
		}
	})
}
