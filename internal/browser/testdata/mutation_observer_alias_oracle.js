new Promise(resolve => {
  const same = WebKitMutationObserver === MutationObserver;
  const records = [];
  const target = document.createElement('div');
  const legacy = new WebKitMutationObserver(items => records.push('legacy:' + items[0].attributeName));
  const canonical = new MutationObserver(items => records.push('canonical:' + items[0].attributeName));
  legacy.observe(target, {attributes:true});
  canonical.observe(target, {attributes:true});
  target.setAttribute('data-value', '1');
  queueMicrotask(() => {
    legacy.disconnect();
    canonical.disconnect();
    resolve(JSON.stringify({same, brand:legacy instanceof MutationObserver, records}));
  });
})
