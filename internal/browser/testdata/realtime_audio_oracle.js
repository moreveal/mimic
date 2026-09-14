(async () => {
  const c = new AudioContext({ sampleRate: 48000 }),
    out = {};
  out.brand = [
    c instanceof AudioContext,
    c instanceof BaseAudioContext,
    c instanceof EventTarget,
    c.destination.context === c,
    c.destination === c.destination,
    c.sampleRate,
  ];
  await c.suspend();
  const t = c.currentTime;
  await new Promise((r) => setTimeout(r, 25));
  out.suspended = [c.state, c.currentTime === t];
  const source = c.createBufferSource();
  source.buffer = c.createBuffer(1, 480, 48000);
  source.buffer.getChannelData(0).fill(0.25);
  const gain = c.createGain();
  gain.gain.value = 0.5;
  source.connect(gain).connect(c.destination);
  let ended = 0,
    trusted = false;
  source.onended = (e) => {
    ended++;
    trusted = e.isTrusted;
  };
  source.start(c.currentTime);
  await c.resume();
  await new Promise((r) => setTimeout(r, 60));
  out.running = [c.state, c.currentTime > t, ended, trusted];
  await c.suspend();
  const stopped = c.currentTime;
  await new Promise((r) => setTimeout(r, 25));
  out.stable = c.currentTime === stopped;
  await c.close();
  out.closed = c.state;
  out.errors = [];
  for (const name of ['resume', 'suspend', 'close'])
    out.errors.push(
      await c[name]().then(
        () => null,
        (e) => e.name,
      ),
    );
  const a = new AudioContext();
  out.independent = a !== c && a.destination !== c.destination;
  await a.close();
  return out;
})();
