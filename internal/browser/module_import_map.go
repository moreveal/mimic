package browser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type importMapJSON struct {
	Imports map[string]string `json:"imports"`
}

// installImportMap records the document's resolved import mappings. Chrome
// resolves both keys and values against the document base when the map is
// processed; keeping canonical strings here makes every module projection use
// the same mapping without performing work inside V8's linker callback.
func (r *Realm) installImportMap(source string) error {
	var parsed importMapJSON
	if err := json.Unmarshal([]byte(source), &parsed); err != nil {
		return fmt.Errorf("invalid import map: %w", err)
	}
	base := r.documentURL()
	if r.importMap == nil {
		r.importMap = make(map[string]string)
	}
	for key, value := range parsed.Imports {
		if key == "" || value == "" {
			continue
		}
		resolved, err := base.Parse(value)
		if err != nil {
			continue
		}
		r.importMap[key] = resolved.String()
	}
	return nil
}

func (r *Realm) resolveModuleSpecifier(specifier, referrer string) (*url.URL, *url.URL, error) {
	base, err := url.Parse(referrer)
	if err != nil {
		return nil, nil, err
	}
	if mapped, ok := r.importMap[specifier]; ok {
		target, err := url.Parse(mapped)
		return target, base, err
	}
	// Package-prefix mappings are considered longest first.
	keys := make([]string, 0, len(r.importMap))
	for key := range r.importMap {
		if strings.HasSuffix(key, "/") && strings.HasPrefix(specifier, key) {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	if len(keys) != 0 {
		mapped := r.importMap[keys[0]] + strings.TrimPrefix(specifier, keys[0])
		target, err := url.Parse(mapped)
		return target, base, err
	}
	if !strings.Contains(specifier, ":") && !strings.HasPrefix(specifier, "/") && !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") {
		return nil, base, fmt.Errorf("Failed to resolve module specifier %q", specifier)
	}
	target, err := base.Parse(specifier)
	return target, base, err
}
