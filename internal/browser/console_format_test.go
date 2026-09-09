package browser

import (
	"context"
	"fmt"
	"testing"

	"github.com/moreveal/mimic/internal/trace"
)

func TestConsoleFormattingOnlyCoercesRequestedArguments(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{
 let effects=[];const value={toString(){effects.push('string');return '12.75'},valueOf(){effects.push('value');return 99}},proxy=new Proxy({},{get(){throw Error('proxy read')}});
 console.log(value,proxy,function(){},null,Symbol('token'));
 console.log('%o %O %c %%',value,proxy,value);
 console.log('%s');
 if(effects.length)return 'plain coercion';
 console.log('numeric %s %d %i %f',value,value,value,value);
 if(effects.join(',')!=='string,string,string,string')return 'numeric coercion';
 effects=[];console.log('%% %s',value,proxy);if(effects.length!==1)return 'percent consumption';
 effects=[];console.log('%q %s',value,proxy);if(effects.length!==1)return 'unknown consumption';
 effects=[];console.log('%c %s',proxy,value);if(effects.length!==1)return 'style consumption';
 console.log('symbols %s %d %i %f',Symbol('token'),Symbol(),Symbol(),Symbol());
 const failure={};let caught=false;try{console.log('%s',{toString(){throw failure}})}catch(e){caught=e===failure}if(!caught)return 'exception identity';
 for(const method of ['debug','info','warn','error','group','groupCollapsed'])console[method](proxy);
 const originalString=String,originalParseInt=parseInt,originalMap=Array.prototype.map,originalIncludes=String.prototype.includes;
 try{globalThis.String=globalThis.parseInt=Array.prototype.map=originalString.prototype.includes=()=>{throw Error('public intrinsic')};console.log('intrinsics %d',{toString(){return '19'}})}
 finally{globalThis.String=originalString;globalThis.parseInt=originalParseInt;Array.prototype.map=originalMap;originalString.prototype.includes=originalIncludes}
 return true;
 })()`)
		if err != nil || value != true {
			t.Fatalf("console behavior: %v %v", value, err)
		}
		found := map[string]bool{}
		for _, event := range p.Trace().Events() {
			if event.Kind != trace.Console {
				continue
			}
			args := fmt.Sprint(event.Data["args"])
			found[args] = true
		}
		for _, want := range []string{"[[object] [object] [function] null Symbol(token)]", "[numeric %s %d %i %f 12.75 12 12 12.75]", "[symbols %s %d %i %f Symbol(token) NaN NaN NaN]", "[intrinsics %d 19]"} {
			if !found[want] {
				t.Errorf("missing trace args %s; got %v", want, found)
			}
		}
	})
}
