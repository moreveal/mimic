import { chromium } from 'playwright-core';
import { fileURLToPath } from 'node:url';

const targetURL = process.env.TARGET_URL || 'http://127.0.0.1:5187';
const mimicURL = process.env.MIMIC_URL || 'http://127.0.0.1:9222';
const chromePath =
  process.env.CHROME_PATH ||
  fileURLToPath(
    new URL(
      '../compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe',
      import.meta.url,
    ),
  );

async function run(label, browser) {
  const context = label === 'Chrome' ? await browser.newContext() : browser.contexts()[0];
  const page = await context.newPage();
  const events = [];
  page.on('pageerror', (error) => events.push(`pageerror ${error.message}`));
  page.on('console', (message) => {
    if (message.type() === 'error') events.push(`console ${message.text()}`);
  });
  const steps = [];
  async function step(name, action) {
    try {
      await action();
      steps.push([name, 'ok']);
    } catch (error) {
      steps.push([name, error.message.split('\n')[0]]);
    }
  }
  await step('navigate', async () => {
    await page.goto(new URL('/probe', targetURL).href);
    await page.locator('#increment').waitFor();
    await page.waitForFunction(() => typeof globalThis.Blazor?.disconnect === 'function', null, {
      timeout: 5000,
    });
  });
  await step('counter', async () => {
    await page.locator('#increment').click();
    await page.locator('#increment').click();
    await page.waitForFunction(
      () => document.querySelector('#count')?.textContent === 'Count: 2',
      null,
      { timeout: 5000 },
    );
  });
  await step('bind', async () => {
    await page.locator('#name').fill('Ada');
    await page.waitForFunction(
      () => document.querySelector('#mirror')?.textContent === 'Name: Ada',
      null,
      { timeout: 5000 },
    );
  });
  await step('add', async () => {
    await page.locator('#add').click();
    await page.waitForFunction(() => document.querySelectorAll('#items li').length === 1, null, {
      timeout: 5000,
    });
  });
  await step('reverse/remove', async () => {
    await page.locator('#name').fill('Bea');
    await page.locator('#add').click();
    await page.waitForFunction(() => document.querySelectorAll('#items li').length === 2, null, {
      timeout: 5000,
    });
    await page.locator('#reverse').click();
    await page.waitForFunction(
      () => document.querySelector('#items li span')?.textContent === 'Bea',
      null,
      { timeout: 5000 },
    );
    await page.locator('#items li').first().getByRole('button', { name: 'Remove' }).click();
    await page.waitForFunction(() => document.querySelectorAll('#items li').length === 1, null, {
      timeout: 5000,
    });
  });
  await step('async', async () => {
    await page.locator('#double').click();
    await page.waitForFunction(
      () => document.querySelector('#count')?.textContent === 'Count: 4',
      null,
      { timeout: 5000 },
    );
  });
  await step('js interop', async () => {
    await page.locator('#js').click();
    await page.waitForFunction(
      () => document.querySelector('#js-result')?.textContent === 'JS: ok:1:/probe',
      null,
      { timeout: 5000 },
    );
  });
  await step('select/checkbox', async () => {
    await page.locator('#choice').selectOption('two');
    await page.locator('#enabled').check();
    await page.waitForFunction(
      () => document.querySelector('#selection')?.textContent === 'Selection: two/True',
      null,
      { timeout: 5000 },
    );
  });
  await step('form validation', async () => {
    await page.locator('#save').click();
    await page.locator('.validation-message').waitFor({ timeout: 5000 });
    await page.locator('#email').fill('ada@example.test');
    await page.locator('#save').click();
    await page.waitForFunction(
      () => document.querySelector('#saved')?.textContent === 'Saved: ada@example.test',
      null,
      { timeout: 5000 },
    );
  });
  await step('JS to .NET callback', async () => {
    await page.locator('#js-callback').click();
    await page.waitForFunction(
      () => document.querySelector('#callback-result')?.textContent === 'Callback: pong',
      null,
      { timeout: 5000 },
    );
  });
  const snapshot = await page.evaluate(() => ({
    count: document.querySelector('#count')?.textContent,
    mirror: document.querySelector('#mirror')?.textContent,
    items: Array.from(
      document.querySelectorAll('#items li span'),
      (element) => element.textContent,
    ),
    busy: document.querySelector('#busy')?.textContent,
    js: document.querySelector('#js-result')?.textContent,
    selection: document.querySelector('#selection')?.textContent,
    saved: document.querySelector('#saved')?.textContent,
    callback: document.querySelector('#callback-result')?.textContent,
  }));
  await step('stream navigation', async () => {
    await page.locator('#stream-link').click();
    await page.waitForURL('**/stream', { timeout: 5000 });
    await page.waitForFunction(
      () => document.querySelector('#stream-state')?.textContent === 'ready',
      null,
      { timeout: 5000 },
    );
  });
  await step('back navigation', async () => {
    await page.locator('#back-link').click();
    await page.waitForURL('**/probe', { timeout: 5000 });
    await page.locator('#increment').waitFor({ timeout: 5000 });
  });
  await step('independent circuits', async () => {
    const peer = await context.newPage();
    try {
      await peer.goto(new URL('/probe', targetURL).href);
      await peer.waitForFunction(() => typeof globalThis.Blazor?.disconnect === 'function', null, {
        timeout: 5000,
      });
      await peer.locator('#increment').click();
      await peer.waitForFunction(
        () => document.querySelector('#count')?.textContent === 'Count: 1',
        null,
        {
          timeout: 5000,
        },
      );
      const original = await page.locator('#count').textContent();
      if (original !== 'Count: 0') throw new Error(`original circuit changed: ${original}`);
    } finally {
      await peer.close();
    }
  });
  const result = { label, steps, snapshot, finalURL: page.url(), events };
  await page.close();
  if (label === 'Chrome') await context.close();
  return result;
}

const chrome = await chromium.launch({ headless: true, executablePath: chromePath });
let chromeResult;
const chromeVersion = chrome.version();
try {
  chromeResult = await run('Chrome', chrome);
} finally {
  await chrome.close();
}
const mimic = await chromium.connectOverCDP(mimicURL);
let mimicResult;
try {
  mimicResult = await run('Mimic', mimic);
} finally {
  await mimic.close();
}
const differences = [];
const baselineFailures = chromeResult.steps.filter(([, status]) => status !== 'ok');
for (let index = 0; index < chromeResult.steps.length; index++) {
  const [name, chromeStatus] = chromeResult.steps[index];
  const mimicStatus = mimicResult.steps[index]?.[1];
  if (chromeStatus !== mimicStatus)
    differences.push({ step: name, chrome: chromeStatus, mimic: mimicStatus });
}
for (const key of Object.keys(chromeResult.snapshot)) {
  if (JSON.stringify(chromeResult.snapshot[key]) !== JSON.stringify(mimicResult.snapshot[key]))
    differences.push({
      field: key,
      chrome: chromeResult.snapshot[key],
      mimic: mimicResult.snapshot[key],
    });
}
console.log(
  JSON.stringify(
    { chromeVersion, baselineFailures, chrome: chromeResult, mimic: mimicResult, differences },
    null,
    2,
  ),
);
if (baselineFailures.length || differences.length) process.exitCode = 1;
