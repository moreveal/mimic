const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const root = path.resolve(__dirname, '../../..');
const artifacts = path.join(root, '.build');
const experiment = process.argv[2] || 'cascade';
const candidate = process.env.POC_BINARY
  ? 'candidate'
  : experiment === 'matching'
    ? 'matchingReplay'
    : experiment === 'cascadeReplay'
      ? 'cascadeReplay'
      : 'noStylesheet';
const binaries = {
  control: process.env.POC_CONTROL_BINARY || path.join(artifacts, 'mimic-wiki-control.exe'),
  [candidate]:
    process.env.POC_BINARY ||
    path.join(
      artifacts,
      experiment === 'matching'
        ? 'mimic-poc-matching-tape.exe'
        : experiment === 'cascadeReplay'
          ? 'mimic-poc-cascade-tape.exe'
          : 'mimic-poc-no-stylesheet.exe',
    ),
};
const env = { ...process.env, PW_MIMIC_ENDPOINT: 'http://127.0.0.1:9438' };
for (const key of Object.keys(env)) {
  if (
    key.startsWith('MIMIC_PROFILE_') ||
    key === 'MIMIC_DIAGNOSTICS' ||
    key === 'PW_CHROME_EXECUTABLE'
  )
    delete env[key];
}
const results = [];
async function workload(label) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, ['tools/runtimecheck/playwright_wikipedia_local.js'], {
      cwd: root,
      env,
      windowsHide: true,
    });
    let output = '';
    child.stdout.on('data', (data) => (output += data));
    child.stderr.on('data', (data) => (output += data));
    child.on('error', reject);
    child.on('exit', (code) => {
      fs.writeFileSync(path.join(artifacts, label + '.log'), output);
      const match = output.match(/RESULT: PASS \((\d+) ms\)/);
      if (code || !match) return reject(new Error(label + ' failed: ' + output.slice(-2000)));
      const stages = [...output.matchAll(/\[MEASURE\] (.*): ([\d.]+) ms/g)].map((m) => [
        m[1],
        Number(m[2]),
      ]);
      resolve({ milliseconds: Number(match[1]), stages: Object.fromEntries(stages) });
    });
  });
}
async function pair(build, trial) {
  const serverEnv = { ...env };
  if (build === 'control' && process.env.POC_CONTROL_ENV)
    Object.assign(serverEnv, JSON.parse(process.env.POC_CONTROL_ENV));
  if (['matching', 'cascadeReplay'].includes(experiment) && build === candidate) {
    serverEnv.MIMIC_STYLE_TAPE_MODE = 'replay';
    serverEnv.MIMIC_STYLE_TAPE_PATH = path.join(
      artifacts,
      experiment === 'matching' ? 'matching-tape.jsonl' : 'cascade-tape.jsonl',
    );
    if (experiment === 'cascadeReplay') serverEnv.MIMIC_STYLE_TAPE_LAYER = 'cascade';
  }
  const child = spawn(binaries[build], ['-listen', '127.0.0.1:9438'], {
    cwd: root,
    env: serverEnv,
    windowsHide: true,
    stdio: process.env.POC_SERVER_LOG ? ['ignore', 'pipe', 'pipe'] : 'ignore',
  });
  if (process.env.POC_SERVER_LOG) {
    const log = path.join(artifacts, `${experiment}-${trial}-${build}-server.log`);
    fs.writeFileSync(log, '');
    child.stdout.on('data', (data) => fs.appendFileSync(log, data));
    child.stderr.on('data', (data) => fs.appendFileSync(log, data));
  }
  const closed = new Promise((resolve) => child.on('exit', resolve));
  try {
    let ready = false;
    for (let i = 0; i < 100; i++) {
      try {
        await fetch(env.PW_MIMIC_ENDPOINT + '/json/version');
        ready = true;
        break;
      } catch {}
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    if (!ready) throw new Error('Server did not start');
    const cold = await workload(`${experiment}-${trial}-${build}-cold`);
    const warm = await workload(`${experiment}-${trial}-${build}-warm`);
    results.push({
      build,
      trial,
      diagnosticEnvironment: Object.fromEntries(
        Object.entries(serverEnv).filter(([key]) => key.startsWith('MIMIC_POC_')),
      ),
      cold,
      warm,
    });
    fs.writeFileSync(
      path.join(artifacts, experiment + '-paired-results.json'),
      JSON.stringify(
        {
          binaries: Object.fromEntries(
            Object.entries(binaries).map(([name, file]) => [
              name,
              {
                file,
                sha256: crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex'),
              },
            ]),
          ),
          diagnosticEnvironment: Object.fromEntries(
            Object.entries(serverEnv).filter(([key]) => key.startsWith('MIMIC_POC_')),
          ),
          results,
        },
        null,
        2,
      ),
    );
    console.log(JSON.stringify({ build, trial, cold: cold.milliseconds, warm: warm.milliseconds }));
  } finally {
    child.kill();
    await closed;
  }
}
(async () => {
  const trials = Number(process.env.POC_TRIALS || 5);
  if (!Number.isInteger(trials) || trials < 1) throw new Error('POC_TRIALS must be positive');
  for (const file of Object.values(binaries))
    if (!fs.existsSync(file)) throw new Error('Missing binary: ' + file);
  const existing = await fetch(env.PW_MIMIC_ENDPOINT + '/json/version', {
    signal: AbortSignal.timeout(1000),
  }).catch(() => null);
  if (existing) throw new Error('Port 9438 is already serving HTTP; refusing ambiguous timing');
  for (let trial = 1; trial <= trials; trial++) {
    for (const build of trial % 2 ? ['control', candidate] : [candidate, 'control'])
      await pair(build, trial);
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
