(() => {
  const result = {};
  for (const name of [
    'GamepadButton',
    'MutationObserver',
    'DataTransferItem',
    'PluginArray',
    'PaymentResponse',
    'AudioNode',
    'USB',
    'HTMLInputElement',
  ]) {
    result[name] = Object.getOwnPropertyNames(globalThis[name].prototype);
  }
  return result;
})()
