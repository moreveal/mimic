(() => {
  const names =
    'activetext buttonborder buttonface buttontext canvas canvastext field fieldtext graytext highlight highlighttext linktext mark marktext selecteditem selecteditemtext visitedtext accentcolor accentcolortext activeborder activecaption appworkspace background buttonhighlight buttonshadow captiontext inactiveborder inactivecaption inactivecaptiontext infobackground infotext menu menutext scrollbar threeddarkshadow threedface threedhighlight threedlightshadow threedshadow window windowframe windowtext'.split(
      ' ',
    );
  const result = {};
  for (const scheme of ['light', 'dark']) {
    document.documentElement.style.colorScheme = scheme;
    result[scheme] = {};
    for (const name of names) {
      const element = document.createElement('div');
      element.style.color = name;
      document.body.append(element);
      result[scheme][name] = [element.style.color, getComputedStyle(element).color];
      element.remove();
    }
  }
  return result;
})();
