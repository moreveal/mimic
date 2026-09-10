package browser

import "github.com/moreveal/mimic/internal/engine"

func (r *Realm) installDocumentCompatibility(host map[string]any) {
	host["stylesheetResource"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		u, err := r.resolveDocument(strarg(args, 0))
		if err != nil {
			return nil, nil
		}
		res, ok := r.agent.Page().loader.CompletedURL(u.String())
		if !ok || res.URL == nil || res.Status < 200 || res.Status >= 300 {
			return nil, nil
		}
		return r.val(map[string]any{"body": string(res.Body), "url": res.URL.String(), "crossOrigin": res.URL.Scheme != r.documentURL().Scheme || res.URL.Host != r.documentURL().Host}), nil
	})

	host["notificationPermission"] = r.fn(func(_ engine.Value, _ []engine.Value) (engine.Value, error) {
		permission := r.agent.Page().Environment().Permissions["notifications"]
		if permission != "granted" && permission != "denied" {
			permission = "default"
		}
		return r.val(permission), nil
	})
	host["documentLastModified"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		if r.lastModified.IsZero() {
			return r.val(0), nil
		}
		return r.val(r.lastModified.UnixMilli()), nil
	})
	host["createDocumentFragment"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.val(nodeData(r.document.CreateDocumentFragment())), nil
	})
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
