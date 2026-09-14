(async () => {
  const out = {};
  for (const type of [
    'lowpass',
    'highpass',
    'bandpass',
    'lowshelf',
    'highshelf',
    'peaking',
    'notch',
    'allpass',
    'iir',
  ]) {
    const context = new OfflineAudioContext(2, 256, 48000);
    const buffer = context.createBuffer(2, 256, 48000);
    buffer.getChannelData(0)[0] = 1;
    buffer.getChannelData(1)[17] = -0.5;
    const source = new AudioBufferSourceNode(context, { buffer });
    const filter =
      type === 'iir'
        ? new IIRFilterNode(context, { feedforward: [0.2, 0.3], feedback: [1, -0.5] })
        : new BiquadFilterNode(context, { type, frequency: 1200, Q: 2, gain: 6 });
    source.connect(filter).connect(context.destination);
    source.start();
    const rendered = await context.startRendering();
    out[type] = [0, 1].map((channel) =>
      Array.from(
        rendered.getChannelData(channel),
        (value) => Math.round(value * 1000000) / 1000000,
      ),
    );
  }
  const context = new OfflineAudioContext(1, 256, 48000);
  const source = new ConstantSourceNode(context, { offset: 0.25 });
  const events = [];
  source.onended = (event) => events.push([event.type, event.isTrusted]);
  source.connect(context.destination);
  source.start(16 / 48000);
  source.stop(48 / 48000);
  const result = await context.startRendering();
  out.constant = { samples: Array.from(result.getChannelData(0)), events };
  return out;
})();
