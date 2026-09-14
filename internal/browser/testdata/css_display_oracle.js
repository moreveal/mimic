(() => {
  const result = {};
  for (const value of [
    'grid',
    'inline grid',
    'grid inline',
    'block flow',
    'inline flow-root',
    'block ruby',
    'inline ruby',
    'flow-root list-item',
    'inline flow-root list-item',
    'run-in',
    'math',
    'inline math',
    'block math',
    'ruby-base',
    'ruby-text',
    'flex list-item',
    'block inline',
    'grid grid',
    'TABLE',
    'var(--display)',
    'invented',
  ]) {
    const element = document.createElement('div');
    element.style.display = value;
    result[value] = [CSS.supports('display', value), element.style.display];
  }
  return result;
})();
