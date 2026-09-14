(async () => {
  const result = {};
  for (const sampleRate of [8000, 22050, 44100, 48000, 96000]) {
    for (const latencyHint of ['interactive', 'balanced', 'playback', 0, 0.001, 0.015, 0.1]) {
      const context = new AudioContext({ sampleRate, latencyHint });
      result[sampleRate + '|' + latencyHint] = [context.baseLatency, context.outputLatency];
      await context.close();
    }
  }
  return result;
})();
