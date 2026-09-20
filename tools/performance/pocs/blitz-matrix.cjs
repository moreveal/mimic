// Uses the unchanged Wikipedia workload. Each cold/warm pair owns a fresh
// process; candidate/control order alternates. No output is overwritten.
const { spawn, execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const root = path.resolve(__dirname, '../../..');
const binaries = {
  baseline: path.resolve(
    process.env.BLITZ_BASELINE || '../blitz-baseline/.build/mimic-baseline.exe',
  ),
  blitz: path.resolve(root, '.build/mimic-blitz.exe'),
};
const output = path.resolve(root, '.build', 'blitz-matrix-' + Date.now());
fs.mkdirSync(output);
const endpoint = 'http://127.0.0.1:9487';
const records = [];
const sha = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const metadata = {
  binaries: Object.fromEntries(
    Object.entries(binaries).map(([name, file]) => [name, { file, sha256: sha(file) }]),
  ),
  workloadSHA256: sha(path.join(root, 'tools/runtimecheck/playwright_wikipedia_local.js')),
  pairs: Number(process.env.BLITZ_PAIRS || 5),
};
fs.writeFileSync(path.join(output, 'metadata.json'), JSON.stringify(metadata, null, 2));
function memory(pid) {
  if (process.platform !== 'win32') return null;
  try {
    return JSON.parse(
      execFileSync(
        'powershell.exe',
        [
          '-NoProfile',
          '-Command',
          `Get-Process -Id ${Number(pid)} | Select-Object WorkingSet64,PrivateMemorySize64,PeakWorkingSet64 | ConvertTo-Json -Compress`,
        ],
        { windowsHide: true, encoding: 'utf8' },
      ),
    );
  } catch {
    return null;
  }
}
async function pair(mode, iteration) {
  const server = spawn(binaries[mode], ['-listen', '127.0.0.1:9487'], {
    cwd: root,
    windowsHide: true,
    env: {
      ...process.env,
      MIMIC_STYLE_ENGINE: mode === 'blitz' ? 'blitz' : 'legacy',
      MIMIC_PROFILE_BLITZ: '0',
    },
  });
  const ended = new Promise((resolve) => server.once('exit', resolve));
  let log = '';
  server.stdout.on('data', (data) => {
    log += data;
  });
  server.stderr.on('data', (data) => {
    log += data;
  });
  try {
    let ready = false;
    for (let i = 0; i < 150; i++) {
      try {
        if ((await fetch(endpoint + '/json/version')).ok) {
          ready = true;
          break;
        }
      } catch {}
      if (server.exitCode !== null) throw new Error('server exited: ' + log);
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    if (!ready) throw new Error('server startup timeout');
    for (const phase of ['cold', 'warm']) {
      const result = await new Promise((resolve, reject) => {
        const child = spawn(
          process.execPath,
          ['tools/runtimecheck/playwright_wikipedia_local.js'],
          {
            cwd: root,
            windowsHide: true,
            env: { ...process.env, PW_MIMIC_ENDPOINT: endpoint },
          },
        );
        let text = '';
        child.stdout.on('data', (data) => {
          text += data;
        });
        child.stderr.on('data', (data) => {
          text += data;
        });
        child.on('error', reject);
        child.on('exit', (code) => resolve({ code, text }));
      });
      fs.writeFileSync(path.join(output, `${iteration}-${mode}-${phase}.log`), result.text);
      const match = /RESULT: PASS \((\d+) ms\)/.exec(result.text);
      if (result.code || !match)
        throw new Error(`${mode}/${phase} failed: ${result.text.slice(-2000)}`);
      const stages = Object.fromEntries(
        [...result.text.matchAll(/\[MEASURE\] (.+): ([\d.]+) ms/g)].map((m) => [
          m[1],
          Number(m[2]),
        ]),
      );
      const record = {
        iteration,
        mode,
        phase,
        wallMS: Number(match[1]),
        stages,
        afterPageClose: memory(server.pid),
      };
      records.push(record);
      fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify(records, null, 2));
      console.log(JSON.stringify(record));
    }
  } finally {
    server.kill();
    await ended;
    fs.writeFileSync(path.join(output, `${iteration}-${mode}-server.log`), log);
  }
}
(async () => {
  console.log('Receipts: ' + output);
  for (let i = 0; i < metadata.pairs; i++)
    for (const mode of i % 2 ? ['blitz', 'baseline'] : ['baseline', 'blitz']) await pair(mode, i);
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
