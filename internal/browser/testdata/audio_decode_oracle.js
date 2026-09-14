(async () => {
  const wav = (samples, rate = 8000) => {
    const buffer = new ArrayBuffer(44 + samples.length * 2),
      view = new DataView(buffer);
    const text = (offset, value) =>
      [...value].forEach((char, index) => view.setUint8(offset + index, char.charCodeAt(0)));
    text(0, 'RIFF');
    view.setUint32(4, 36 + samples.length * 2, true);
    text(8, 'WAVE');
    text(12, 'fmt ');
    view.setUint32(16, 16, true);
    view.setUint16(20, 1, true);
    view.setUint16(22, 1, true);
    view.setUint32(24, rate, true);
    view.setUint32(28, rate * 2, true);
    view.setUint16(32, 2, true);
    view.setUint16(34, 16, true);
    text(36, 'data');
    view.setUint32(40, samples.length * 2, true);
    samples.forEach((value, index) => view.setInt16(44 + index * 2, value, true));
    return buffer;
  };
  const context = new OfflineAudioContext(1, 32, 8000),
    log = [],
    input = wav([0, 32767, -32768, 16384]);
  const promise = context.decodeAudioData(
    input,
    (buffer) => log.push(['success', buffer.length]),
    (error) => log.push(['error', error.name]),
  );
  log.push(['sync', input.byteLength]);
  const buffer = await promise;
  log.push(['promise', buffer.length]);
  const invalid = await context.decodeAudioData(new ArrayBuffer(8)).then(
    () => '',
    (error) => error.name,
  );
  const resampled = await new OfflineAudioContext(1, 32, 16000).decodeAudioData(
    wav([0, 32767, -32768, 16384]),
  );
  return {
    log,
    invalid,
    buffer: {
      channels: buffer.numberOfChannels,
      rate: buffer.sampleRate,
      length: buffer.length,
      duration: buffer.duration,
      samples: Array.from(buffer.getChannelData(0), (value) => Math.round(value * 100000) / 100000),
    },
    resampled: {
      rate: resampled.sampleRate,
      length: resampled.length,
      finite: Array.from(resampled.getChannelData(0)).every(Number.isFinite),
    },
  };
})();
