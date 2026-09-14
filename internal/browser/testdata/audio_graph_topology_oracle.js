(async () => {
  const samples = (buffer) =>
    Array.from({ length: buffer.numberOfChannels }, (_, channel) =>
      Array.from(buffer.getChannelData(channel), (value) => Math.round(value * 100000) / 100000),
    );
  const result = {};

  {
    const context = new OfflineAudioContext(2, 16, 8000);
    const a = new ConstantSourceNode(context, { offset: 0.25 });
    const b = new ConstantSourceNode(context, { offset: 0.5 });
    const gain = new GainNode(context, { gain: 2 });
    a.connect(gain);
    b.connect(gain);
    gain.connect(context.destination);
    gain.connect(context.destination);
    a.start();
    b.start();
    result.fan = samples(await context.startRendering());
  }
  {
    const context = new OfflineAudioContext(2, 16, 8000);
    const buffer = context.createBuffer(2, 16, 8000);
    buffer.getChannelData(0).fill(0.25);
    buffer.getChannelData(1).fill(0.75);
    const source = new AudioBufferSourceNode(context, { buffer });
    const split = new ChannelSplitterNode(context, { numberOfOutputs: 2 });
    const merge = new ChannelMergerNode(context, { numberOfInputs: 2 });
    source.connect(split);
    split.connect(merge, 1, 0);
    split.connect(merge, 0, 1);
    merge.connect(context.destination);
    source.start();
    result.ports = samples(await context.startRendering());
  }
  {
    const context = new OfflineAudioContext(1, 16, 8000);
    const carrier = new ConstantSourceNode(context, { offset: 1 });
    const control = new ConstantSourceNode(context, { offset: 0.25 });
    const gain = new GainNode(context, { gain: 0.5 });
    carrier.connect(gain);
    control.connect(gain.gain);
    gain.connect(context.destination);
    carrier.start();
    control.start();
    result.param = samples(await context.startRendering());
  }
  {
    const context = new OfflineAudioContext(1, 384, 8000);
    const impulse = context.createBuffer(1, 384, 8000);
    impulse.getChannelData(0)[0] = 1;
    const source = new AudioBufferSourceNode(context, { buffer: impulse });
    const gain = new GainNode(context, { gain: 0.5 });
    const delay = new DelayNode(context, { maxDelayTime: 1, delayTime: 128 / 8000 });
    source.connect(gain);
    gain.connect(context.destination);
    gain.connect(delay);
    delay.connect(gain);
    source.start();
    const rendered = await context.startRendering();
    result.feedback = [0, 127, 128, 255, 256, 383].map(
      (i) => Math.round(rendered.getChannelData(0)[i] * 100000) / 100000,
    );
  }
  {
    const context = new OfflineAudioContext(1, 8, 8000);
    const buffer = context.createBuffer(1, 8, 8000);
    buffer.copyToChannel(new Float32Array([-1, -0.5, 0, 0.5, 1, 2, -2, 0.25]), 0);
    const source = new AudioBufferSourceNode(context, { buffer });
    const shaper = new WaveShaperNode(context, { curve: [-1, -0.25, 1] });
    source.connect(shaper).connect(context.destination);
    source.start();
    result.wave = samples(await context.startRendering());
  }
  {
    const context = new OfflineAudioContext(1, 8, 8000);
    const input = context.createBuffer(1, 8, 8000);
    input.getChannelData(0)[0] = 1;
    const impulse = context.createBuffer(1, 3, 8000);
    impulse.copyToChannel(new Float32Array([1, 0.5, 0.25]), 0);
    const source = new AudioBufferSourceNode(context, { buffer: input });
    const convolver = new ConvolverNode(context, { buffer: impulse, disableNormalization: true });
    source.connect(convolver).connect(context.destination);
    source.start();
    result.convolution = samples(await context.startRendering());
  }
  return result;
})();
