(() => {
  const context = new AudioContext();
  const describe = (value, keys) => ({
    tag: Object.prototype.toString.call(value),
    inputs: value.numberOfInputs,
    outputs: value.numberOfOutputs,
    values: Object.fromEntries(
      keys.map((key) => [key, value[key] === value[key] ? typeof value[key] : 'nan']),
    ),
  });
  const destination = context.createMediaStreamDestination();
  const streamSource = context.createMediaStreamSource(destination.stream);
  const track = destination.stream.getAudioTracks()[0];
  const element = document.createElement('audio');
  const elementSource = context.createMediaElementSource(element);
  let duplicate;
  try {
    context.createMediaElementSource(element);
    duplicate = '';
  } catch (error) {
    duplicate = error.name;
  }
  const processor = context.createScriptProcessor(256, 1, 2);
  const result = {
    destination: describe(destination, ['stream']),
    stream: [destination.stream === destination.stream, track.kind, track.readyState],
    streamSource: describe(streamSource, ['mediaStream']),
    elementSource: describe(elementSource, ['mediaElement']),
    identities: [
      streamSource.mediaStream === destination.stream,
      elementSource.mediaElement === element,
    ],
    duplicate,
    processor: [
      processor.numberOfInputs,
      processor.numberOfOutputs,
      processor.bufferSize,
      processor.onaudioprocess,
    ],
  };
  context.close();
  return result;
})();
