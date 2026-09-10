globalThis.runImagePreloadCase = async function (options) {
  const destination = options.destination || 'image';
  const url = '/ci?case=' + options.name + '&completed=' + !!options.completed + '&as=' + destination + '&fail=' + !!options.fail + '&invalid=' + !!options.invalid;
  const events = [];
  const link = document.createElement('link');
  link.rel = 'preload';
  link.as = options.preloadAs || destination;
  if (options.linkCrossOrigin !== undefined) link.crossOrigin = options.linkCrossOrigin;
  link.href = url;
  const preloaded = new Promise(resolve => {
    link.onload = () => { events.push('link:load'); resolve(); };
    link.onerror = () => { events.push('link:error'); resolve(); };
  });
  document.head.appendChild(link);
  if (options.completed) await preloaded;
  else await fetch('/started?case=' + options.name);
  const image = destination === 'image' ? new Image() : document.createElement(destination === 'style' ? 'link' : 'script');
  if (options.imageCrossOrigin !== undefined) image.crossOrigin = options.imageCrossOrigin;
  const loaded = new Promise(resolve => {
    image.onload = () => { events.push('image:load'); resolve(); };
    image.onerror = () => { events.push('image:error'); resolve(); };
  });
  if (destination === 'fetch') {
    fetch(url).then(response => response.text()).then(() => image.onload(), () => image.onerror());
  } else if (destination === 'xhr') {
    const xhr = new XMLHttpRequest();
    xhr.open('GET', url); xhr.onload = () => image.onload(); xhr.onerror = () => image.onerror(); xhr.send();
  } else if (destination === 'style') {
    image.rel = 'stylesheet'; image.href = url; document.head.appendChild(image);
  } else {
    image.src = url;
    if (destination === 'script') document.head.appendChild(image);
  }
  if (!options.completed) await fetch('/release?case=' + options.name);
  await Promise.all([preloaded, loaded]);
  return {events: events.sort(), complete: destination === 'image' ? image.complete : true, width: destination === 'image' ? image.naturalWidth : 0,
    timing: performance.getEntriesByName(new URL(url, location.href).href).map(e => e.initiatorType)};
};
