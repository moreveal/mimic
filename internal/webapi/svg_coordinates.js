const svgOwner = (n) => {
  for (let p = parent(n); elementSlot(p); p = parent(p))
    if (tag(p) === 'svg' && elementSlot(p).namespaceURI === ns) return p;
  return null;
};
const svgElementCheck = (value) => {
  const d = elementSlot(value);
  if (!d || d.namespaceURI !== ns) throw new TypeError('Illegal invocation');
  return value;
};
for (const name of ['ownerSVGElement', 'viewportElement'])
  svgProp('SVGElement', name, function () {
    return svgOwner(svgElementCheck(this));
  });
const svgCTM = (n, screen) => {
  let connected = false,
    hidden = false;
  for (let p = n; isDOMNode(p); p = parent(p)) {
    if (p === document || elementSlot(p)?.type === 'document') connected = true;
    if (elementSlot(p)?.type === 'element' && displayNone(p)) hidden = true;
  }
  if (!connected) return svgMatrix();
  let m = ident,
    outer = null;
  for (let p = n; isDOMNode(p) && elementSlot(p)?.namespaceURI === ns; p = parent(p)) {
    const local = hidden && p === n ? ident : transform(p, bounds(p));
    m = multiply(tag(p) === 'svg' ? multiply(local, viewportTransform(p)) : local, m);
    if (tag(p) === 'svg') {
      outer = p;
      if (!screen && p !== n) break;
      if (!screen && p === n && !svgOwner(p)) break;
    }
  }
  if (screen && outer) {
    const zoom = svgViewportState(outer);
    m = multiply([zoom.scale, 0, 0, zoom.scale, zoom.translate.x, zoom.translate.y], m);
    const body = parent(outer);
    if (tag(body) !== 'body') svgFail('screenCTM', 'HTML ancestor layout is unsupported');
    const siblings = childElements(body);
    if (
      siblings
        .slice(0, siblings.indexOf(outer))
        .some((n) => !['script', 'style', 'link'].includes(tag(n)) && !displayNone(n))
    )
      svgFail('screenCTM', 'preceding HTML flow layout is unsupported');
    const entries = computedCSSDeclarations(body),
      css = (key) => entries.find((e) => e.name === key)?.value;
    const value = (key, fallback) => {
      const raw = css(key);
      if (raw == null) return fallback;
      const v = cssResolveLength(raw, {
        em: cssComputedFontSize(body),
        rem: cssComputedFontSize(parent(body)),
        percent: 0,
      });
      if (v === null) svgFail('screenCTM', 'unsupported body box length');
      return v;
    };
    if (css('transform') && css('transform') !== 'none')
      svgFail('screenCTM', 'HTML ancestor transform is unsupported');
    const x = value('margin-left', 8) + value('padding-left', 0),
      y = value('margin-top', 8) + value('padding-top', 0);
    m = multiply([1, 0, 0, 1, x, y], m);
  }
  return svgMatrix(m);
};
svgMethod('SVGGraphicsElement', 'getCTM', function () {
  return svgCTM(check(this), false);
});
svgMethod('SVGGraphicsElement', 'getScreenCTM', function () {
  return svgCTM(check(this), true);
});
svgMethod('SVGSVGElement', 'getElementById', function (id) {
  const n = check(this);
  if (tag(n) !== 'svg') throw new TypeError('Illegal invocation');
  id = String(id);
  const walk = (root) => {
    for (const child of childElements(root)) {
      if (attr(child, 'id') === id) return child;
      const found = walk(child);
      if (found) return found;
    }
    return null;
  };
  return walk(n);
});
