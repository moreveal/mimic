(() => {
  const out = {};
  for (const [tag, property] of [
    ['div', 'inputMode'],
    ['div', 'enterKeyHint'],
    ['input', 'autocomplete'],
    ['textarea', 'autocomplete'],
    ['select', 'autocomplete'],
    ['form', 'autocomplete'],
    ['audio', 'preload'],
  ]) {
    const e = document.createElement(tag);
    const values = [
      null,
      '',
      'none',
      'text',
      'TEL',
      ' email ',
      'numeric',
      'decimal',
      'search',
      'enter',
      'done',
      'next',
      'go',
      'previous',
      'send',
      'on',
      'off',
      'EMAIL',
      'section-A shipping home tel',
      'shipping email',
      'home email',
      'username webauthn',
      'webauthn username',
      'foo',
      'metadata',
      'auto',
      'NONE',
    ];
    out[tag + '.' + property] = values.map((value) => {
      if (value === null) e.removeAttribute(property.toLowerCase());
      else e.setAttribute(property.toLowerCase(), value);
      return [value, e[property]];
    });
    e[property] = 'EMAIL';
    out[tag + '.' + property + '.set'] = [e[property], e.getAttribute(property.toLowerCase())];
  }
  return out;
})();
