package chrome152

import (
	"encoding/json"
	"sort"
	"strings"
)

// Chrome's Trusted Types event-attribute set includes handlers on interfaces
// outside Window. Derive it from our existing versioned declaration catalog;
// the frozen behavioral oracle verifies the resulting set independently.
func trustedTypeEventAttributes(catalog string) []string {
	var interfaces []struct {
		Members []struct {
			Kind string
			Name string
		}
	}
	if err := json.Unmarshal([]byte(catalog), &interfaces); err != nil {
		panic(err)
	}
	names := map[string]bool{}
	for _, iface := range interfaces {
		for _, member := range iface.Members {
			if member.Kind == "attribute" && strings.HasPrefix(member.Name, "on") && strings.ToLower(member.Name) == member.Name {
				names[member.Name] = true
			}
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
