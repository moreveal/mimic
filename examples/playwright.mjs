import assert from 'node:assert/strict';
import { chromium } from 'playwright-core';

const browser = await chromium.connectOverCDP(process.env.MIMIC_URL || 'http://127.0.0.1:9222');
try {
  const context = browser.contexts()[0];
  const page = await context.newPage();
  try {
    await page.goto(process.env.TARGET_URL || 'http://127.0.0.1:3000', { waitUntil: 'load' });
    await page.locator('#quantity').fill('3');
    await page.locator('#calculate').click();
    await page.waitForFunction(() => document.querySelector('#total').textContent === '126 USD');
    const result = await page.locator('#total').textContent();
    assert.equal(result, '126 USD');
    console.log(`Playwright: ${result} — input, click, fetch, and DOM update`);
  } finally {
    await page.close();
  }
} finally {
  // Closing a connected Playwright Browser disconnects this client.
  await browser.close();
}
