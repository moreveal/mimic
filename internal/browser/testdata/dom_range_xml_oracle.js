(() => {
  const root = document.createElement('div');
  root.innerHTML = 'a<section><i>b</i></section><!--c-->';
  const filter = (node) =>
    node.localName === 'section' ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT;
  const iterator = document.createNodeIterator(root, NodeFilter.SHOW_ALL, filter);
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ALL, filter);
  const collect = (traversal) => {
    const result = [];
    for (let node; (node = traversal.nextNode()); )
      result.push(node.localName || node.nodeName + ':' + node.data);
    return result;
  };

  const rangeRoot = document.createElement('div');
  rangeRoot.innerHTML = '<b>hello</b><i> world</i>';
  const range = document.createRange();
  range.setStart(rangeRoot.firstChild.firstChild, 1);
  range.setEnd(rangeRoot.lastChild.firstChild, 3);
  const cloned = range.cloneContents();

  const parser = new DOMParser();
  const xml = parser.parseFromString(
    '<r xmlns="urn:r" xmlns:p="urn:p"><p:c p:a="&quot;&amp;">x&lt;y</p:c></r>',
    'application/xml',
  );
  const serialized = new XMLSerializer().serializeToString(xml);
  const roundTrip = parser.parseFromString(serialized, 'application/xml');
  return {
    iterator: collect(iterator),
    walker: collect(walker),
    range: [String(range), cloned.firstChild.localName, cloned.textContent],
    xml: [
      serialized,
      roundTrip.documentElement.firstChild.namespaceURI,
      roundTrip.documentElement.firstChild.getAttributeNS('urn:p', 'a'),
      roundTrip.documentElement.firstChild.textContent,
    ],
  };
})()
