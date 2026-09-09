package browser

import "github.com/moreveal/mimic/internal/engine"

func (r *Realm) installDocumentCompatibility(host map[string]any) {
	host["parseInertDocument"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		node, err := r.document.ParseInertDocument(strarg(args, 0), strarg(args, 1), r.documentURL().String())
		if err != nil {
			return nil, err
		}
		return r.val(nodeData(node)), nil
	})
	host["attributeNameNS"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.val(r.document.AttributeNameNS(int64(numarg(args, 0)), strarg(args, 1), strarg(args, 2))), nil
	})
	host["setAttributeNS"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return nil, r.document.SetAttributeNS(int64(numarg(args, 0)), strarg(args, 1), strarg(args, 2), strarg(args, 3))
	})
	host["elementByID"] = r.packedFn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.val(r.document.ElementByID(int64(numarg(args, 0)), strarg(args, 1))), nil
	}, "ns")
	host["createXMLDocument"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.val(nodeData(r.document.CreateXMLDocument(strarg(args, 0), strarg(args, 1)))), nil
	})
	host["createDocumentElement"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.val(nodeData(r.document.CreateDocumentElement(int64(numarg(args, 0)), strarg(args, 1), strarg(args, 2)))), nil
	})
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
