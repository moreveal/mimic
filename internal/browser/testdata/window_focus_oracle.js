(async () => {
  const events = [];
  for (const type of ['focus', 'blur'])
    window.addEventListener(type, (event) => events.push([type, event.isTrusted]));
  const initial = document.hasFocus();
  window.blur();
  await new Promise((resolve) => setTimeout(resolve));
  const blurred = document.hasFocus();
  window.focus();
  await new Promise((resolve) => setTimeout(resolve));
  return { initial, blurred, focused: document.hasFocus(), events };
})();
