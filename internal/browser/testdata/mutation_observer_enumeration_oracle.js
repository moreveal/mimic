new Promise(resolve => {
  const observer = new MutationObserver(records => {
    wrapped.disconnect();
    resolve(JSON.stringify({methods, flags, attribute:records[0].attributeName}));
  });
  // Constructor instrumentation discovers inherited Web IDL operations with
  // for-in, then forwards each method to its original branded instance.
  const wrapped = {};
  const methods = [];
  for (const name in observer) {
    if (typeof observer[name] === 'function') {
      methods.push(name);
      wrapped[name] = (...args) => Reflect.apply(observer[name], observer, args);
    }
  }
  methods.sort();
  const flags = ['disconnect', 'observe', 'takeRecords'].map(name => {
    const d = Object.getOwnPropertyDescriptor(MutationObserver.prototype, name);
    return [name, d.enumerable, d.configurable, d.writable];
  });
  const target = document.createElement('div');
  wrapped.observe(target, {attributes:true});
  target.setAttribute('data-value', '1');
})
