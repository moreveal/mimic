self.onmessage = async event => {
  const {base, label, redirects} = event.data;
  const out = {label, location: {href: location.href, origin: location.origin}, types: {}};
  for (const key of ['fetch', 'Response', 'Headers', 'Request', 'AbortController', 'AbortSignal']) out.types[key] = typeof self[key];
  const describeError = error => ({name: error && error.name, message: error && error.message, code: error && error.code});
  const request = async (input, init) => {
    let promise;
    try { promise = fetch(input, init); } catch (error) { return {threw: true, error: describeError(error)}; }
    try {
      const response = await promise;
      const result = {promise: promise instanceof Promise, response: response instanceof Response, headers: response.headers instanceof Headers,
        status: response.status, statusText: response.statusText, ok: response.ok, url: response.url, type: response.type, redirected: response.redirected,
        bodyUsedBefore: response.bodyUsed, header: response.headers.get('X-Probe')};
      const clone = response.clone();
      result.body = await response.text();
      result.cloneBody = await clone.text();
      result.bodyUsedAfter = response.bodyUsed;
      try { await response.text(); } catch (error) { result.secondReadError = describeError(error); }
      return result;
    } catch (error) { return {promise: promise instanceof Promise, rejected: true, error: describeError(error)}; }
  };
  out.relative = await request('relative?worker=' + label);
  out.root = await request('/root?worker=' + label);
  out.absolute = await request(base + '/absolute?worker=' + label);
  out.fetchRedirect = await request(base + '/fetch-redirect?worker=' + label);
  out.notFound = await request(base + '/not-found?worker=' + label);
  out.invalid = await request('http://[');
  const controller = new AbortController();
  controller.abort();
  out.preAbort = await request(base + '/slow?worker=' + label, {signal: controller.signal});
  const custom = new AbortController(), reason = {custom: true};
  custom.abort(reason);
  try { await fetch(base + '/slow?worker=' + label, {signal: custom.signal}); } catch (error) { out.customAbortIdentity = error === reason; }
  const during = new AbortController(), order = [];
  during.signal.addEventListener('abort', () => order.push('event'));
  const pending = fetch(base + '/slow?worker=' + label, {signal: during.signal}).catch(error => { order.push('rejection'); return error; });
  setTimeout(() => { during.abort(); order.push('after-abort'); }, 10);
  const aborted = await pending;
  out.duringAbort = {order, error: describeError(aborted), reasonIdentity: aborted === during.signal.reason};
  const bodyController = new AbortController();
  const bodyResponse = await fetch(base + '/slow-body?worker=' + label, {signal: bodyController.signal});
  bodyController.abort();
  try { await bodyResponse.text(); out.bodyAbort = {resolved: true}; }
  catch (error) { out.bodyAbort = {status: bodyResponse.status, error: describeError(error)}; }
  if (redirects) {
    out.redirectMethods = {};
    for (const [status, method] of [[301, 'POST'], [302, 'POST'], [303, 'PUT'], [307, 'POST'], [308, 'POST']]) {
      out.redirectMethods[status] = await request(base + '/redirect/' + status, {method, body: 'payload', headers: {'Content-Type': 'text/plain', 'X-Keep': 'yes'}});
    }
    out.notRedirect304 = await request(base + '/redirect/304');
    out.manualRedirect = await request(base + '/redirect/302', {redirect: 'manual'});
    out.errorRedirect = await request(base + '/redirect/302', {redirect: 'error'});
    out.redirectLoop = await request(base + '/loop?hop=0');
  }
  self.postMessage(out);
};
