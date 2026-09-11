//go:build windows && amd64

package v8

import (
	"context"
	"math"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestHostRecordsDefineOwnDataProperties(t *testing.T) {
	r := (Factory{}).New()
	defer r.Close()
	ctx := context.Background()
	if _, err := r.Eval(ctx, `globalThis.setterCalls=0;Object.defineProperty(Object.prototype,'blocked',{set(){setterCalls++},configurable:true});Object.defineProperty(Object.prototype,'readOnly',{value:0,configurable:true});`, "record-setters"); err != nil {
		t.Fatal(err)
	}
	record := map[string]any{"__proto__": map[string]any{"marker": true}, "blocked": 1, "readOnly": 2}
	if err := r.Set("direct", record); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]any{
		"jsonRecord": record,
		// Non-finite numbers force the recursive callback conversion path.
		"recursiveRecord": map[string]float64{"__proto__": 3, "blocked": math.NaN(), "readOnly": 2},
	} {
		data := data
		if err := r.Set(name, r.Function(func(engine.Value, []engine.Value) (engine.Value, error) { return r.Value(data), nil })); err != nil {
			t.Fatal(err)
		}
	}
	v, err := r.Eval(ctx, `(()=>{
 const records=[direct,jsonRecord(),recursiveRecord()];
 for(const record of records){
  if(Object.getPrototypeOf(record)!==Object.prototype)return false;
  for(const name of ['__proto__','blocked','readOnly']){
   const d=Object.getOwnPropertyDescriptor(record,name);
   if(!d||!d.writable||!d.enumerable||!d.configurable||!('value' in d))return false;
  }
 }
 return setterCalls===0&&direct.__proto__.marker===true&&records[1].__proto__.marker===true&&records[2].__proto__===3&&Number.isNaN(records[2].blocked)&&records.every(r=>r.readOnly===2);
})()`, "record-observations")
	if err != nil || v.Export() != true {
		t.Fatalf("record conversion: %v, %v", v, err)
	}
}
