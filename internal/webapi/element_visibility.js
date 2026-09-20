// Observe the existing box/style model; zero-area boxes still count as boxes.
function observeElementVisibility(element, options = {}) {
  return checkElementVisibility.call(element, options);
}
function checkElementVisibility(options = {}) {
  if (!elementSlot(this)) throw new TypeError('Illegal invocation');
  if (!this.isConnected) return false;
  options = options ?? {};
  return withStyleReadCache(() => {
    if (blitzSkippedContent(this)) return false;
    const flags = {
      visibilityProperty: !!(options.visibilityProperty || options.checkVisibilityCSS),
      opacityProperty: !!(options.opacityProperty || options.checkOpacity),
      contentVisibilityAuto: !!options.contentVisibilityAuto,
    };
    if (!flags.visibilityProperty && !flags.opacityProperty && !flags.contentVisibilityAuto) {
      const batched = cssBatchedForeignVisibility(this);
      if (batched !== undefined) return batched;
    }
    const foreign = foreignCSSObservation(this, 'visibility', JSON.stringify(flags));
    if (foreign !== null) return foreign;
    if (!cssBoxModel.hasBox(this) || cssBoxModel.state(this).display === 'contents') return false;
    if (
      (options.visibilityProperty || options.checkVisibilityCSS) &&
      ['hidden', 'collapse'].includes(getComputedStyle(this).visibility)
    )
      return false;
    for (let element = this; element; element = geometryParent(element)) {
      const state = cssBoxModel.state(element);
      if (element !== this && state.get('content-visibility') === 'hidden') return false;
      if (
        (options.opacityProperty || options.checkOpacity) &&
        Number(state.get('opacity') ?? 1) === 0
      )
        return false;
    }
    return true;
  });
}
Object.defineProperty(Element.prototype, 'checkVisibility', {
  value: checkElementVisibility,
  writable: true,
  enumerable: true,
  configurable: true,
});
