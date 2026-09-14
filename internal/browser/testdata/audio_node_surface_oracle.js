(() => {
  const context = new OfflineAudioContext(2, 128, 48000);
  const names = [
    'DelayNode',
    'ChannelSplitterNode',
    'ChannelMergerNode',
    'StereoPannerNode',
    'PannerNode',
    'ConvolverNode',
    'WaveShaperNode',
    'BiquadFilterNode',
    'IIRFilterNode',
    'ConstantSourceNode',
    'AnalyserNode',
  ];
  const result = {};
  const summarize = (value) => {
    const own = {};
    for (const key of Object.getOwnPropertyNames(Object.getPrototypeOf(value))) {
      if (key === 'constructor') continue;
      const descriptor = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(value), key);
      try {
        const observed = value[key];
        own[key] = [
          typeof observed === 'object'
            ? Object.prototype.toString.call(observed)
            : typeof observed === 'function'
              ? 'function'
              : observed,
          descriptor.enumerable,
          descriptor.configurable,
          !!descriptor.get,
          !!descriptor.set,
          !!descriptor.value,
        ];
      } catch (error) {
        own[key] = [
          error.name,
          descriptor.enumerable,
          descriptor.configurable,
          !!descriptor.get,
          !!descriptor.set,
          !!descriptor.value,
        ];
      }
    }
    return {
      tag: Object.prototype.toString.call(value),
      inputs: value.numberOfInputs,
      outputs: value.numberOfOutputs,
      channelCount: value.channelCount,
      channelCountMode: value.channelCountMode,
      channelInterpretation: value.channelInterpretation,
      own,
    };
  };
  for (const name of names) {
    try {
      let node;
      if (name === 'IIRFilterNode')
        node = new IIRFilterNode(context, { feedforward: [1], feedback: [1] });
      else node = new globalThis[name](context);
      result[name] = summarize(node);
    } catch (error) {
      result[name] = error.name + ':' + error.message;
    }
  }
  result.listener = {
    same: context.listener === context.listener,
    tag: Object.prototype.toString.call(context.listener),
    values: {
      positionX: context.listener.positionX.value,
      positionY: context.listener.positionY.value,
      positionZ: context.listener.positionZ.value,
      forwardX: context.listener.forwardX.value,
      forwardY: context.listener.forwardY.value,
      forwardZ: context.listener.forwardZ.value,
      upX: context.listener.upX.value,
      upY: context.listener.upY.value,
      upZ: context.listener.upZ.value,
    },
  };
  return result;
})();
