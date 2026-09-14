(() => {
  const result = {};
  for (const condition of [
    'selector(div)',
    'selector(div > .x)',
    'selector(:has(> img))',
    'selector(:is(div, .x))',
    'selector(::before)',
    'selector(:unknown)',
    'selector(div, span)',
    'selector(:is(div, :unknown))',
    'selector(:where(:unknown))',
    'selector([x="and or"])',
    'not selector(div)',
    '(selector(div))',
    'selector(div) and (display: grid)',
    'selector(div) or selector(:unknown)',
    '(display: grid) and not (color: red)',
    '(--custom: hello)',
    '(--custom:)',
    'color: red',
    '(color: red !important)',
    '(color: red) /* comment */',
    'not (display: grid) and (color: red)',
  ])
    result[condition] = CSS.supports(condition);
  return result;
})();
