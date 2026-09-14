(async () => {
  document.body.innerHTML = `<input id="parent">`;
  const frame = document.createElement('iframe');
  frame.id = 'frame';
  frame.srcdoc = '<input id=child>';
  await new Promise((resolve) => {
    frame.onload = resolve;
    document.body.append(frame);
  });
  const pEl = document.getElementById('parent');
  const child = frame.contentDocument.getElementById('child'),
    log = [];
  const watch = (target, label) =>
    ['focus', 'blur', 'focusin', 'focusout'].forEach((type) =>
      target.addEventListener(
        type,
        (event) =>
          log.push([
            label,
            type,
            event.bubbles,
            event.composed,
            event.relatedTarget?.id || event.relatedTarget?.tagName || null,
          ]),
        true,
      ),
    );
  watch(pEl, 'parent');
  watch(frame, 'frame');
  watch(child, 'child');
  pEl.focus();
  const parentState = [
    document.activeElement.id,
    document.hasFocus(),
    frame.contentDocument.hasFocus(),
  ];
  child.focus();
  const childState = [
    document.activeElement.id,
    frame.contentDocument.activeElement.id,
    document.hasFocus(),
    frame.contentDocument.hasFocus(),
  ];
  child.blur();
  const blurState = [
    document.activeElement.id || document.activeElement.tagName,
    frame.contentDocument.activeElement.id || frame.contentDocument.activeElement.tagName,
  ];
  return { parentState, childState, blurState, log };
})();
