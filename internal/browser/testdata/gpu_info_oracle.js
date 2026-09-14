(async () => {
  const adapter = await navigator.gpu.requestAdapter();
  const info = adapter.info;
  const result = {
    same: info === adapter.info,
    subgroupMinSize: info.subgroupMinSize,
    subgroupMaxSize: info.subgroupMaxSize,
    isFallbackAdapter: info.isFallbackAdapter,
  };
  result.receivers = {};
  for (const name of ['subgroupMinSize', 'subgroupMaxSize', 'isFallbackAdapter']) {
    const getter = Object.getOwnPropertyDescriptor(GPUAdapterInfo.prototype, name).get;
    try {
      getter.call({});
      result.receivers[name] = 'accepted';
    } catch (error) {
      result.receivers[name] = error.name;
    }
  }
  return result;
})();
