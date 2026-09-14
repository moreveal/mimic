(async () => {
  const context = new AudioContext();
  const result = {
    workletSame: context.audioWorklet === context.audioWorklet,
    workletTag: Object.prototype.toString.call(context.audioWorklet),
  };
  const source = `class ProbeProcessor extends AudioWorkletProcessor { static get parameterDescriptors() { return [{name:'gain',defaultValue:.5,minValue:0,maxValue:2,automationRate:'a-rate'}] } process(){ return true } } registerProcessor('probe-processor', ProbeProcessor)`;
  const url = URL.createObjectURL(new Blob([source], { type: 'text/javascript' }));
  try {
    result.add = await context.audioWorklet.addModule(url).then(
      (value) => [String(value), typeof value],
      (error) => [error.name, error.message],
    );
    const node = new AudioWorkletNode(context, 'probe-processor', {
      numberOfInputs: 2,
      numberOfOutputs: 2,
      outputChannelCount: [1, 2],
      parameterData: { gain: 0.75 },
    });
    result.node = {
      tag: Object.prototype.toString.call(node),
      inputs: node.numberOfInputs,
      outputs: node.numberOfOutputs,
      portSame: node.port === node.port,
      parametersSame: node.parameters === node.parameters,
      parameterSize: node.parameters.size,
      gain: node.parameters.get('gain').value,
      handler: node.onprocessorerror,
    };
    result.missing = (() => {
      try {
        new AudioWorkletNode(context, 'missing');
        return '';
      } catch (error) {
        return error.name;
      }
    })();
  } finally {
    URL.revokeObjectURL(url);
    await context.close();
  }
  return result;
})();
