// Controlled local lifecycle experiment, not the frozen Wikipedia benchmark.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const net = require('node:net');
const crypto = require('node:crypto');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright-core');

const root = path.resolve(__dirname, '../..');
const mimicPath = path.resolve(
  root,
  process.env.MIMIC_BINARY || '.build/mimic-chromium-control.exe',
);
const chromePath = path.resolve(
  root,
  process.env.CHROME_BINARY ||
    'compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe',
);
const output = path.resolve(root, process.env.OUTPUT || '.build/observation-chain-results.json');
const counts = (process.env.COUNTS || '100,10000').split(',').map(Number);
const orders = (process.env.ORDERS || 'rect,style,visible,click').split(',');
const trials = Number(process.env.TRIALS || 3);
const browsers = (process.env.BROWSERS || 'mimic,chrome').split(',');
const position = process.env.POSITION || 'before';
const diagnostics = process.env.DIAGNOSTICS === '1';
assert(['before', 'after'].includes(position));
assert(counts.every((n) => Number.isInteger(n) && n > 0));
assert(orders.every((v) => ['rect', 'style', 'visible', 'click'].includes(v)));
assert(browsers.every((v) => ['mimic', 'chrome'].includes(v)));
assert(Number.isInteger(trials) && trials > 0);
const sha = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
let activeStage;
const report = {
  startedAt: new Date().toISOString(),
  counts,
  orders,
  trials,
  position,
  diagnostics,
  viewport: { width: 1280, height: 800 },
  headless: true,
  scriptSHA256: sha(__filename),
  binaries: Object.fromEntries(
    browsers.map((name) => {
      const file = name === 'chrome' ? chromePath : mimicPath;
      return [name, { path: file, sha256: sha(file) }];
    }),
  ),
  notes: [
    'First page in a browser process is labelled processFirstPage, not Wikipedia E2E cold.',
    'Direct first rect/style executes in the same callback as DOM construction.',
    'Playwright first visible/click necessarily crosses a command boundary after construction.',
    'No assertion requires identical pixel metrics between browsers; dependency responses are checked.',
    'Timings include controller wall time and page-local direct-read time separately.',
    'DIAGNOSTICS runs include extra CDP traffic in phase/row totals and are not latency comparisons.',
    'Phase totals overlap their individual stages: never sum both. after-input follows the settled input chain.',
    'unrelated-text changes a fixed-height row; default positioning places it below the target.',
  ],
  rows: [],
  serverEvents: [],
  launches: [],
};
function save() {
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, JSON.stringify(report, null, 2));
}
function listen(server) {
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => resolve(server.address().port));
  });
}
async function unusedPort() {
  const server = net.createServer();
  const port = await listen(server);
  await new Promise((resolve) => server.close(resolve));
  return port;
}
async function launch(name) {
  if (name === 'chrome') {
    const port = await unusedPort();
    const owned = await chromium.launch({
      executablePath: chromePath,
      headless: true,
      args: [`--remote-debugging-port=${port}`],
    });
    let browser;
    try {
      browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`);
    } catch (error) {
      await owned.close();
      throw error;
    }
    return {
      browser,
      async close() {
        try {
          await browser.close();
        } finally {
          await owned.close();
        }
      },
    };
  }
  const port = await unusedPort();
  const child = spawn(mimicPath, ['-listen', `127.0.0.1:${port}`], {
    cwd: root,
    windowsHide: true,
    env: { ...process.env, ...(diagnostics ? { MIMIC_PROFILE_HOSTS: '1' } : {}) },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let logs = '';
  child.stdout.on('data', (data) => {
    logs = (logs + data).slice(-16000);
  });
  child.stderr.on('data', (data) => {
    logs = (logs + data).slice(-16000);
    if (process.env.MIMIC_CHAIN_TRACE === '1')
      report.serverEvents.push({ at: performance.now(), stage: activeStage, text: String(data) });
  });
  const endpoint = `http://127.0.0.1:${port}`;
  let browser;
  try {
    const deadline = Date.now() + 30000;
    while (Date.now() < deadline) {
      if (child.exitCode !== null) throw new Error(`Mimic exited: ${logs}`);
      try {
        const response = await fetch(`${endpoint}/json/version`);
        if (response.ok) break;
      } catch {}
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    browser = await chromium.connectOverCDP(endpoint, { timeout: 10000 });
  } catch (error) {
    child.kill();
    throw error;
  }
  return {
    browser,
    async close() {
      try {
        await browser.close();
      } finally {
        if (child.exitCode === null) {
          const exited = new Promise((resolve) => child.once('exit', resolve));
          child.kill();
          await exited;
        }
      }
    },
  };
}

async function runCase(context, origin, row) {
  const started = performance.now();
  row.stages = [];
  let page, cdp;
  const stage = async (name, fn) => {
    activeStage = { browser: row.browser, trial: row.trial, n: row.n, order: row.order, name };
    const before = cdp ? await cdp.send('Mimic.getDiagnostics') : null;
    const start = performance.now();
    try {
      const result = await fn();
      const wallMs = performance.now() - start;
      const after = cdp ? await cdp.send('Mimic.getDiagnostics') : null;
      let costs;
      if (after) {
        const old = before.costs || before.detail?.costs || {};
        const current = after.costs || after.detail?.costs || {};
        costs = Object.fromEntries(
          [...new Set([...Object.keys(old), ...Object.keys(current)])].map((key) => [
            key,
            {
              count: (current[key]?.count || 0) - (old[key]?.count || 0),
              ns: (current[key]?.ns || 0) - (old[key]?.ns || 0),
            },
          ]),
        );
      }
      const realmCosts = {};
      for (const current of after?.realms || []) {
        const prior = (before?.realms || []).find((value) => value.realm === current.realm);
        const oldCosts = prior?.diagnostics?.costs || {};
        const newCosts = current.diagnostics?.costs || {};
        realmCosts[current.realm] = Object.fromEntries(
          Object.entries(newCosts)
            .map(([key, value]) => [
              key,
              {
                count: value.count - (oldCosts[key]?.count || 0),
                ns: value.ns - (oldCosts[key]?.ns || 0),
              },
            ])
            .filter(([, value]) => value.count || value.ns),
        );
      }
      row.stages.push({ name, wallMs, result, costs, realmCosts });
      return result;
    } catch (error) {
      row.stages.push({ name, wallMs: performance.now() - start, error: String(error) });
      throw error;
    }
  };
  try {
    page = await stage('page.create', () => context.newPage());
    row.stages[row.stages.length - 1].result = undefined;
    if (diagnostics && row.browser === 'mimic') cdp = await context.newCDPSession(page);
    await page.setViewportSize(report.viewport);
    page.setDefaultTimeout(30000);
    await stage('page.navigate', async () => {
      await page.goto(origin, { waitUntil: 'domcontentloaded' });
      return true;
    });
    const setup = await stage('fixture.build-and-first-direct', () =>
      page.evaluate(
        ({ n, order, position }) => {
          const start = performance.now();
          const container = document.createElement('section');
          container.id = 'fixture';
          container.style.width = '800px';
          const button = '<button id="target" type="button">Probe</button>';
          const rows = '<div class="row"></div>'.repeat(n);
          container.innerHTML = position === 'before' ? button + rows : rows + button;
          document.body.appendChild(container);
          window.probeClicks = 0;
          document.getElementById('target').addEventListener('click', () => {
            window.probeClicks++;
          });
          const built = performance.now();
          let first = null;
          const target = document.getElementById('target');
          if (order === 'rect') {
            const value = target.getBoundingClientRect();
            first = { width: value.width, height: value.height, x: value.x, y: value.y };
          } else if (order === 'style') {
            first = { display: getComputedStyle(target).display };
          }
          return { buildMs: built - start, firstDirectMs: performance.now() - built, first };
        },
        { n: row.n, order: row.order, position },
      ),
    );
    row.buildMs = setup.buildMs;
    const target = page.locator('#target');
    let expectedClicks = 0;
    if (row.order === 'visible') {
      assert.equal(await stage('first.visible', () => target.isVisible()), true);
    } else if (row.order === 'click') {
      await stage('first.click', async () => {
        await target.click();
        return true;
      });
      expectedClicks++;
    }
    const direct = (name) =>
      stage(name, () =>
        page.evaluate(() => {
          const target = document.getElementById('target');
          let start = performance.now();
          const box = target.getBoundingClientRect();
          const rectMs = performance.now() - start;
          start = performance.now();
          const display = getComputedStyle(target).display;
          return {
            rectMs,
            styleMs: performance.now() - start,
            width: box.width,
            height: box.height,
            x: box.x,
            y: box.y,
            display,
          };
        }),
      );
    let prior;
    for (const phase of [
      'settle',
      'after-input',
      'irrelevant-attribute',
      'width',
      'unrelated-text',
    ]) {
      const phaseStart = performance.now();
      if (phase === 'irrelevant-attribute' || phase === 'width' || phase === 'unrelated-text') {
        await stage(`${phase}.mutate`, () =>
          page.evaluate((kind) => {
            const container = document.getElementById('fixture');
            const row = container.querySelector('.row');
            if (kind === 'irrelevant-attribute') row.setAttribute('data-probe-unused', 'changed');
            if (kind === 'width') container.style.width = '640px';
            if (kind === 'unrelated-text') row.textContent = 'Changed row text';
            return true;
          }, phase),
        );
      }
      const value = await direct(`${phase}.direct`);
      assert(value.width > 0 && value.height > 0);
      if (phase === 'after-input' || phase === 'irrelevant-attribute') {
        assert.equal(value.width, prior.width);
        assert.equal(value.height, prior.height);
      }
      if (phase === 'width')
        assert(value.width < prior.width, 'Ancestor width must affect target width');
      assert.equal(await stage(`${phase}.visible`, () => target.isVisible()), true);
      assert.equal(
        await stage(`${phase}.role`, () =>
          page.getByRole('button', { name: 'Probe', exact: true }).count(),
        ),
        1,
      );
      await stage(`${phase}.scroll`, async () => {
        await target.scrollIntoViewIfNeeded();
        return true;
      });
      const domClick = await stage(`${phase}.dom-click`, () =>
        page.evaluate(() => {
          const start = performance.now();
          document.getElementById('target').click();
          return { pageMs: performance.now() - start, clicks: window.probeClicks };
        }),
      );
      expectedClicks++;
      assert.equal(domClick.clicks, expectedClicks);
      const mouseBox = position === 'after' ? await direct(`${phase}.mouse-position`) : value;
      await stage(`${phase}.mouse-click`, async () => {
        await page.mouse.click(mouseBox.x + mouseBox.width / 2, mouseBox.y + mouseBox.height / 2);
        return true;
      });
      expectedClicks++;
      await stage(`${phase}.locator-click`, async () => {
        await target.click();
        return true;
      });
      expectedClicks++;
      const state = await stage(`${phase}.verify`, () =>
        page.evaluate(() => ({
          clicks: window.probeClicks,
          rows: document.getElementById('fixture').children.length - 1,
        })),
      );
      assert.equal(state.clicks, expectedClicks);
      assert.equal(state.rows, row.n);
      row.stages.push({ name: `${phase}.total`, wallMs: performance.now() - phaseStart });
      prior = value;
      if (phase === 'settle') {
        // Separate commands distinguish durable reuse from one callback's cache.
        for (let i = 0; i < 5; i++) await direct(`settled-separate-repeat.${i}`);
      }
    }
    // Separate commands test cross-callback reuse, unlike a loop in one callback.
    for (let i = 0; i < 5; i++) await direct(`separate-repeat.${i}`);
    if (process.env.EXTRA_REPEATS === '1') {
      for (let i = 0; i < 3; i++) {
        assert.equal(
          await stage(`clean-role.${i}`, () =>
            page.getByRole('button', { name: 'Probe', exact: true }).count(),
          ),
          1,
        );
        assert.equal(await stage(`clean-visible.${i}`, () => target.isVisible()), true);
        await stage(`clean-scroll.${i}`, async () => {
          await target.scrollIntoViewIfNeeded();
          return true;
        });
      }
      await stage('raf-cadence', () =>
        page.evaluate(
          () =>
            new Promise((resolve) => {
              const started = performance.now(),
                callbacks = [];
              const next = (timestamp) => {
                callbacks.push({ timestamp, elapsed: performance.now() - started });
                if (callbacks.length === 5) resolve(callbacks);
                else requestAnimationFrame(next);
              };
              requestAnimationFrame(next);
            }),
        ),
      );
    }
    row.ok = true;
  } catch (error) {
    row.ok = false;
    row.error = error.stack || String(error);
  } finally {
    row.totalMs = performance.now() - started;
    if (page) await page.close().catch(() => {});
    save();
  }
}

async function main() {
  const server = http.createServer((req, res) => {
    res.writeHead(200, { 'content-type': 'text/html', 'cache-control': 'no-store' });
    res.end(
      '<!doctype html><meta charset="utf-8"><style>' +
        'html,body{margin:0;padding:0}body{font:16px/16px Arial}' +
        '.row{height:2px}#target{display:block;width:100%;height:32px;padding:0;border:0}' +
        '</style><body></body>',
    );
  });
  const port = await listen(server);
  save();
  try {
    for (let trial = 0; trial < trials; trial++) {
      const order = trial % 2 ? [...browsers].reverse() : browsers;
      for (const name of order) {
        const launchStart = performance.now();
        const running = await launch(name);
        report.launches.push({ browser: name, trial, wallMs: performance.now() - launchStart });
        try {
          const context = running.browser.contexts()[0] || (await running.browser.newContext());
          report.binaries[name].browserVersion = running.browser.version();
          let pageIndex = 0;
          let cases = counts.flatMap((n) => orders.map((first) => ({ n, first })));
          if (trial % 2) cases.reverse();
          const shift = (Math.floor(trial / 2) * orders.length) % cases.length;
          cases = cases.slice(shift).concat(cases.slice(0, shift));
          for (const { n, first } of cases) {
            const row = {
              browser: name,
              trial,
              n,
              order: first,
              processFirstPage: pageIndex++ === 0,
            };
            report.rows.push(row);
            await runCase(context, `http://127.0.0.1:${port}/`, row);
            console.log(
              JSON.stringify({
                browser: name,
                trial,
                n,
                order: first,
                ok: row.ok,
                totalMs: row.totalMs,
                error: row.error,
              }),
            );
          }
        } finally {
          await running.close();
        }
      }
    }
  } finally {
    await new Promise((resolve) => server.close(resolve));
    report.finishedAt = new Date().toISOString();
    save();
  }
  if (report.rows.some((row) => !row.ok)) process.exitCode = 1;
}
main().catch((error) => {
  report.fatal = error.stack || String(error);
  save();
  console.error(error);
  process.exitCode = 1;
});
