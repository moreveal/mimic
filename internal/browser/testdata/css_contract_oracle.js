(() => {
  const conditions = [
    'not (display: potato) and (color: red)',
    '(display: grid) or (display: potato) and (color: red)',
    '((display: grid) and ((color: red)))',
    'not ((display: potato) or (color: invalid))',
    '(unknown-function(foo))',
    'not (unknown-function(foo))',
    '(display: grid) and ((color: red) or (color: invalid))',
  ];
  const declarations = [
    ['position', 'absolute'],
    ['position', 'potato'],
    ['width', 'calc(10px + 2%)'],
    ['width', 'red'],
    ['overflow', 'hidden auto'],
    ['overflow', 'hidden potato'],
    ['z-index', '12'],
    ['z-index', '1.5'],
    ['box-sizing', 'border-box'],
    ['visibility', 'collapse'],
    ['white-space', 'break-spaces'],
    ['object-fit', 'scale-down'],
    ['cursor', 'pointer'],
    ['cursor', 'potato'],
  ];
  const custom = ['', 'hello', '{}', '[]', '"unterminated', 'a;b', 'a!important', 'var(--y)'];
  return {
    conditions: Object.fromEntries(conditions.map((value) => [value, CSS.supports(value)])),
    declarations: Object.fromEntries(
      declarations.map(([name, value]) => [name + ':' + value, CSS.supports(name, value)]),
    ),
    custom: Object.fromEntries(custom.map((value) => [value, CSS.supports('--probe', value)])),
  };
})();
