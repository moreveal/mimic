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
  registerBootstrapCallback('registerShadowSnapshot',() => {
    const result = [], seen = new Set();
    const documentStyles = constructedStyleSheets.snapshot(document);
    if (documentStyles.length) result.push({hostID:host.documentRootID(), styles:documentStyles});
    const visit = node => {
      if (seen.has(node)) return;
      seen.add(node);
      let root = elementShadows.get(node);
      // ShadyDOM keeps its logical shadow tree in a DocumentFragment and
      // exposes it through the public wrapper even when native shadowRoot is
      // intentionally hidden. Adopt that root into the authoritative snapshot
      // registry without changing the polyfill's observable DOM projection.
      if (!root && node instanceof Element && globalThis.ShadyDOM?.inUse && typeof globalThis.ShadyDOM.wrap === 'function') {
        try {
          const candidate = globalThis.ShadyDOM.wrap(node)?.shadowRoot;
          if (candidate && (candidate instanceof ShadowRoot || globalThis.ShadyDOM.isShadyRoot?.(candidate))) {
            const option = (name, fallback) => { try { const value = candidate[name]; return value === undefined ? fallback : value; } catch { return fallback; } };
            if (!shadowSlots.has(candidate)) shadowSlots.set(candidate, {host:node, mode:String(option('mode', 'open')), delegatesFocus:!!option('delegatesFocus', false),
              slotAssignment:String(option('slotAssignment', 'named')), serializable:!!option('serializable', false), clonable:!!option('clonable', false), onslotchange:null});
            elementShadows.set(node, candidate);
            root = candidate;
          }
        } catch {}
      }
      if (root) {
        const state = shadowSlots.get(root), children = Array.from(root.childNodes);
        result.push({hostID:elementSlot(node).nodeId, mode:state.mode, delegatesFocus:state.delegatesFocus,
          children:children.map(n => elementSlot(n).nodeId), html:children.length ? '' : fragmentState(root)?.html || '',
          styles:constructedStyleSheets.snapshot(root)});
        for (const child of children) visit(child);
      }
      for (const child of Array.from(node.childNodes || [])) visit(child);
    };
    visit(document);
    return JSON.stringify(result);
  });
}
