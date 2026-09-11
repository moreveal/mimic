(() => {
  const out = {}, impl = document.implementation;
  const attempt = fn => { try { fn(); return 'ok'; } catch (e) { return e.name; } };
  const d = impl.createHTMLDocument('body setter'), old = d.body;
  const body = document.createElement('body');
  body.appendChild(document.createElement('span'));
  out.replace = attempt(() => { 'use strict'; d.body = body; });
  out.identity = [d.body === body, body.parentNode === d.documentElement,
    old.parentNode === null, body.ownerDocument === d, body.firstChild.ownerDocument === d];
  out.same = attempt(() => { d.body = body; });
  out.invalid = [null, undefined, {}, 'body', d.createElement('div'),
    d.createElementNS('urn:test', 'body'), d.createTextNode('x')].map(value => attempt(() => { d.body = value; }));
  out.unchanged = d.body === body;
  const frameset = d.createElement('frameset');
  out.frameset = attempt(() => { d.body = frameset; });
  out.framesetIdentity = d.body === frameset;
  frameset.remove();
  out.append = attempt(() => { d.body = body; });
  out.appendIdentity = d.body === body && d.documentElement.lastChild === body;
  const empty = impl.createDocument(null, '', null);
  out.noRoot = attempt(() => { empty.body = body; });
  const xml = impl.createDocument('urn:test', 'root', null);
  out.xml = attempt(() => { xml.body = body; });
  out.xmlState = [xml.body === null, body.parentNode === xml.documentElement, body.ownerDocument === xml];
  const setter = Object.getOwnPropertyDescriptor(Document.prototype, 'body').set;
  out.setter = typeof setter;
  out.brand = attempt(() => setter.call({}, body));
  return out;
})()
