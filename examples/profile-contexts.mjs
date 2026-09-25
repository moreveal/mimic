// Node 22+: bounded raw-CDP scraping with independent portable profiles.
// Start the local fixture with `npm run fixture` before running this example.
const endpoint = process.env.MIMIC_URL || 'http://127.0.0.1:9222';
const targetURL = process.env.TARGET_URL || 'http://127.0.0.1:3000';
const concurrency = Number(process.env.CONCURRENCY || 8);
if (!Number.isInteger(concurrency) || concurrency < 1 || concurrency > 100) {
  throw new Error('CONCURRENCY must be an integer from 1 to 100');
}
const holdAllLive = process.env.HOLD_ALL_LIVE === '1';
if (holdAllLive && concurrency !== 100) {
  throw new Error('HOLD_ALL_LIVE=1 requires CONCURRENCY=100');
}
const proxies = JSON.parse(process.env.PROXIES_JSON || '[]');
if (
  !Array.isArray(proxies) ||
  proxies.some((proxy) => !proxy || typeof proxy.server !== 'string')
) {
  throw new Error('PROXIES_JSON must be an array of {server, username?, password?} objects');
}
// Optional: 100 arrays of CDP CookieParam objects, one array per task.
// Without this input the example installs a distinct demonstration cookie.
const cookieSets = JSON.parse(process.env.COOKIES_JSON || 'null');
if (
  cookieSets !== null &&
  (!Array.isArray(cookieSets) ||
    cookieSets.length !== 100 ||
    cookieSets.some(
      (set) =>
        !Array.isArray(set) ||
        set.some(
          (cookie) =>
            !cookie || typeof cookie.name !== 'string' || typeof cookie.value !== 'string',
        ),
    ))
) {
  throw new Error('COOKIES_JSON must contain 100 arrays of {name, value, url?} cookies');
}
if (process.env.RESOURCE_POLICY && process.env.RESOURCE_POLICY !== 'dataExtraction') {
  throw new Error('RESOURCE_POLICY currently accepts dataExtraction');
}
// Resource blocking can change site behavior and fingerprint observations.
// Enable it only after checking the target workload.
const resourcePolicy =
  process.env.RESOURCE_POLICY === 'dataExtraction' ? { presets: ['dataExtraction'] } : undefined;
const discovery = await fetch(`${endpoint}/json/version`).then((r) => r.json());
const socket = new WebSocket(discovery.webSocketDebuggerUrl);
await new Promise((resolve, reject) => {
  socket.addEventListener('open', resolve, { once: true });
  socket.addEventListener('error', reject, { once: true });
});
let sequence = 0;
const pending = new Map();
const events = new Map();
socket.addEventListener('message', ({ data }) => {
  const message = JSON.parse(data);
  if (message.id) {
    const request = pending.get(message.id);
    if (!request) return;
    pending.delete(message.id);
    clearTimeout(request.timer);
    if (message.error) request.reject(new Error(JSON.stringify(message.error)));
    else request.resolve(message.result);
  } else {
    events.get(`${message.sessionId}:${message.method}`)?.resolve(message.params);
  }
});
socket.addEventListener('close', () => {
  for (const request of pending.values()) {
    clearTimeout(request.timer);
    request.reject(new Error('CDP connection closed'));
  }
  pending.clear();
  for (const event of [...events.values()]) event.reject(new Error('CDP connection closed'));
});
function send(method, params = {}, sessionId) {
  return new Promise((resolve, reject) => {
    const id = ++sequence;
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error(`${method} timed out`));
    }, 30000);
    pending.set(id, { resolve, reject, timer });
    socket.send(JSON.stringify({ id, method, params, sessionId }));
  });
}
function nextEvent(sessionId, method) {
  const key = `${sessionId}:${method}`;
  let cancel;
  const promise = new Promise((resolve, reject) => {
    const finish = (callback, value) => {
      clearTimeout(timer);
      events.delete(key);
      callback(value);
    };
    const timer = setTimeout(() => finish(reject, new Error(`${method} timed out`)), 30000);
    cancel = () => finish(resolve);
    events.set(key, {
      resolve: (value) => finish(resolve, value),
      reject: (error) => finish(reject, error),
    });
  });
  // Navigation errors can arrive before the caller awaits this event.
  promise.catch(() => {});
  return { promise, cancel: () => cancel() };
}

