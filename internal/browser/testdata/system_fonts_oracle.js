(() => {
  const out = {};
  for (const keyword of ['caption', 'icon', 'menu', 'message-box', 'small-caption', 'status-bar']) {
    const e = document.createElement('div');
    e.style.font = keyword;
    document.body.append(e);
    const c = getComputedStyle(e);
    out[keyword] = {
      specified: e.style.font,
      style: c.fontStyle,
      weight: c.fontWeight,
      size: c.fontSize,
      family: c.fontFamily,
      lineHeight: c.lineHeight,
    };
    e.remove();
  }
  return out;
})();
