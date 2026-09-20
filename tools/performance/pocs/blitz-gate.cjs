const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '../../..');
const binary = path.resolve(root, process.env.BLITZ_BINARY || '.build/mimic-blitz.exe');
const mode = process.env.BLITZ_GATE_MODE || 'blitz';
const endpoint = 'http://127.0.0.1:9487';

async function run() {
  const server = spawn(binary, ['-listen', '127.0.0.1:9487'], {
    cwd: root,
    env: { ...process.env, MIMIC_STYLE_ENGINE: mode },
    windowsHide: true,
  });
  let serverLog = '';
  server.stdout.on('data', (data) => {
    serverLog += data;
  });
  server.stderr.on('data', (data) => {
    serverLog += data;
  });
  try {
    let ready = false;
    for (let i = 0; i < 100; i++) {
      try {
        const response = await fetch(endpoint + '/json/version');
        if (response.ok) {
          ready = true;
          break;
        }
      } catch {}
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    if (!ready) throw new Error('server did not become ready: ' + serverLog);
    for (const phase of ['cold', 'warm']) {
      const result = await new Promise((resolve, reject) => {
        const child = spawn(
          process.execPath,
          ['tools/runtimecheck/playwright_wikipedia_local.js'],
          {
            cwd: root,
            env: { ...process.env, PW_MIMIC_ENDPOINT: endpoint },
            windowsHide: true,
          },
        );
        let output = '';
        child.stdout.on('data', (data) => {
          output += data;
        });
        child.stderr.on('data', (data) => {
          output += data;
        });
        child.on('error', reject);
        child.on('exit', (code) => resolve({ code, output }));
      });
      fs.writeFileSync(path.join(root, '.build', `blitz-${mode}-${phase}.log`), result.output);
      console.log(result.output.slice(-6000));
      if (result.code) throw new Error(`${phase} gate failed (${result.code})`);
    }
  } finally {
    fs.writeFileSync(path.join(root, '.build', `blitz-${mode}-server.log`), serverLog);
    server.kill();
  }
}
run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
