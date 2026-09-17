import assert from 'node:assert/strict';
import puppeteer from 'puppeteer-core';

const browser = await puppeteer.connect({
  browserURL: process.env.MIMIC_URL || 'http://127.0.0.1:9222',
  defaultViewport: null,
});
try {
  const results = await Promise.all(
    Array.from({ length: 10 }, async (_, index) => {
      const page = await browser.newPage();
      try {
        await page.goto(process.env.TARGET_URL || 'http://127.0.0.1:3000', { waitUntil: 'load' });
        return await page.evaluate(async (value) => {
          window.jobId = value;
          const quote = await fetch('/api/quote').then((response) => response.json());
          await Promise.resolve();
          return { jobId: window.jobId, price: quote.price };
        }, index);
      } finally {
        await page.close();
      }
    }),
  );
  assert.deepEqual(
    results,
    Array.from({ length: 10 }, (_, jobId) => ({ jobId, price: 42 })),
  );
  console.log('Concurrency: 10/10 pages, separate window state, fetch results collected');
} finally {
  await browser.disconnect();
}
