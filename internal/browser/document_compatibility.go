package browser

import "github.com/moreveal/mimic/internal/engine"

func (r *Realm) installDocumentCompatibility(host map[string]any) {
	host["createHTMLDocument"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		var title *string
		if len(args) > 0 {
			value := strarg(args, 0)
			title = &value
		}
		return r.val(nodeData(r.document.CreateHTMLDocument(title))), nil
	})
	host["nodeOwnerDocument"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.val(r.document.OwnerDocumentID(int64(numarg(args, 0)))), nil
	})
	host["adoptNodeDocument"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		r.document.AdoptNode(int64(numarg(args, 0)), int64(numarg(args, 1)))
		return nil, nil
	})
}
