package browser

import (
	"encoding/json"

	"github.com/moreveal/mimic/internal/dom"
)

// Snapshot only host-owned observations: tracing must not invoke extra JS
// getters, stringify a JS object, or retain a mutable DOM node / engine handle.
func queryTraceResult(n dom.Node, found bool) any {
	if !found {
		return nil
	}
	attrs := make(map[string]string, len(n.Attributes))
	for k, v := range n.Attributes {
		attrs[k] = v
	}
	return map[string]any{"nodeId": n.ID, "type": n.Type, "tagName": n.TagName,
		"namespaceURI": n.Namespace, "parentId": n.Parent, "attributes": attrs,
		"children": append([]int64{}, n.Children...)}
}

func workerMessageTrace(id int64, direction string, data any) map[string]any {
	event := map[string]any{"worker": id, "direction": direction, "dataEncoding": "export-json"}
	// Freeze the already-exported message before either receiver can mutate it.
	// Keep strings in full: they may be executable source rather than a preview.
	encoded, err := json.Marshal(data)
	if err != nil {
		event["dataCaptureError"] = err.Error()
		return event
	}
	event["data"] = json.RawMessage(encoded)
	return event
}

// A JSON projection is diagnostic only. The opaque wire remains authoritative
// for delivery and preserves values (such as cycles and BigInt) JSON cannot.
func serializedWorkerMessageTrace(id int64, direction string, wire any, projection string) map[string]any {
	event := map[string]any{"worker": id, "direction": direction, "dataEncoding": "structured-clone", "wire": wire}
	if json.Valid([]byte(projection)) {
		event["data"] = json.RawMessage(projection)
	} else {
		event["dataCaptureError"] = "Message is not JSON representable; structured clone wire is retained."
	}
	return event
}
