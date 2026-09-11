package webapi

import (
	"encoding/json"
	"github.com/moreveal/mimic/compatibility"
)

// The public capture schema remains unchanged. Bootstrap transport uses tuples
// to avoid reparsing the same field names thousands of times in each realm.
// decodeExposure restores every descriptor field before publication.
func marshalExposure(exposure compatibility.RealmExposure) ([]byte, error) {
	rows := func(properties []compatibility.SurfaceProperty) []any {
		if properties == nil {
			return nil
		}
		result := make([]any, len(properties))
		for i, p := range properties {
			flags := 0
			if p.Enumerable {
				flags |= 1
			}
			if p.Configurable {
				flags |= 2
			}
			if p.Getter {
				flags |= 4
			}
			if p.Setter {
				flags |= 8
			}
			if p.Writable != nil {
				if *p.Writable {
					flags |= 32
				} else {
					flags |= 16
				}
			}
			result[i] = []any{p.Name, flags, p.ValueType, p.FunctionName, p.FunctionLength}
		}
		return result
	}
	var prototypes map[string][]any
	if exposure.Prototypes != nil {
		prototypes = make(map[string][]any, len(exposure.Prototypes))
	}
	for name, properties := range exposure.Prototypes {
		prototypes[name] = rows(properties)
	}
	return json.Marshal([]any{exposure.PropertyOrder, rows(exposure.Properties), prototypes, exposure.PrototypeOrder, exposure.InterfaceOrder})
}
