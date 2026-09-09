self.onmessage = async event => {
  const out = [];
  for (const stage of ['before-read', 'during-read']) {
    for (const reasonKind of ['default', 'object', 'string']) {
      const controller = new AbortController(), reason = reasonKind === 'object' ? {token: 1} : 'custom';
      const response = await fetch(event.data.base + '/slow-body?stage=' + stage + '&reason=' + reasonKind, {signal: controller.signal});
      const abort = () => reasonKind === 'default' ? controller.abort() : controller.abort(reason);
      let pending;
      if (stage === 'before-read') { abort(); pending = response.text(); }
      else { pending = response.text(); setTimeout(abort, 10); }
      try { out.push({stage, reasonKind, body: await pending}); }
      catch (error) { out.push({stage, reasonKind, error: {name: error && error.name, message: error && error.message, code: error && error.code}, sameReason: error === controller.signal.reason, sameCustom: error === reason, value: typeof error === 'string' ? error : undefined}); }
    }
  }
  postMessage(out);
};