let next = 0;
let failed = 0;
let allLiveResolve;
let allLiveCount = 0;
const usedProfileIds = new Set();
const allLive = new Promise((resolve) => {
  allLiveResolve = resolve;
});
try {
  console.error('Mimic:', await send('Mimic.getVersion'));
  // Import manual settings explicitly when needed; generated mode is the default.
  let manual;
  if (process.env.PROFILE_MODE === 'manual') {
    manual = await send('Mimic.importProfile', {
      mode: 'manual',
      profile: { locale: { languages: ['en-US', 'en'], timezone: 'America/New_York' } },
    });
    console.error(manual.warnings);
  }
  await Promise.all(
    Array.from({ length: concurrency }, async () => {
      while (next < 100) {
        const index = next++;
        let browserContextId;
        try {
          let context;
          for (let attempt = 0; attempt < 8; attempt++) {
            context = await send('Mimic.createContext', {
              // Omission generates a fresh Context identity. A seed prefix
              // makes the same 100 identities reproducible across runs.
              ...(manual
                ? { profile: manual.profile }
                : process.env.FINGERPRINT_SEED_PREFIX
                  ? {
                      profile: {
                        generate: {
                          seed: `${process.env.FINGERPRINT_SEED_PREFIX}-${index}-${attempt}`,
                        },
                      },
                    }
                  : {}),
              ...(proxies.length ? { proxy: proxies[index % proxies.length] } : {}),
              ...(resourcePolicy ? { resourcePolicy } : {}),
              disposeOnDetach: true,
            });
            if (manual || !usedProfileIds.has(context.profileId)) break;
            await send('Target.disposeBrowserContext', {
              browserContextId: context.browserContextId,
            });
            context = undefined;
          }
          if (!context) throw new Error('Could not generate a distinct profile after 8 attempts');
          usedProfileIds.add(context.profileId);
          browserContextId = context.browserContextId;
          const cookies = (
            cookieSets?.[index] || [{ name: 'mimic_task', value: `task-${index}` }]
          ).map((cookie) => ({ ...cookie, url: cookie.url || targetURL }));
          await send('Storage.setCookies', { browserContextId, cookies });
          const { targetId } = await send('Target.createTarget', {
            browserContextId,
            url: 'about:blank',
          });
          const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
          await send('Page.enable', {}, sessionId);
          const loaded = nextEvent(sessionId, 'Page.loadEventFired');
          try {
            const navigation = await send('Page.navigate', { url: targetURL }, sessionId);
            if (navigation.errorText) throw new Error(navigation.errorText);
            await loaded.promise;
          } finally {
            loaded.cancel();
          }
          const result = await send(
            'Runtime.evaluate',
            {
              expression: `({
                title: document.title,
                heading: document.querySelector('h1')?.textContent,
                links: document.querySelectorAll('a').length,
                width: innerWidth
              })`,
              returnByValue: true,
            },
            sessionId,
          );
          if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails));
          if (holdAllLive) {
            if (++allLiveCount === 100) allLiveResolve();
            await allLive;
          }
          console.log(
            JSON.stringify({ index, profileId: context.profileId, result: result.result.value }),
          );
        } catch (error) {
          failed++;
          if (holdAllLive) allLiveResolve(); // A failed job cannot fill the barrier.
          console.error(`Job ${index}:`, error.message);
        } finally {
          if (browserContextId) await send('Target.disposeBrowserContext', { browserContextId });
        }
      }
    }),
  );
} finally {
  socket.close();
}
if (failed) process.exitCode = 1;
