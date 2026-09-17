import assert from 'node:assert/strict';
import puppeteer from 'puppeteer-core';

const browser = await puppeteer.connect({
  browserURL: process.env.MIMIC_URL || 'http://127.0.0.1:9222',
  defaultViewport: null,
});
try {
  const page = await browser.newPage();
  try {
    await page.goto(process.env.TARGET_URL || 'http://127.0.0.1:3000', { waitUntil: 'load' });
    await page.waitForSelector('h1');
    const result = await page.evaluate(() => ({
      heading: document.querySelector('h1').textContent,
      language: navigator.language,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      viewport: [innerWidth, innerHeight],
      darkMode: matchMedia('(prefers-color-scheme: dark)').matches,
    }));
    assert.deepEqual(result, {
      heading: 'Demo shop',
      language: 'en-US',
      timezone: 'America/New_York',
      viewport: [1280, 720],
      darkMode: true,
    });
    console.log('Puppeteer profile:', JSON.stringify(result));
  } finally {
    await page.close();
  }
} finally {
  await browser.disconnect();
}
