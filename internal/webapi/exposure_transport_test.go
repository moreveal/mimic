package webapi

import (
	"encoding/json"
	"github.com/dop251/goja"
	"github.com/moreveal/mimic/compatibility"
	"reflect"
	"testing"
)

func TestExposureTransportPreservesDescriptorFields(t *testing.T) {
	section := handwrittenSurfaceSection(t, "const decodeExposure", "const applyTargetExposure")
	var properties []compatibility.SurfaceProperty
	for flags := 0; flags < 16; flags++ {
		for writable := 0; writable < 3; writable++ {
			p := compatibility.SurfaceProperty{Name: "property\"λ", Enumerable: flags&1 != 0, Configurable: flags&2 != 0, Getter: flags&4 != 0, Setter: flags&8 != 0, ValueType: "function", FunctionName: "method"}
			if writable != 0 {
				v := writable == 2
				p.Writable = &v
			}
			if flags%2 == 0 {
				n := flags
				p.FunctionLength = &n
			}
			properties = append(properties, p)
		}
	}
	for _, want := range []compatibility.RealmExposure{{}, {Properties: []compatibility.SurfaceProperty{}, Prototypes: map[string][]compatibility.SurfaceProperty{}}, {PropertyOrder: []string{"b", "a"}, PrototypeOrder: map[string][]string{"__proto__": {"b", "a"}}, InterfaceOrder: map[string][]string{"Example": {"length", "name", "prototype", "create"}}, Properties: properties, Prototypes: map[string][]compatibility.SurfaceProperty{"__proto__": properties, "empty": nil}}} {
		data, err := marshalExposure(want)
		if err != nil {
			t.Fatal(err)
		}
		vm := goja.New()
		if err = vm.Set("transport", string(data)); err != nil {
			t.Fatal(err)
		}
		v, err := vm.RunString(section + `;JSON.stringify(decodeExposure(JSON.parse(transport)))`)
		if err != nil {
			t.Fatal(err)
		}
		var got compatibility.RealmExposure
		if err = json.Unmarshal([]byte(v.String()), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("descriptor transport changed fields: got %#v want %#v", got, want)
		}
		v, err = vm.RunString(`(()=>{const input={properties:[]};return decodeExposure(input)===input&&!Object.hasOwn(Object.prototype,'empty')})()`)
		if err != nil || !v.ToBoolean() {
			t.Fatalf("legacy input/prototype isolation: %v %v", v, err)
		}
	}
}
