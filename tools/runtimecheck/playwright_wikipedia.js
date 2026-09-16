const assert = require('node:assert/strict');
const { chromium } = require('playwright-core');

const startedAt = Date.now();
let stepNumber = 0;
function log(message, details) {
  const elapsed = String(Date.now() - startedAt).padStart(6, ' ');
  const suffix = details === undefined ? '' : ` ${JSON.stringify(details)}`;
  console.log(`[+${elapsed}ms] [${String(++stepNumber).padStart(2, '0')}] ${message}${suffix}`);
}

(async () => {
  let browser;
  let currentStage = 'initialization';
  try {
    currentStage = 'connectOverCDP';
    if (process.env.PW_CHROME_EXECUTABLE) {
      log('Launching control Chrome', {
        executablePath: process.env.PW_CHROME_EXECUTABLE,
      });
      browser = await chromium.launch({
        executablePath: process.env.PW_CHROME_EXECUTABLE,
        headless: true,
      });
      log('Chrome launched');
    } else {
      const endpoint = process.env.PW_MIMIC_ENDPOINT || 'http://127.0.0.1:9222';
      log('Connecting Playwright to Mimic', {
        endpoint,
      });
      browser = await chromium.connectOverCDP(endpoint);
      log('CDP connection established');
    }

    const context = browser.contexts()[0] ?? (await browser.newContext());
    assert.ok(context, 'A default browser context must exist');
    const page = await context.newPage();
    page.setDefaultTimeout(15_000);
    page.setDefaultNavigationTimeout(30_000);
    log('New page created');

    page.on('console', (message) =>
      log('Browser console', { type: message.type(), text: message.text() }),
    );
    page.on('requestfailed', (request) =>
      log('Request failed', {
        url: request.url(),
        error: request.failure()?.errorText,
      }),
    );

    currentStage = 'open Wikipedia main page';
    log('Navigating to Wikipedia', { url: 'https://en.wikipedia.org/wiki/Main_Page' });
    const mainResponse = await page.goto('https://en.wikipedia.org/wiki/Main_Page', {
      waitUntil: 'domcontentloaded',
    });
    assert.ok(mainResponse, 'Main navigation must return a response');
    assert.equal(mainResponse.status(), 200);
    await page.locator('#firstHeading').waitFor({ state: 'attached' });
    log('Wikipedia main page loaded', {
      status: mainResponse.status(),
      title: await page.title(),
      heading: (await page.locator('#firstHeading').innerText()).trim(),
    });

    currentStage = 'search for JavaScript';
    const searchInput = page.locator('input[name="search"]').first();
    await searchInput.fill('JavaScript');
    assert.equal(await searchInput.inputValue(), 'JavaScript');
    log('Search query entered', { value: await searchInput.inputValue() });

    await Promise.all([
      page.waitForURL(/\/wiki\/JavaScript(?:$|[#?])/),
      searchInput.press('Enter'),
    ]);
    await page.locator('#firstHeading').waitFor({ state: 'visible' });
    log('Search navigation completed', {
      url: page.url(),
      title: await page.title(),
      heading: (await page.locator('#firstHeading').innerText()).trim(),
    });

    currentStage = 'inspect JavaScript article';
    assert.equal((await page.locator('#firstHeading').innerText()).trim(), 'JavaScript');
    const firstParagraph = page
      .locator('#mw-content-text p')
      .filter({ hasText: /programming language/i })
      .first();
    await firstParagraph.waitFor({ state: 'visible' });
    const paragraphText = (await firstParagraph.innerText()).replace(/\s+/g, ' ').trim();
    assert.match(paragraphText, /programming language/i);
    log('Article DOM inspected', {
      paragraphPreview: paragraphText.slice(0, 180),
      bodyTextLength: (await page.locator('body').innerText()).length,
    });

    currentStage = 'follow ECMAScript link';
    const ecmaScriptLink = page.getByRole('link', { name: 'ECMAScript', exact: true }).first();
    await ecmaScriptLink.scrollIntoViewIfNeeded();
    log('Internal article link located', {
      text: (await ecmaScriptLink.innerText()).trim(),
      href: await ecmaScriptLink.getAttribute('href'),
    });
    await Promise.all([page.waitForURL(/\/wiki\/ECMAScript(?:$|[#?])/), ecmaScriptLink.click()]);
    await page.locator('#firstHeading').waitFor({ state: 'visible' });
    assert.equal((await page.locator('#firstHeading').innerText()).trim(), 'ECMAScript');
    log('Internal link navigation completed', {
      url: page.url(),
      title: await page.title(),
      heading: (await page.locator('#firstHeading').innerText()).trim(),
    });

    currentStage = 'browser back navigation';
    await page.goBack({ waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/wiki\/JavaScript(?:$|[#?])/);
    assert.equal((await page.locator('#firstHeading').innerText()).trim(), 'JavaScript');
    log('Back navigation restored JavaScript article', {
      url: page.url(),
      heading: (await page.locator('#firstHeading').innerText()).trim(),
    });

    const cdp = await context.newCDPSession(page);
    const diagnostics = await cdp.send("Mimic.getDiagnostics");

    require("node:fs").writeFileSync(
      "mimic-hosts.json",
      JSON.stringify(diagnostics, null, 2)
    );

    console.log("Diagnostics written to mimic-hosts.json");

    currentStage = 'close page';
    await page.close();
    log('Page closed normally');
    console.log(`RESULT: PASS (${Date.now() - startedAt} ms)`);
  } catch (error) {
    console.error(`RESULT: FAIL at stage: ${currentStage}`);
    console.error(error && error.stack ? error.stack : error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close().catch(() => {});
  }
})();
