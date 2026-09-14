(() => {
  const out = {};
  out.initial = typeof document.hasFocus();
  out.detached = document.implementation.createHTMLDocument('').hasFocus();
  const input = document.createElement('input');
  document.body.append(input);
  input.focus();
  out.focused = [document.activeElement === input, document.hasFocus()];
  input.blur();
  out.blurred = [document.activeElement === document.body, document.hasFocus()];
  input.remove();
  try {
    Document.prototype.hasFocus.call({});
    out.brand = 'no-error';
  } catch (e) {
    out.brand = e.name;
  }
  return out;
})();
