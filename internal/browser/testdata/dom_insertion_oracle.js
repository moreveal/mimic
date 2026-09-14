(() => {
  const output = {}, host = document.createElement('section');
  document.body.appendChild(host);
  const source = document.createElement('div'), target = document.createElement('div');
  host.append(source, target);
  const child = document.createElement('span'), foreign = document.createElement('i');
  source.append(child, foreign);
  const failure = callback => { try { callback(); return 'none'; } catch (error) { return error.name; } };
  output.reference = failure(() => target.insertBefore(child, foreign));
  output.referenceAtomic = child.parentNode === source && source.firstChild === child && !target.firstChild;
  output.selfReference = failure(() => target.insertBefore(child, child));
  output.cycle = failure(() => child.appendChild(source));
  output.cycleAtomic = source.parentNode === host && child.parentNode === source;
  output.sameChild = source.insertBefore(child, child) === child && source.firstChild === child;
  const invalidObserver = new MutationObserver(() => {});
  invalidObserver.observe(source, { childList: true, subtree: true, attributes: true });
  invalidObserver.observe(target, { childList: true });
  output.observedReference = failure(() => target.insertBefore(child, foreign));
  output.observedReplacement = failure(() => target.replaceChild(child, foreign));
  output.observedAtomic = child.parentNode === source && source.firstChild === child && !target.firstChild && invalidObserver.takeRecords().length === 0;
  invalidObserver.disconnect();
  const other = document.implementation.createHTMLDocument('inert');
  output.creation = ['createElement', 'createTextNode', 'createComment'].map(name => other[name]('div').ownerDocument === other);
  const fragment = other.createDocumentFragment(), original = [];
  for (let i = 0; i < 32; i++) { const node = other.createElement('b'); node.textContent = String(i); fragment.appendChild(node); original.push(node); }
  const sentinel = document.createElement('em'); target.appendChild(sentinel);
  const observer = new MutationObserver(() => {});
  observer.observe(fragment, { childList: true }); observer.observe(target, { childList: true });
  target.insertBefore(fragment, sentinel);
  output.fragment = { empty: !fragment.firstChild, identity: original.every((node, index) => target.childNodes[index] === node), owner: original.every(node => node.ownerDocument === document), sentinel: target.lastChild === sentinel, sourceOwner: fragment.ownerDocument === other };
  output.records = observer.takeRecords().map(record => ({ target: record.target === fragment ? 'fragment' : 'target', added: record.addedNodes.length, removed: record.removedNodes.length, previous: record.previousSibling === null, next: record.nextSibling === sentinel }));
  observer.disconnect();
  output.leaves = ['createTextNode', 'createComment'].map(name => { const node = document[name]('leaf'); target.appendChild(node); return node.parentNode === target && node.ownerDocument === document; });
  host.remove();
  return output;
})()
