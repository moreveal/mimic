// RGB spaces use D50-adapted profile matrices and the sRGB transfer curve.
// Coefficients describe colorimetry; they are independent of image observations.
const canvasColorSpaces = new Set(['srgb', 'display-p3']);
const canvasWideColor = (value) => {
  const m = /^color\(\s*(srgb|display-p3)\s+([^()]*)\)$/i.exec(String(value).trim());
  if (!m) return null;
  const parts = m[2].trim().split(/\s*\/\s*/);
  if (parts.length > 2) return null;
  const components = parts[0].split(/\s+/);
  if (components.length !== 3) return null;
  const number = (v) =>
    /^[+-]?(?:\d*\.\d+|\d+)(?:e[+-]?\d+)?%?$/i.test(v)
      ? parseFloat(v) * (v.endsWith('%') ? 0.01 : 1)
      : NaN;
  const c = components.map(number),
    alpha = parts.length === 2 ? number(parts[1]) : 1;
  if (![...c, alpha].every(Number.isFinite)) return null;
  return [...c.map((v) => v * 255), Math.max(0, Math.min(1, alpha)) * 255, m[1].toLowerCase()];
};
const canvasGamutMatrices = (() => {
  const F = Math.fround,
    srgb = [
      [0x6fa2, 0x6299, 0x24a0],
      [0x38f5, 0xb785, 0x0f84],
      [0x0390, 0x18da, 0xb6cf],
    ].map((row) => row.map((v) => v / 65536)),
    p3 = [
      [0.515102, 0.291965, 0.157153],
      [0.241182, 0.692236, 0.0665819],
      [-0.00104941, 0.0418818, 0.784378],
    ].map((row) => row.map(F));
  const inverse = (m) => {
    const [a, b, c] = m[0],
      [d, e, f] = m[1],
      [g, h, i] = m[2],
      det = a * (e * i - f * h) - b * (d * i - f * g) + c * (d * h - e * g);
    return [
      [e * i - f * h, c * h - b * i, b * f - c * e],
      [f * g - d * i, a * i - c * g, c * d - a * f],
      [d * h - e * g, b * g - a * h, a * e - b * d],
    ].map((row) => row.map((v) => F(v / det)));
  };
  const multiply = (a, b) =>
    a.map((row) => [0, 1, 2].map((j) => F(row.reduce((n, v, k) => n + v * b[k][j], 0))));
  return { srgb: multiply(inverse(srgb), p3), 'display-p3': multiply(inverse(p3), srgb) };
})();
const canvasConvertColor = (rgba, target) => {
  const from = rgba[4] || 'srgb';
  if (from === target) return rgba.slice(0, 4);
  const decode = (x) => {
      const a = Math.abs(x);
      return Math.sign(x) * (a <= 0.04045 ? a / 12.92 : ((a + 0.055) / 1.055) ** 2.4);
    },
    encode = (x) => {
      const a = Math.abs(x);
      return Math.sign(x) * (a <= 0.0031308 ? 12.92 * a : 1.055 * a ** (1 / 2.4) - 0.055);
    };
  const matrix = canvasGamutMatrices[target];
  const linear = rgba.slice(0, 3).map((v) => decode(v / 255));
  return matrix
    .map((row) => encode(row.reduce((sum, v, i) => sum + v * linear[i], 0)) * 255)
    .concat(rgba[3]);
};
// Paint colors enter the shared extended-sRGB working representation before
// projection into a backing surface. Values outside [0,1] stay unclipped until
// an integer storage/readback format requires it.
const canvasPaintColor = (rgba, target) =>
  canvasConvertColor(
    rgba[4] === 'display-p3' ? canvasConvertColor(rgba, 'srgb').concat('srgb') : rgba,
    target,
  );
// Allocate engine-specific storage after restore, not in the portable bootstrap.
let canvasHalfScratch = null;
const canvasHalf = (value) => {
  if (typeof Float16Array !== 'function')
    fail('NotSupportedError', 'Float16 typed arrays are unavailable in this engine');
  if (!canvasHalfScratch) canvasHalfScratch = new Float16Array(1);
  canvasHalfScratch[0] = value;
  return canvasHalfScratch[0];
};
const canvasPixelArray = (width, height, format) => {
  if (format === 'rgba-unorm8') return storage(width, height);
  if (format !== 'rgba-float16') throw new TypeError('Invalid pixel format');
  if (typeof Float16Array !== 'function')
    fail('NotSupportedError', 'Float16 typed arrays are unavailable in this engine');
  if (width * height > 16777216)
    fail('NotSupportedError', 'Canvas storage above 16 million pixels is not implemented');
  return new Float16Array(width * height * 4);
};
const canvasStore = (pixels, index, value) => {
  pixels[index] =
    pixels instanceof Float32Array ? canvasHalf(value / 255) * 255 : Math.round(value);
};
