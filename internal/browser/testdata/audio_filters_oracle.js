(() => {
  const c = new OfflineAudioContext(1, 128, 48000),
    out = {};
  for (const type of [
    'lowpass',
    'highpass',
    'bandpass',
    'lowshelf',
    'highshelf',
    'peaking',
    'notch',
    'allpass',
  ]) {
    const n = new BiquadFilterNode(c, { type, frequency: 1000, Q: 2, gain: 6 });
    const f = new Float32Array([0, 500, 1000, 2000, 10000, 24000, -1, 24001]),
      m = new Float32Array(f.length),
      p = new Float32Array(f.length);
    n.getFrequencyResponse(f, m, p);
    out[type] = {
      m: Array.from(m, (v) => (Number.isFinite(v) ? Math.round(v * 100000) / 100000 : String(v))),
      p: Array.from(p, (v) => (Number.isFinite(v) ? Math.round(v * 100000) / 100000 : String(v))),
    };
  }
  const i = new IIRFilterNode(c, { feedforward: [0.5, 0.5], feedback: [1] });
  const f = new Float32Array([0, 12000, 24000]),
    m = new Float32Array(3),
    p = new Float32Array(3);
  i.getFrequencyResponse(f, m, p);
  out.iir = Array.from(m, (v) => Math.round(v * 100000) / 100000);
  return out;
})();
