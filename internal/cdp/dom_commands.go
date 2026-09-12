package cdp

import (
	"context"
	"fmt"

	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/dom"
)

func coordinateValue(value any) float64 {
	switch value := value.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func (s *session) nodeID(ctx context.Context, p map[string]any) (int64, error) {
	if object := stringValue(p["objectId"]); object != "" {
		return s.runtimeDebugger().RequestNode(ctx, object)
	}
	id := int64(intValue(p["nodeId"], intValue(p["backendNodeId"], 0)))
	d, ok := s.page.Document()
	if !ok {
		return 0, fmt.Errorf("No document")
	}
	if _, ok := d.Get(id); !ok {
		return 0, fmt.Errorf("Could not find node with given id")
	}
	return id, nil
}

func (s *session) nodeFunction(ctx context.Context, p map[string]any, fn string, args []any) (any, error) {
	id, err := s.nodeID(ctx, p)
	if err != nil {
		return nil, err
	}
	frame, ok := s.page.FrameForDOMNode(id)
	if !ok {
		return nil, fmt.Errorf("Node does not belong to the document")
	}
	objectID := stringValue(p["objectId"])
	d := s.runtimeDebugger()
	if objectID == "" {
		object, err := d.ResolveNode(ctx, frame.ID, "", id, "")
		if err != nil {
			return nil, err
		}
		objectID = stringValue(object["objectId"])
		defer d.ReleaseObject(ctx, objectID)
	}
	result, err := d.CallFunction(ctx, frame.ID, "", fn, map[string]any{"objectId": objectID, "arguments": args}, browser.DebuggerOptions{ReturnByValue: true})
	if err != nil {
		return nil, err
	}
	if ex := result["exceptionDetails"]; ex != nil {
		return nil, fmt.Errorf("DOM operation failed: %v", ex)
	}
	value, _ := result["result"].(map[string]any)
	return value["value"], nil
}

func (s *session) frameElementOffset(ctx context.Context, frame *browser.Frame) (float64, float64, error) {
	parent := frame.Parent()
	if parent == nil {
		return 0, 0, nil
	}
	d := s.runtimeDebugger()
	object, err := d.ResolveNode(ctx, parent.ID, "", frame.ElementNodeID(), "")
	if err != nil {
		return 0, 0, err
	}
	objectID := stringValue(object["objectId"])
	defer d.ReleaseObject(ctx, objectID)
	result, err := d.CallFunction(ctx, parent.ID, "", `function(){const r=this.getBoundingClientRect();return {x:r.x,y:r.y}}`, map[string]any{"objectId": objectID}, browser.DebuggerOptions{ReturnByValue: true})
	if err != nil {
		return 0, 0, err
	}
	if ex := result["exceptionDetails"]; ex != nil {
		return 0, 0, fmt.Errorf("iframe geometry failed: %v", ex)
	}
	remote, _ := result["result"].(map[string]any)
	offset, _ := remote["value"].(map[string]any)
	return coordinateValue(offset["x"]), coordinateValue(offset["y"]), nil
}

func (s *session) describeNode(id int64, depth int) (map[string]any, error) {
	d, ok := s.page.Document()
	if !ok {
		return nil, fmt.Errorf("No document")
	}
	n, ok := d.Get(id)
	if !ok {
		return nil, fmt.Errorf("Could not find node with given id")
	}
	result := cdpNode(d, n, depth)
	var visit func(*browser.Frame)
	visit = func(frame *browser.Frame) {
		if frame.ElementNodeID() == id {
			result["frameId"] = frame.ID
		}
		for _, child := range frame.Children() {
			visit(child)
		}
	}
	visit(s.page.Top)
	return result, nil
}

func (s *session) handleDOM(ctx context.Context, method string, p map[string]any) (any, bool, error) {
	empty := map[string]any{}
	switch method {
	case "DOM.resolveNode":
		id, err := s.nodeID(ctx, p)
		if err != nil {
			return nil, true, err
		}
		frameID, realmID, err := s.runtimeContext(int64(intValue(p["executionContextId"], 0)))
		if err != nil {
			return nil, true, err
		}
		if frameID == "" {
			if frame, ok := s.page.FrameForDOMNode(id); ok {
				frameID = frame.ID
			}
		}
		object, err := s.runtimeDebugger().ResolveNode(ctx, frameID, realmID, id, stringValue(p["objectGroup"]))
		return map[string]any{"object": object}, true, err
	case "DOM.requestNode":
		id, err := s.runtimeDebugger().RequestNode(ctx, stringValue(p["objectId"]))
		if err == nil && id != 0 {
			if d, ok := s.page.Document(); ok {
				s.emitDOMAncestors(d, id)
			}
		}
		return map[string]any{"nodeId": id}, true, err
	case "DOM.describeNode":
		id, err := s.nodeID(ctx, p)
		if err != nil {
			return nil, true, err
		}
		node, err := s.describeNode(id, intValue(p["depth"], 0))
		return map[string]any{"node": node}, true, err
	case "DOM.getAttributes":
		id, err := s.nodeID(ctx, p)
		if err != nil {
			return nil, true, err
		}
		node, err := s.describeNode(id, 0)
		if err != nil {
			return nil, true, err
		}
		attrs := node["attributes"]
		if attrs == nil {
			return nil, true, fmt.Errorf("Node is not an Element")
		}
		return map[string]any{"attributes": attrs}, true, nil
	case "DOM.requestChildNodes":
		id, err := s.nodeID(ctx, p)
		if err != nil {
			return nil, true, err
		}
		d, _ := s.page.Document()
		children := []any{}
		depth := intValue(p["depth"], 1)
		if depth == 0 {
			return nil, true, fmt.Errorf("Depth should be a positive number or -1")
		}
		for _, child := range d.Children(id) {
			children = append(children, cdpNode(d, child, depth-1))
		}
		s.event("DOM.setChildNodes", map[string]any{"parentId": id, "nodes": children})
		return empty, true, nil
	case "DOM.focus":
		_, err := s.nodeFunction(ctx, p, `function(){if(!this.isConnected||typeof this.focus!=='function')throw new Error('Element is not focusable');this.focus()}`, nil)
		return empty, true, err
	case "DOM.scrollIntoViewIfNeeded":
		_, err := s.nodeFunction(ctx, p, `function(){if(!this.isConnected)throw new Error('Node is detached from document');this.scrollIntoView({block:'center',inline:'center'})}`, nil)
		return empty, true, err
	case "DOM.getContentQuads", "DOM.getBoxModel":
		id, idErr := s.nodeID(ctx, p)
		if idErr != nil {
			return nil, true, idErr
		}
		value, err := s.nodeFunction(ctx, p, `function(){if(!this.isConnected||this.nodeType!==1)throw new Error('Could not compute box model');const r=this.getBoundingClientRect(),s=getComputedStyle(this),n=k=>parseFloat(s[k])||0,q=(l,t,r,b)=>[l,t,r,t,r,b,l,b];if(s.display==='none'||!r.width||!r.height)throw new Error('Could not compute box model');const padding=q(r.left+n('borderLeftWidth'),r.top+n('borderTopWidth'),r.right-n('borderRightWidth'),r.bottom-n('borderBottomWidth'));return {border:q(r.left,r.top,r.right,r.bottom),padding,content:q(padding[0]+n('paddingLeft'),padding[1]+n('paddingTop'),padding[2]-n('paddingRight'),padding[5]-n('paddingBottom')),margin:q(r.left-n('marginLeft'),r.top-n('marginTop'),r.right+n('marginRight'),r.bottom+n('marginBottom')),width:r.width,height:r.height}}`, nil)
		if err != nil {
			return nil, true, err
		}
		model, _ := value.(map[string]any)
		frame, _ := s.page.FrameForDOMNode(id)
		for frame != nil && frame.Parent() != nil {
			dx, dy, offsetErr := s.frameElementOffset(ctx, frame)
			if offsetErr != nil {
				return nil, true, offsetErr
			}
			for _, name := range []string{"border", "padding", "content", "margin"} {
				quad, _ := model[name].([]any)
				for index := range quad {
					coordinate := coordinateValue(quad[index])
					if index%2 == 0 {
						quad[index] = coordinate + dx
					} else {
						quad[index] = coordinate + dy
					}
				}
			}
			frame = frame.Parent()
		}
		if method == "DOM.getContentQuads" {
			quad, _ := model["content"].([]any)
			return map[string]any{"quads": []any{quad}}, true, nil
		}
		return map[string]any{"model": model}, true, nil
	case "DOM.setAttributeValue", "DOM.removeAttribute", "DOM.setNodeValue", "DOM.setOuterHTML", "DOM.removeNode":
		fn := ""
		args := []any{}
		arg := func(v any) { args = append(args, map[string]any{"value": v}) }
		switch method {
		case "DOM.setAttributeValue":
			fn = `function(n,v){this.setAttribute(n,v)}`
			arg(p["name"])
			arg(p["value"])
		case "DOM.removeAttribute":
			fn = `function(n){this.removeAttribute(n)}`
			arg(p["name"])
		case "DOM.setNodeValue":
			fn = `function(v){if(![3,4,8].includes(this.nodeType))throw new Error('Node is not a character data node');this.nodeValue=v}`
			arg(p["value"])
		case "DOM.setOuterHTML":
			fn = `function(v){this.outerHTML=v}`
			arg(p["outerHTML"])
		case "DOM.removeNode":
			fn = `function(){if(!this.parentNode)throw new Error('Node has no parent');this.parentNode.removeChild(this)}`
		}
		_, err := s.nodeFunction(ctx, p, fn, args)
		return empty, true, err
	}
	return nil, false, nil
}

func (s *session) emitDOMAncestors(d *dom.Document, id int64) {
	n, ok := d.Get(id)
	if !ok || n.Parent == 0 {
		return
	}
	s.emitDOMAncestors(d, n.Parent)
	children := []any{}
	for _, child := range d.Children(n.Parent) {
		children = append(children, cdpNode(d, child, 0))
	}
	s.event("DOM.setChildNodes", map[string]any{"parentId": n.Parent, "nodes": children})
}
