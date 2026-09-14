(async () => {
  const result = {};
  const first = await navigator.gpu.requestAdapter(),
    second = await navigator.gpu.requestAdapter();
  result.adapterIdentity = [
    first === second,
    first.info === first.info,
    first.features === first.features,
    first.limits === first.limits,
  ];
  const fallback = await navigator.gpu.requestAdapter({ forceFallbackAdapter: true });
  result.fallback = fallback === null ? null : fallback.info.isFallbackAdapter;
  result.invalidPreference = await navigator.gpu
    .requestAdapter({ powerPreference: 'invalid' })
    .then(
      () => '',
      (error) => error.name,
    );
  const device = await first.requestDevice();
  result.deviceIdentity = [
    device.queue === device.queue,
    device.features === device.features,
    device.limits === device.limits,
    device.adapterInfo === device.adapterInfo,
    device.adapterInfo === first.info,
    device.lost === device.lost,
  ];
  result.secondDevice = await first.requestDevice().then(
    () => '',
    (error) => error.name,
  );
  result.receiver = {};
  for (const [type, property] of [
    [GPUAdapter, 'info'],
    [GPUDevice, 'queue'],
    [GPUSupportedLimits, 'maxTextureDimension2D'],
  ]) {
    try {
      Object.getOwnPropertyDescriptor(type.prototype, property).get.call({});
      result.receiver[property] = '';
    } catch (error) {
      result.receiver[property] = error.name;
    }
  }
  device.destroy();
  result.lost = await device.lost.then((info) => [info.reason, typeof info.message]);
  return result;
})();
