// Typed specified-value serialization shared by inline and stylesheet CSSOM.
const cssSignedLength = (value) => {
  const term = cssLengthTerm(value);
  if (term) return cssSerializeNumber(term.number) + term.unit;
  if (value === '0') return '0px';
  // A declaration containing var() is validated after substitution. Preserve
  // it here so geometry can resolve Tailwind-style calc(var(--spacing)*n).
  if (/^(?:calc\()?var\(/i.test(value)) return value;
  return cssLengthValue(value);
};
const cssTransformTerm = (value, type, serialize = cssSerializeNumber) => {
  const primitive = (input) => {
    if (type === 'number' || type === 'scalar') {
      if (cssNumberRegex.test(input)) return { number: Number(input), unit: '' };
      if (type === 'number' && input.endsWith('%') && cssNumberRegex.test(input.slice(0, -1)))
        return { number: Number(input.slice(0, -1)) / 100, unit: '' };
      return null;
    }
    if (type === 'angle') {
      if (input === '0') return { number: 0, unit: 'deg' };
      const m = cssDimensionRegex.exec(input);
      return m && ['deg', 'rad', 'grad', 'turn'].includes(m[2])
        ? { number: Number(m[1]), unit: m[2] }
        : null;
    }
    return cssLengthTerm(input) || (input === '0' ? { number: 0, unit: 'px' } : null);
  };
  const calc = /^calc\(\s*([^\s()]+)(?:\s+([+-])\s+([^\s()]+))?\s*\)$/i.exec(value);
  if (calc) {
    let a = primitive(calc[1]),
      b = calc[3] ? primitive(calc[3]) : null;
    if (!a || (calc[3] && !b)) return null;
    if (type === 'angle') {
      const scales = { deg: 1, rad: 180 / Math.PI, grad: 0.9, turn: 360 };
      a = { number: a.number * scales[a.unit], unit: 'deg' };
      if (b) b = { number: b.number * scales[b.unit], unit: 'deg' };
    }
    if (b && a.unit !== b.unit) return type === 'length' ? cssLengthValue(value) : null;
    return (
      'calc(' +
      serialize(a.number + (b ? (calc[2] === '-' ? -b.number : b.number) : 0)) +
      a.unit +
      ')'
    );
  }
  const term = primitive(value);
  return term ? serialize(term.number) + term.unit : null;
};
const parseCSSTransform = (value, serialize = cssSerializeNumber) => {
  if (value === 'none') return value;
  if (/\b(?:var|env)\s*\(/i.test(value)) return value;
  const functions = {
      matrix: [6, 6, 'scalar'],
      matrix3d: [16, 16, 'scalar'],
      translate: [1, 2, 'length'],
      translatex: [1, 1, 'length'],
      translatey: [1, 1, 'length'],
      translatez: [1, 1, 'length'],
      translate3d: [3, 3, 'length'],
      scale: [1, 2, 'number'],
      scalex: [1, 1, 'number'],
      scaley: [1, 1, 'number'],
      scalez: [1, 1, 'number'],
      scale3d: [3, 3, 'number'],
      rotate: [1, 1, 'angle'],
      rotatex: [1, 1, 'angle'],
      rotatey: [1, 1, 'angle'],
      rotatez: [1, 1, 'angle'],
      rotate3d: [4, 4, 'scalar'],
      skew: [1, 2, 'angle'],
      skewx: [1, 1, 'angle'],
      skewy: [1, 1, 'angle'],
      perspective: [1, 1, 'length'],
    },
    out = [];
  let index = 0;
  while (index < value.length) {
    while (/\s/.test(value[index] || '') && index < value.length) index++;
    if (index === value.length) break;
    const match = /^([a-zA-Z0-9]+)\(/.exec(value.slice(index));
    if (!match) return null;
    const key = match[1].toLowerCase(),
      spec = functions[key];
    if (!spec) return null;
    const start = index + match[0].length;
    let end = start,
      depth = 1;
    for (; end < value.length && depth; end++) {
      if (value[end] === '(') depth++;
      else if (value[end] === ')') depth--;
    }
    if (depth) return null;
    const args = cssSplitTopLevel(value.slice(start, end - 1), ',');
    if (!args || args.length < spec[0] || args.length > spec[1]) return null;
    if (
      args.some(
        (v, i) =>
          v.includes('%') &&
          (key === 'perspective' || key === 'translatez' || (key === 'translate3d' && i === 2)),
      )
    )
      return null;
    const converted = args.map((v, i) =>
      key === 'perspective' && v.trim() === 'none'
        ? 'none'
        : cssTransformTerm(v.trim(), key === 'rotate3d' && i === 3 ? 'angle' : spec[2], serialize),
    );
    if (key === 'perspective' && converted[0] !== null && Number.parseFloat(converted[0]) < 0)
      return null;
    if (converted.includes(null)) return null;
    const name = key.replace(
      /^(translate|scale|rotate|skew)([xyz])$/,
      (_, prefix, axis) => prefix + axis.toUpperCase(),
    );
    out.push(name + '(' + converted.join(', ') + ')');
    index = end;
  }
  return out.length ? out.join(' ') : null;
};
const parseCSSShadow = (value, text = false) => {
  if (value === 'none') return value;
  const layers = cssSplitTopLevel(value, ',');
  if (!layers) return null;
  const out = [];
  for (const layer of layers) {
    const tokens = cssValueTokens(layer.replace(/(?<=[\d.)])#/g, ' #'));
    if (!tokens) return null;
    let color = null,
      inset = false;
    const lengths = [];
    for (const token of tokens) {
      const c = cssColorValue(token),
        length = cssSignedLength(token);
      if (c !== null) {
        if (color !== null) return null;
        color = c;
      } else if (token === 'inset' && !text) {
        if (inset) return null;
        inset = true;
      } else if (length !== null && !length.endsWith('%')) lengths.push(length);
      else return null;
    }
    if (
      lengths.length < 2 ||
      lengths.length > (text ? 3 : 4) ||
      (lengths.length >= 3 && Number.parseFloat(lengths[2]) < 0)
    )
      return null;
    out.push([color, ...lengths, inset ? 'inset' : ''].filter(Boolean).join(' '));
  }
  return out.join(', ');
};
// Retain the parsed precision in the authoritative declaration record. Public
// CSSOM text is a rounded projection, never the input to geometric observations.
// Blink's simple transform parser retains seven fractional digits. Complex
// lists use the general numeric parser; eligibility applies to the whole list.
const cssTransformNumericInput = (value) => {
  const parts = String(value)
    .trim()
    .match(/[a-zA-Z0-9]+\([^()]*\)/g);
  if (
    !parts ||
    parts.map((v) => v.replace(/\s+/g, '')).join('') !== String(value).replace(/\s+/g, '')
  )
    return value;
  for (const part of parts) {
    if (part.length < 12) return value;
    const m = /^(translate(?:[XYZxyz]|3d)?|matrix3d|scale3d|rotate[Zz]?)\((.*)\)$/.exec(part);
    if (!m) return value;
    const args = m[2].split(',').map((v) => v.trim()),
      name = m[1].toLowerCase();
    const count = name === 'matrix3d' ? 16 : name.endsWith('3d') ? 3 : name === 'translate' ? 2 : 1;
    if (args.length !== count) return value;
    const unit = name.startsWith('rotate')
      ? '(?:deg|rad|grad|turn)'
      : name.startsWith('translate')
        ? 'px'
        : '';
    if (
      !args.every(
        (v) =>
          new RegExp('^-?(?:[0-9]+(?:\\.[0-9]+)?|\\.[0-9]+)' + unit + '$', 'i').test(v) ||
          Number(v) === 0,
      )
    )
      return value;
  }
  return parts
    .join(' ')
    .replace(
      /(-?)([0-9]*)\.([0-9]{8,})/g,
      (_, sign, whole, fraction) =>
        sign +
        String(Number(whole || 0) + Number(fraction.slice(0, 7)) * 0.000000100000000000000009),
    );
};
const cssPrecisionDeclaration = (entry, input) => {
  if (entry.name === 'transform') {
    const parsed = parseCSSTransform(cssTransformNumericInput(String(input).trim()), (number) =>
      String(Math.max(-3.4028234663852886e38, Math.min(3.4028234663852886e38, number))),
    );
    if (parsed !== null)
      entry.parsedValue = parsed.replace(/calc\(([^()]+)\)/g, (_, term) =>
        cssDimensionRegex.test(term) || cssNumberRegex.test(term) ? term : 'calc(' + term + ')',
      );
  }
  return entry;
};
cssLonghandParsers.set('transform', parseCSSTransform);
cssLonghandParsers.set('content-visibility', cssKeywordValue(['visible', 'auto', 'hidden']));
for (const name of [
  'color',
  'background-color',
  'outline-color',
  'fill',
  'stroke',
  'stop-color',
  'flood-color',
  'lighting-color',
])
  cssLonghandParsers.set(name, (value) =>
    ['fill', 'stroke'].includes(name) && /^(?:none|url\()/i.test(value)
      ? value
      : cssColorValue(value),
  );
for (const name of ['opacity', 'fill-opacity', 'stroke-opacity', 'stop-opacity', 'flood-opacity'])
  cssLonghandParsers.set(name, (value) => cssTransformTerm(value, 'number'));
cssLonghandParsers.set('box-shadow', (value) => parseCSSShadow(value));
cssLonghandParsers.set('text-shadow', (value) => parseCSSShadow(value, true));
cssLonghandParsers.set('transform-origin', (value) => {
  const words = cssValueTokens(value);
  if (!words || words.length < 1 || words.length > 3) return null;
  const converted = words.map((v) =>
    ['left', 'right', 'top', 'bottom', 'center'].includes(v) ? v : cssSignedLength(v),
  );
  if (converted.includes(null)) return null;
  if (converted.length === 1)
    return ['top', 'bottom'].includes(converted[0])
      ? 'center ' + converted[0]
      : converted[0] + ' center';
  return converted.join(' ');
});
for (const name of ['grid-area', 'grid-row', 'grid-column'])
  cssLonghandParsers.set(
    name,
    (value) =>
      cssSplitTopLevel(value, '/')
        ?.map((v) => v.trim())
        .join(' / ') ?? null,
  );
cssLonghandParsers.set('stroke-dasharray', (value) => {
  if (value === 'none') return value;
  const terms = value
    .split(/[\s,]+/)
    .filter(Boolean)
    .map((v) => (cssNumberRegex.test(v) ? cssSerializeNumber(Number(v)) : cssSignedLength(v)));
  return !terms.length || terms.some((v) => v === null || Number.parseFloat(v) < 0)
    ? null
    : terms.join(', ');
});

cssLonghandParsers.set(
  'font-family',
  (value) =>
    cssSplitTopLevel(value, ',')
      ?.map((v) => {
        v = v.trim();
        return /^[a-zA-Z_-][\w-]*(?:\s+[a-zA-Z_-][\w-]*)+$/.test(v)
          ? JSON.stringify(v.replace(/\s+/g, ' '))
          : v;
      })
      .join(', ') ?? null,
);
cssLonghandParsers.set('background', (value) => cssColorValue(value) ?? value);
const cssOneOrTwoKeywords = (keywords) => (value) => {
  const words = cssValueTokens(value.toLowerCase());
  return words &&
    words.length >= 1 &&
    words.length <= 2 &&
    words.every((word) => keywords.has(word))
    ? words.join(' ')
    : null;
};
for (const name of ['width', 'height', 'min-width', 'min-height', 'max-width', 'max-height'])
  cssLonghandParsers.set(name, (value) => {
    const lower = value.toLowerCase();
    if (
      ['auto', 'none', 'min-content', 'max-content', 'fit-content', 'stretch'].includes(lower) &&
      !(name.startsWith('min-') && lower === 'none')
    )
      return lower;
    return /\bvar\(/i.test(value) ? cssSignedLength(value) : cssLengthValue(value);
  });
cssLonghandParsers.set(
  'position',
  cssKeywordValue(['static', 'relative', 'absolute', 'fixed', 'sticky']),
);
const overflowKeywords = new Set(['visible', 'hidden', 'clip', 'scroll', 'auto']);
cssLonghandParsers.set('overflow', cssOneOrTwoKeywords(overflowKeywords));
for (const name of ['overflow-x', 'overflow-y'])
  cssLonghandParsers.set(name, cssKeywordValue([...overflowKeywords]));
cssLonghandParsers.set('z-index', (value) =>
  value.toLowerCase() === 'auto' || /^[+-]?\d+$/.test(value) ? value.toLowerCase() : null,
);
cssLonghandParsers.set('box-sizing', cssKeywordValue(['content-box', 'border-box']));
cssLonghandParsers.set('visibility', cssKeywordValue(['visible', 'hidden', 'collapse']));
cssLonghandParsers.set(
  'white-space',
  cssKeywordValue([
    'normal',
    'pre',
    'nowrap',
    'pre-wrap',
    'pre-line',
    'break-spaces',
    'collapse wrap',
    'preserve nowrap',
    'preserve wrap',
    'preserve-breaks wrap',
    'break-spaces wrap',
  ]),
);
cssLonghandParsers.set(
  'object-fit',
  cssKeywordValue(['fill', 'contain', 'cover', 'none', 'scale-down']),
);
cssLonghandParsers.set(
  'cursor',
  cssKeywordValue(
    'auto default none context-menu help pointer progress wait cell crosshair text vertical-text alias copy move no-drop not-allowed grab grabbing e-resize n-resize ne-resize nw-resize s-resize se-resize sw-resize w-resize ew-resize ns-resize nesw-resize nwse-resize col-resize row-resize all-scroll zoom-in zoom-out'.split(
      ' ',
    ),
  ),
);
for (const name of ['top', 'right', 'bottom', 'left'])
  cssLonghandParsers.set(name, (value) => (value === 'auto' ? value : cssSignedLength(value)));
cssShorthandComponents.inset = ['top', 'right', 'bottom', 'left'];
cssShorthandParsers.set('inset', (value) =>
  cssFourValues(value, (v) => (v === 'auto' ? v : cssSignedLength(v))),
);
cssShorthandComponents.outline = ['outline-width', 'outline-style', 'outline-color'];
cssShorthandParsers.set('outline', parseCSSBorder);
cssShorthandComponents['border-image'] = cssBorderImageReset;
for (const name of ['gap', 'border-spacing'])
  cssLonghandParsers.set(name, (value) => {
    const words = cssValueTokens(value);
    if (!words || words.length < 1 || words.length > 2) return null;
    const converted = words.map((v) => (name === 'gap' && v === 'normal' ? v : cssLengthValue(v)));
    return converted.includes(null)
      ? null
      : converted.length === 2 && converted[0] === converted[1]
        ? converted[0]
        : converted.join(' ');
  });
