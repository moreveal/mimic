(async () => {
  const out = {},
    c = new OfflineAudioContext(1, 256, 8000),
    a = c.createAnalyser();
  out.defaults = [
    a.fftSize,
    a.frequencyBinCount,
    a.minDecibels,
    a.maxDecibels,
    a.smoothingTimeConstant,
    a.context === c,
  ];
  const f = new Float32Array(2050).fill(7),
    b = new Uint8Array(2050).fill(7);
  a.getFloatTimeDomainData(f);
  a.getByteTimeDomainData(b);
  out.silence = [
    f.slice(0, 2048).every((v) => v === 0),
    b.slice(0, 2048).every((v) => v === 128),
    f[2048],
    b[2048],
  ];
  const freq = new Float32Array(1026).fill(7);
  a.getFloatFrequencyData(freq);
  out.silentFrequency = [freq.slice(0, 1024).every((v) => v === -Infinity), freq[1024]];
  a.fftSize = 32;
  const s = c.createConstantSource();
  s.offset.value = 0.25;
  s.connect(a).connect(c.destination);
  s.start();
  const buffer = await c.startRendering();
  const t = new Float32Array(32),
    bytes = new Uint8Array(32);
  a.getFloatTimeDomainData(t);
  a.getByteTimeDomainData(bytes);
  out.constant = [
    Array.from(t),
    Array.from(bytes),
    buffer.getChannelData(0).every((v) => v === 0.25),
  ];
  out.invalid = [];
  for (const size of [0, 16, 33, 65536]) {
    try {
      a.fftSize = size;
      out.invalid.push(null);
    } catch (e) {
      out.invalid.push(e.name);
    }
  }
  out.finalFFT = a.fftSize;
  return out;
})();
