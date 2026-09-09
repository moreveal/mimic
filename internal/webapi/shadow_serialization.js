// Shadow attachment state lives in the realm, while every materialized child
// remains a canonical DOM node. Export only IDs and immutable attachment data.
{
  const serializeShadow = root => {
    const children = Array.from(root.childNodes);
    return children.length ? host.serializeNodeList(children.map(n => elementSlot(n).nodeId)) : fragmentState(root).html || '';
  };
  Object.defineProperty(ShadowRoot.prototype, 'innerHTML', {
    configurable: true, enumerable: true,
    get() { if (!shadowSlots.has(this)) throw new TypeError('Illegal invocation'); return serializeShadow(this); },
    set(value) {
      if (!shadowSlots.has(this)) throw new TypeError('Illegal invocation');
      // Fragment parsing uses the shadow host's context, not template insertion
      // mode. Create the parser context directly to avoid invoking a custom
      // element constructor for a temporary, unobservable parsing container.
      const owner = shadowSlots.get(this).host;
      const parsed = wrap(host.createNS(owner.namespaceURI, owner.localName));
      host.setInnerHTML(elementSlot(parsed).nodeId, value == null ? '' : String(value));
      const children = Array.from(parsed.childNodes);
      for (const child of children) parsed.removeChild(child);
      this.replaceChildren(...children);
      fragmentState(this).html = '';
    }
  });
  host.registerShadowSnapshot(() => {
    const result = [], seen = new Set();
    const documentStyles = constructedStyleSheets.snapshot(document);
    if (documentStyles.length) result.push({hostID:host.documentRootID(), styles:documentStyles});
    const visit = node => {
      if (seen.has(node)) return;
      seen.add(node);
      const root = elementShadows.get(node);
      if (root) {
        const state = shadowSlots.get(root), children = Array.from(root.childNodes);
        result.push({hostID:elementSlot(node).nodeId, mode:state.mode, delegatesFocus:state.delegatesFocus,
          children:children.map(n => elementSlot(n).nodeId), html:children.length ? '' : fragmentState(root).html || '',
          styles:constructedStyleSheets.snapshot(root)});
        for (const child of children) visit(child);
      }
      for (const child of Array.from(node.childNodes || [])) visit(child);
    };
    visit(document);
    return JSON.stringify(result);
  });
}
