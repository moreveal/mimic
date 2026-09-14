(async () => {
  const out = {},
    u = navigator.userAgentData;
  out.identity = [
    u === navigator.userAgentData,
    u.brands === u.brands,
    Object.isFrozen(u.brands),
    Object.isFrozen(u.brands[0]),
  ];
  out.arguments = [];
  for (const hints of [null, undefined, 'model'])
    out.arguments.push(
      await u.getHighEntropyValues(hints).then(
        () => null,
        (e) => e.name,
      ),
    );
  const local = await u.getHighEntropyValues([
    'architecture',
    'bitness',
    'fullVersionList',
    'formFactors',
    'wow64',
  ]);
  const source = `(async()=>{const u=navigator.userAgentData;postMessage({ua:await u.getHighEntropyValues(['architecture','bitness','fullVersionList','formFactors','wow64']),same:u===navigator.userAgentData,frozen:Object.isFrozen(u.brands),storageSame:navigator.storage===navigator.storage,persisted:await navigator.storage.persisted(),estimate:await navigator.storage.estimate(),persist:typeof navigator.storage.persist})})()`;
  const remote = await new Promise((resolve, reject) => {
    const url = URL.createObjectURL(new Blob([source], { type: 'text/javascript' })),
      w = new Worker(url);
    w.onmessage = (e) => {
      w.terminate();
      URL.revokeObjectURL(url);
      resolve(e.data);
    };
    w.onerror = (e) => {
      w.terminate();
      reject(e.message);
    };
  });
  out.worker = [
    JSON.stringify(local) === JSON.stringify(remote.ua),
    remote.same,
    remote.frozen,
    remote.storageSame,
    remote.persisted,
    remote.persist,
  ];
  const estimate = await navigator.storage.estimate();
  out.quota = [estimate.quota === remote.estimate.quota, estimate.usage === remote.estimate.usage];
  return out;
})();
