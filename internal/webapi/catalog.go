package webapi

import (
	"encoding/json"

	"github.com/moreveal/mimic/compatibility"
)

// selectedCatalog removes fallback bindings which applyTargetExposure would
// immediately delete. The complete captured exposure still owns final shape;
// native members and handwritten implementations are not filtered here.
// This runs once per immutable bundle/profile, outside any Page runtime lock.
func selectedCatalog(source string, exposure compatibility.RealmExposure) string {
	if source == "" {
		return source
	}
	var specs []map[string]json.RawMessage
	if json.Unmarshal([]byte(source), &specs) != nil {
		return source
	}
	decodeString := func(raw json.RawMessage) string { var s string; _ = json.Unmarshal(raw, &s); return s }
	byName := make(map[string]map[string]json.RawMessage, len(specs))
	for _, spec := range specs {
		byName[decodeString(spec["name"])] = spec
	}
	globals := make(map[string]bool, len(exposure.Properties))
	for _, property := range exposure.Properties {
		globals[property.Name] = true
	}
	keep := make(map[string]bool)
	var include func(string)
	include = func(name string) {
		if name == "" || keep[name] {
			return
		}
		keep[name] = true
		include(decodeString(byName[name]["parent"]))
	}
	for name, spec := range byName {
		if globals[name] {
			include(name)
			continue
		}
		var aliases []string
		_ = json.Unmarshal(spec["legacyWindowAliases"], &aliases)
		for _, alias := range aliases {
			if globals[alias] {
				include(name)
				break
			}
		}
	}
	selected := make([]map[string]json.RawMessage, 0, len(keep))
	neededByDescendants := map[string]map[string]bool{}
	for name, members := range exposure.Prototypes {
		seen := map[string]bool{}
		for current := name; current != "" && !seen[current]; current = decodeString(byName[current]["parent"]) {
			seen[current] = true
			if neededByDescendants[current] == nil {
				neededByDescendants[current] = map[string]bool{}
			}
			for _, member := range members {
				neededByDescendants[current][member.Name] = true
			}
		}
	}
	for _, spec := range specs {
		name := decodeString(spec["name"])
		if !keep[name] {
			continue
		}
		if _, captured := exposure.Prototypes[name]; !captured {
			// Missing capture data is not evidence that the prototype is empty.
			selected = append(selected, spec)
			continue
		}
		// Ancestors may temporarily exist solely to build an exposed child's
		// prototype chain. Their members are still normalized by the exposure.
		allowed := map[string]bool{}
		// Keep members which influence an exposed descendant's inherited
		// lookup during generation, even if final ownership moves downwards.
		for member := range neededByDescendants[name] {
			allowed[member] = true
		}
		seen := map[string]bool{}
		for current := name; current != "" && !seen[current]; current = decodeString(byName[current]["parent"]) {
			seen[current] = true
			for _, property := range exposure.Prototypes[current] {
				allowed[property.Name] = true
			}
		}
		var members []map[string]json.RawMessage
		if json.Unmarshal(spec["members"], &members) != nil {
			return source
		}
		filtered := make([]map[string]json.RawMessage, 0, len(members))
		for _, member := range members {
			var static bool
			_ = json.Unmarshal(member["static"], &static)
			if static || decodeString(member["kind"]) == "constant" || allowed[decodeString(member["name"])] {
				filtered = append(filtered, member)
			}
		}
		encoded, err := json.Marshal(filtered)
		if err != nil {
			return source
		}
		spec["members"] = encoded
		selected = append(selected, spec)
	}
	encoded, err := json.Marshal(selected)
	if err != nil {
		return source
	}
	return string(encoded)
}
