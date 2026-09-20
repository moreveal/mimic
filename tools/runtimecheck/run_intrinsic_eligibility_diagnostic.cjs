// Diagnostic traffic is deliberately enabled; these are not timing-gate runs.
const fs = require('node:fs');
const path = require('node:path');
const net = require('node:net');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const root = path.resolve(__dirname, '../..');
const build = path.join(root, '.build');
const variant = process.env.INTRINSIC_DIAGNOSTIC_VARIANT || 'intrinsic-eligibility';
fs.mkdirSync(build, { recursive: true });
async function unusedPort() {
  const probe = net.createServer();
  probe.listen(0, '127.0.0.1');
  await once(probe, 'listening');
  const port = probe.address().port;
  await new Promise((resolve, reject) =>
    probe.close((error) => (error ? reject(error) : resolve())),
  );
  return port;
}
function start(command, args, env, logPath) {
  const log = fs.createWriteStream(logPath);
  const child = spawn(command, args, {
    cwd: root,
    env,
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.pipe(log, { end: false });
  child.stderr.pipe(log, { end: false });
  const closed = new Promise((resolve) => {
    let spawnError;
    child.once('error', (error) => {
      spawnError = error;
    });
    child.once('close', (code, signal) =>
      log.end(() => resolve({ code, signal, error: spawnError })),
    );
  });
  return { child, closed };
}
async function main() {
  const endpoint = `http://127.0.0.1:${await unusedPort()}`;
  const server = start(
    path.join(build, `mimic-${variant}.exe`),
    ['-listen', new URL(endpoint).host],
    {
      ...process.env,
      MIMIC_CHAIN_TRACE: '1',
      MIMIC_PROFILE_HOSTS: '1',
      MIMIC_PROFILE_CDP: '1',
    },
    path.join(build, `${variant}-server.log`),
  );
  try {
    const deadline = Date.now() + 30000;
    let ready = false;
    while (Date.now() < deadline) {
      if (server.child.exitCode !== null || server.child.signalCode !== null || !server.child.pid)
        throw new Error(`Server exited before readiness: ${JSON.stringify(await server.closed)}`);
      try {
        const response = await fetch(`${endpoint}/json/version`, {
          signal: AbortSignal.timeout(1000),
        });
        if (response.ok) {
          ready = true;
          break;
        }
      } catch {}
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    if (!ready) throw new Error('Server readiness timed out after 30 seconds');
    for (const mode of process.env.INTRINSIC_DIAGNOSTIC_COLD_ONLY ? ['cold'] : ['cold', 'warm']) {
      const workload = start(
        process.execPath,
        ['tools/runtimecheck/playwright_wikipedia_profile.js'],
        {
          ...process.env,
          PW_MIMIC_ENDPOINT: endpoint,
          MIMIC_PROFILE_STAGE_DIAGNOSTICS: '1',
          MIMIC_PROFILE_EXPRESSION:
            '({url: location.href, stats: globalThis.__intrinsicEligibility || null})',
          MIMIC_PROFILE_OUTPUT: path.join(build, `${variant}-${mode}.json`),
        },
        path.join(build, `${variant}-${mode}.log`),
      );
      let timedOut = false;
      const timer = setTimeout(() => {
        timedOut = true;
        workload.child.kill();
      }, 120000);
      const result = await workload.closed;
      clearTimeout(timer);
      if (timedOut || result.error || result.code !== 0)
        throw new Error(`${mode} diagnostic failed: ${JSON.stringify({ timedOut, ...result })}`);
      console.log(`${mode} diagnostic complete; diagnostic timings are not benchmark results.`);
    }
  } finally {
    if (server.child.exitCode === null && server.child.signalCode === null) server.child.kill();
    await server.closed;
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
