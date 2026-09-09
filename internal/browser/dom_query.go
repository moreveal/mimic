package browser

import (
	"context"
	"encoding/json"
	"fmt"
)

// QueryDOM performs a CDP selector query through the same realm-local parser
// and matcher as the DOM APIs. The caller holds the Page command boundary.
func (p *Page) QueryDOM(ctx context.Context, nodeID int64, selector string, all bool) ([]int64, error) {
	p.mu.RLock()
	realm := p.Top.Realm
	p.mu.RUnlock()
	if realm == nil {
		return nil, fmt.Errorf("page has no realm")
	}
	node, ok := realm.document.Get(nodeID)
	if !ok {
		return nil, fmt.Errorf("could not find node with given id")
	}
	if node.Type != "element" && node.Type != "document" && node.Type != "fragment" {
		return nil, fmt.Errorf("node is not a query container")
	}
	if deferred, ok := realm.runtime.(*deferredRuntime); ok {
		if _, err := deferred.ready(); err != nil {
			return nil, err
		}
	}
	if realm.domQueryCallback == nil {
		return nil, fmt.Errorf("DOM selector callback unavailable")
	}
	value, err := realm.runtime.Call(ctx, realm.domQueryCallback, nil, realm.val(nodeID), realm.val(selector), realm.val(all))
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("selector callback returned no result")
	}
	result := struct {
		IDs   []int64 `json:"ids"`
		Error string  `json:"error"`
	}{IDs: make([]int64, 0)}
	if err := json.Unmarshal([]byte(value.String()), &result); err != nil {
		return nil, fmt.Errorf("invalid selector result: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return result.IDs, nil
}
