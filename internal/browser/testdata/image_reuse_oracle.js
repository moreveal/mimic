async (control, loading) => {
  const pause = () => new Promise(resolve => setTimeout(resolve, 120));
  const read = image => [image.complete, image.naturalWidth, image.currentSrc.endsWith(control + loading)];
  const image = new Image();
  image.loading = loading;
  image.src = '/image?' + control + loading;
  window.images = [image];
  await pause();
  const detached = read(image);
  document.body.append(image);
  await image.decode();
  const inserted = read(image);
  const clone = image.cloneNode();
  window.images.push(clone);
  const events = [];
  clone.onload = () => events.push('clone');
  await pause();
  const detachedClone = read(clone);
  document.body.append(clone);
  await pause();
  const insertedClone = read(clone);
  for (let i = 0; i < 4; i++) {
    window.images.push(image.cloneNode());
    await pause();
  }
  const other = new Image();
  window.images.push(other);
  other.src = image.src;
  await other.decode();
  return {stages: [detached, inserted, detachedClone, insertedClone, read(other)], events};
}
