(async () => {
  const render = async (setup) => {
    const context = new OfflineAudioContext(1, 16, 8000),
      source = new ConstantSourceNode(context, { offset: 1 }),
      gain = new GainNode(context);
    setup(gain.gain);
    source.connect(gain).connect(context.destination);
    source.start();
    return Array.from(
      (await context.startRendering()).getChannelData(0),
      (value) => Math.round(value * 1000000) / 1000000,
    );
  };
  const result = {};
  result.target = await render((param) => {
    param.value = 0;
    param.setTargetAtTime(1, 0, 4 / 8000);
  });
  result.targetZero = await render((param) => {
    param.value = 0;
    param.setTargetAtTime(1, 0, 0);
  });
  result.cancel = await render((param) => {
    param.setValueAtTime(0, 0);
    param.linearRampToValueAtTime(1, 8 / 8000);
    param.cancelAndHoldAtTime(4 / 8000);
  });
  result.sameTime = await render((param) => {
    param.setValueAtTime(0.25, 0);
    param.setValueAtTime(0.75, 0);
    param.linearRampToValueAtTime(1, 8 / 8000);
  });
  const context = new OfflineAudioContext(1, 8, 8000),
    param = context.createGain().gain;
  result.exceptions = {};
  for (const [name, call] of Object.entries({
    targetNegative: () => param.setTargetAtTime(1, 0, -1),
    exponentialZero: () => param.exponentialRampToValueAtTime(0, 1),
    curveOverlap: () => {
      param.setValueCurveAtTime([0, 1], 0, 1);
      param.setValueAtTime(1, 0.5);
    },
  }))
    try {
      call();
      result.exceptions[name] = '';
    } catch (error) {
      result.exceptions[name] = error.name;
    }
  return result;
})();
