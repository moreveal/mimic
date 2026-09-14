// Standard named color values measured in Chrome 152; shared by CSS and pixel observations.
const cssColorNames = {
  aliceblue: [240.0, 248.0, 255.0, 1],
  antiquewhite: [250.0, 235.0, 215.0, 1],
  aqua: [0.0, 255.0, 255.0, 1],
  aquamarine: [127.0, 255.0, 212.0, 1],
  azure: [240.0, 255.0, 255.0, 1],
  beige: [245.0, 245.0, 220.0, 1],
  bisque: [255.0, 228.0, 196.0, 1],
  black: [0.0, 0.0, 0.0, 1],
  blanchedalmond: [255.0, 235.0, 205.0, 1],
  blue: [0.0, 0.0, 255.0, 1],
  blueviolet: [138.0, 43.0, 226.0, 1],
  brown: [165.0, 42.0, 42.0, 1],
  burlywood: [222.0, 184.0, 135.0, 1],
  cadetblue: [95.0, 158.0, 160.0, 1],
  chartreuse: [127.0, 255.0, 0.0, 1],
  chocolate: [210.0, 105.0, 30.0, 1],
  coral: [255.0, 127.0, 80.0, 1],
  cornflowerblue: [100.0, 149.0, 237.0, 1],
  cornsilk: [255.0, 248.0, 220.0, 1],
  crimson: [220.0, 20.0, 60.0, 1],
  cyan: [0.0, 255.0, 255.0, 1],
  darkblue: [0.0, 0.0, 139.0, 1],
  darkcyan: [0.0, 139.0, 139.0, 1],
  darkgoldenrod: [184.0, 134.0, 11.0, 1],
  darkgray: [169.0, 169.0, 169.0, 1],
  darkgreen: [0.0, 100.0, 0.0, 1],
  darkgrey: [169.0, 169.0, 169.0, 1],
  darkkhaki: [189.0, 183.0, 107.0, 1],
  darkmagenta: [139.0, 0.0, 139.0, 1],
  darkolivegreen: [85.0, 107.0, 47.0, 1],
  darkorange: [255.0, 140.0, 0.0, 1],
  darkorchid: [153.0, 50.0, 204.0, 1],
  darkred: [139.0, 0.0, 0.0, 1],
  darksalmon: [233.0, 150.0, 122.0, 1],
  darkseagreen: [143.0, 188.0, 143.0, 1],
  darkslateblue: [72.0, 61.0, 139.0, 1],
  darkslategray: [47.0, 79.0, 79.0, 1],
  darkslategrey: [47.0, 79.0, 79.0, 1],
  darkturquoise: [0.0, 206.0, 209.0, 1],
  darkviolet: [148.0, 0.0, 211.0, 1],
  deeppink: [255.0, 20.0, 147.0, 1],
  deepskyblue: [0.0, 191.0, 255.0, 1],
  dimgray: [105.0, 105.0, 105.0, 1],
  dimgrey: [105.0, 105.0, 105.0, 1],
  dodgerblue: [30.0, 144.0, 255.0, 1],
  firebrick: [178.0, 34.0, 34.0, 1],
  floralwhite: [255.0, 250.0, 240.0, 1],
  forestgreen: [34.0, 139.0, 34.0, 1],
  fuchsia: [255.0, 0.0, 255.0, 1],
  gainsboro: [220.0, 220.0, 220.0, 1],
  ghostwhite: [248.0, 248.0, 255.0, 1],
  gold: [255.0, 215.0, 0.0, 1],
  goldenrod: [218.0, 165.0, 32.0, 1],
  gray: [128.0, 128.0, 128.0, 1],
  green: [0.0, 128.0, 0.0, 1],
  greenyellow: [173.0, 255.0, 47.0, 1],
  grey: [128.0, 128.0, 128.0, 1],
  honeydew: [240.0, 255.0, 240.0, 1],
  hotpink: [255.0, 105.0, 180.0, 1],
  indianred: [205.0, 92.0, 92.0, 1],
  indigo: [75.0, 0.0, 130.0, 1],
  ivory: [255.0, 255.0, 240.0, 1],
  khaki: [240.0, 230.0, 140.0, 1],
  lavender: [230.0, 230.0, 250.0, 1],
  lavenderblush: [255.0, 240.0, 245.0, 1],
  lawngreen: [124.0, 252.0, 0.0, 1],
  lemonchiffon: [255.0, 250.0, 205.0, 1],
  lightblue: [173.0, 216.0, 230.0, 1],
  lightcoral: [240.0, 128.0, 128.0, 1],
  lightcyan: [224.0, 255.0, 255.0, 1],
  lightgoldenrodyellow: [250.0, 250.0, 210.0, 1],
  lightgray: [211.0, 211.0, 211.0, 1],
  lightgreen: [144.0, 238.0, 144.0, 1],
  lightgrey: [211.0, 211.0, 211.0, 1],
  lightpink: [255.0, 182.0, 193.0, 1],
  lightsalmon: [255.0, 160.0, 122.0, 1],
  lightseagreen: [32.0, 178.0, 170.0, 1],
  lightskyblue: [135.0, 206.0, 250.0, 1],
  lightslategray: [119.0, 136.0, 153.0, 1],
  lightslategrey: [119.0, 136.0, 153.0, 1],
  lightsteelblue: [176.0, 196.0, 222.0, 1],
  lightyellow: [255.0, 255.0, 224.0, 1],
  lime: [0.0, 255.0, 0.0, 1],
  limegreen: [50.0, 205.0, 50.0, 1],
  linen: [250.0, 240.0, 230.0, 1],
  magenta: [255.0, 0.0, 255.0, 1],
  maroon: [128.0, 0.0, 0.0, 1],
  mediumaquamarine: [102.0, 205.0, 170.0, 1],
  mediumblue: [0.0, 0.0, 205.0, 1],
  mediumorchid: [186.0, 85.0, 211.0, 1],
  mediumpurple: [147.0, 112.0, 219.0, 1],
  mediumseagreen: [60.0, 179.0, 113.0, 1],
  mediumslateblue: [123.0, 104.0, 238.0, 1],
  mediumspringgreen: [0.0, 250.0, 154.0, 1],
  mediumturquoise: [72.0, 209.0, 204.0, 1],
  mediumvioletred: [199.0, 21.0, 133.0, 1],
  midnightblue: [25.0, 25.0, 112.0, 1],
  mintcream: [245.0, 255.0, 250.0, 1],
  mistyrose: [255.0, 228.0, 225.0, 1],
  moccasin: [255.0, 228.0, 181.0, 1],
  navajowhite: [255.0, 222.0, 173.0, 1],
  navy: [0.0, 0.0, 128.0, 1],
  oldlace: [253.0, 245.0, 230.0, 1],
  olive: [128.0, 128.0, 0.0, 1],
  olivedrab: [107.0, 142.0, 35.0, 1],
  orange: [255.0, 165.0, 0.0, 1],
  orangered: [255.0, 69.0, 0.0, 1],
  orchid: [218.0, 112.0, 214.0, 1],
  palegoldenrod: [238.0, 232.0, 170.0, 1],
  palegreen: [152.0, 251.0, 152.0, 1],
  paleturquoise: [175.0, 238.0, 238.0, 1],
  palevioletred: [219.0, 112.0, 147.0, 1],
  papayawhip: [255.0, 239.0, 213.0, 1],
  peachpuff: [255.0, 218.0, 185.0, 1],
  peru: [205.0, 133.0, 63.0, 1],
  pink: [255.0, 192.0, 203.0, 1],
  plum: [221.0, 160.0, 221.0, 1],
  powderblue: [176.0, 224.0, 230.0, 1],
  purple: [128.0, 0.0, 128.0, 1],
  rebeccapurple: [102.0, 51.0, 153.0, 1],
  red: [255.0, 0.0, 0.0, 1],
  rosybrown: [188.0, 143.0, 143.0, 1],
  royalblue: [65.0, 105.0, 225.0, 1],
  saddlebrown: [139.0, 69.0, 19.0, 1],
  salmon: [250.0, 128.0, 114.0, 1],
  sandybrown: [244.0, 164.0, 96.0, 1],
  seagreen: [46.0, 139.0, 87.0, 1],
  seashell: [255.0, 245.0, 238.0, 1],
  sienna: [160.0, 82.0, 45.0, 1],
  silver: [192.0, 192.0, 192.0, 1],
  skyblue: [135.0, 206.0, 235.0, 1],
  slateblue: [106.0, 90.0, 205.0, 1],
  slategray: [112.0, 128.0, 144.0, 1],
  slategrey: [112.0, 128.0, 144.0, 1],
  snow: [255.0, 250.0, 250.0, 1],
  springgreen: [0.0, 255.0, 127.0, 1],
  steelblue: [70.0, 130.0, 180.0, 1],
  tan: [210.0, 180.0, 140.0, 1],
  teal: [0.0, 128.0, 128.0, 1],
  thistle: [216.0, 191.0, 216.0, 1],
  tomato: [255.0, 99.0, 71.0, 1],
  turquoise: [64.0, 224.0, 208.0, 1],
  violet: [238.0, 130.0, 238.0, 1],
  wheat: [245.0, 222.0, 179.0, 1],
  white: [255.0, 255.0, 255.0, 1],
  whitesmoke: [245.0, 245.0, 245.0, 1],
  yellow: [255.0, 255.0, 0.0, 1],
  yellowgreen: [154.0, 205.0, 50.0, 1],
  transparent: [0.0, 0.0, 0.0, 0.0],
};
const cssColorRGBA = (value, scheme = 'light') => {
  if (typeof value !== 'string') return null;
  let s = value.trim().toLowerCase();
  if (Object.prototype.hasOwnProperty.call(cssColorNames, s)) return cssColorNames[s].slice();
  if (/^[a-z]+$/.test(s) && typeof host.systemColors === 'function') {
    const system = host.systemColors()[scheme]?.[s];
    if (system && /^rgba?\(/.test(system)) return cssColorRGBA(system, scheme);
  }
  if (/^#[0-9a-f]{3,4}$/.test(s)) s = '#' + Array.from(s.slice(1), (v) => v + v).join('');
  if (/^#[0-9a-f]{6}(?:[0-9a-f]{2})?$/.test(s))
    return [
      parseInt(s.slice(1, 3), 16),
      parseInt(s.slice(3, 5), 16),
      parseInt(s.slice(5, 7), 16),
      s.length === 9 ? parseInt(s.slice(7), 16) / 255 : 1,
    ];
  const match = /^(rgba?|hsla?)\(([^()]*)\)$/.exec(s);
  if (!match) return null;
  const body = match[2].trim(),
    legacy = body.includes(',');
  let a;
  if (legacy) {
    if (body.includes('/')) return null;
    a = body.split(',').map((v) => v.trim());
    if (a.some((v) => !v || /\s/.test(v))) return null;
  } else {
    const parts = body.split('/');
    if (parts.length > 2) return null;
    a = parts[0].trim().split(/\s+/);
    if (a.length !== 3) return null;
    if (parts.length === 2) a.push(parts[1].trim());
  }
  if (a.length < 3 || a.length > 4) return null;
  const numeric = /^[+-]?(?:\d*\.\d+|\d+)(?:e[+-]?\d+)?%?$/i,
    angle = /^[+-]?(?:\d*\.\d+|\d+)(?:e[+-]?\d+)?(?:deg|grad|rad|turn)?$/i;
  if (!a.every((v, i) => (i === 0 && match[1].startsWith('hsl') ? angle : numeric).test(v)))
    return null;
  if (
    legacy &&
    match[1].startsWith('rgb') &&
    a.slice(0, 3).some((v) => v.endsWith('%') !== a[0].endsWith('%'))
  )
    return null;
  const n = (v, scale) => {
      const x = parseFloat(v);
      return Number.isFinite(x) ? x * (v.endsWith('%') ? scale / 100 : 1) : NaN;
    },
    clamp = (v, max) => Math.max(0, Math.min(max, v));
  let alpha = a[3] === undefined ? 1 : clamp(n(a[3], 1), 1),
    rgb;
  if (match[1].startsWith('rgb')) rgb = a.slice(0, 3).map((v) => Math.round(clamp(n(v, 255), 255)));
  else {
    if (!a[1].endsWith('%') || !a[2].endsWith('%')) return null;
    let h = n(a[0], 1);
    if (a[0].endsWith('turn')) h *= 360;
    else if (a[0].endsWith('grad')) h *= 0.9;
    else if (a[0].endsWith('rad')) h *= 180 / Math.PI;
    h = (((h % 360) + 360) % 360) / 60;
    const sat = clamp(n(a[1], 1), 1),
      light = clamp(n(a[2], 1), 1),
      c = (1 - Math.abs(2 * light - 1)) * sat,
      x = c * (1 - Math.abs((h % 2) - 1)),
      m = light - c / 2;
    rgb = (
      [
        [c, x, 0],
        [x, c, 0],
        [0, c, x],
        [0, x, c],
        [x, 0, c],
        [c, 0, x],
      ][Math.floor(h)] || [NaN, NaN, NaN]
    ).map((v) => Math.round((v + m) * 255));
  }
  return [...rgb, alpha].every(Number.isFinite) ? [...rgb, alpha] : null;
};
const cssSerializeColor = (rgba) =>
  rgba[3] === 1
    ? 'rgb(' + rgba.slice(0, 3).join(', ') + ')'
    : 'rgba(' + rgba.slice(0, 3).join(', ') + ', ' + Math.round(rgba[3] * 1000) / 1000 + ')';
