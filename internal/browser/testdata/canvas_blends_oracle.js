(() => {
  const out = {},
    modes =
      'source-over copy source-in source-out source-atop destination-over destination-in destination-out destination-atop xor lighter multiply screen overlay darken lighten color-dodge color-burn hard-light soft-light difference exclusion hue saturation color luminosity'.split(
        ' ',
      );
  for (const mode of modes) {
    const c = document.createElement('canvas');
    c.width = 2;
    c.height = 1;
    const x = c.getContext('2d');
    x.fillStyle = 'rgb(51,102,153)';
    x.fillRect(0, 0, 2, 1);
    x.globalCompositeOperation = mode;
    x.fillStyle = 'rgb(204,76,25)';
    x.fillRect(0, 0, 1, 1);
    out[mode] = Array.from(x.getImageData(0, 0, 2, 1).data);
  }
  return out;
})();
