const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const http = require('node:http')
const { chromium } = require('playwright-core')

const FIXTURE = path.join(__dirname, 'wikipedia-fixture')

const manifest = JSON.parse(fs.readFileSync(path.join(FIXTURE, 'manifest.json'), 'utf8'))

const startedAt = Date.now()
let stepNumber = 0

function log (message, details) {
  const elapsed = String(Date.now() - startedAt).padStart(6, ' ')
  const suffix = details === undefined ? '' : ` ${JSON.stringify(details)}`

  console.log(
    `[+${elapsed}ms] ` + `[${String(++stepNumber).padStart(2, '0')}] ` + `${message}${suffix}`,
  )
}

const PAGE_MAP = {
  '/wiki/Main_Page': 'pages/main.html',
  '/wiki/JavaScript': 'pages/javascript.html',
  '/wiki/ECMAScript': 'pages/ecmascript.html',
}

function contentType (headers) {
  return headers?.['content-type'] || headers?.['Content-Type'] || 'application/octet-stream'
}

async function startServer () {
  const server = http.createServer((req, res) => {
    const requestURL = new URL(req.url, 'http://127.0.0.1')

    const pageFile = PAGE_MAP[requestURL.pathname]

    if (pageFile) {
      const body = fs.readFileSync(path.join(FIXTURE, pageFile))

      res.writeHead(200, {
        'content-type': 'text/html; charset=utf-8',
        'cache-control': 'no-store',
      })

      res.end(body)
      return
    }

    res.writeHead(404, {
      'content-type': 'text/plain',
    })

    res.end('fixture resource not found')
  })

  await new Promise((resolve, reject) => {
    server.once('error', reject)

    server.listen(0, '127.0.0.1', () => {
      server.off('error', reject)
      resolve()
    })
  })

  const address = server.address()

  return {
    server,
    origin: `http://127.0.0.1:${address.port}`,
  }
}

async function measure (name, fn) {
  const started = performance.now()

  try {
    const result = await fn()
    const ms = performance.now() - started

    console.log(`[MEASURE] ${name}: ${ms.toFixed(2)} ms`)
    return result
  } catch (error) {
    const ms = performance.now() - started
    console.log(`[MEASURE] ${name}: FAIL after ${ms.toFixed(2)} ms`)
    throw error
  }
}

;(async () => {
  let browser
  let server

  let currentStage = 'initialization'

  try {
    const local = await startServer()

    server = local.server
    const origin = local.origin

    log('Fixture server started', { origin })

    if (process.env.PW_CHROME_EXECUTABLE) {
      log('Launching control Chrome', {
        executablePath: process.env.PW_CHROME_EXECUTABLE,
      })

      browser = await chromium.launch({
        executablePath: process.env.PW_CHROME_EXECUTABLE,
        headless: true,
      })

      log('Chrome launched')
    } else {
      const endpoint = process.env.PW_MIMIC_ENDPOINT || 'http://127.0.0.1:9222'

      log('Connecting Playwright to Mimic', {
        endpoint,
      })

      browser = await chromium.connectOverCDP(endpoint)

      log('CDP connection established')
    }

    const context = browser.contexts()[0] ?? (await browser.newContext())

    assert.ok(context)

    /*
     * Frozen network:
     *
     * Every HTTP(S) request to Wikipedia/Wikimedia is served
     * from the captured manifest.
     *
     * Navigation URLs are translated to localhost pages.
     */
    await context.route(/^https?:\/\//, async route => {
      const request = route.request()
      const url = request.url()

      if (request.method() !== 'GET') {
        await route.abort()
        return
      }

      let parsed

      try {
        parsed = new URL(url)
      } catch {
        await route.abort()
        return
      }

      /*
       * Page navigations.
       */
      if (parsed.hostname === 'en.wikipedia.org' && PAGE_MAP[parsed.pathname]) {
        const filename = PAGE_MAP[parsed.pathname]

        await route.fulfill({
          status: 200,
          contentType: 'text/html; charset=utf-8',
          body: fs.readFileSync(path.join(FIXTURE, filename)),
        })

        return
      }

      /*
       * Frozen Wikipedia search.
       *
       * The real search form navigates through /w/index.php?... .
       * Reproduce the deterministic server-side result with a redirect
       * to the captured JavaScript article.
       */
      if (
        parsed.hostname === 'en.wikipedia.org' &&
        parsed.pathname === '/w/index.php' &&
        parsed.searchParams.get('search') === 'JavaScript'
      ) {
        await route.fulfill({
          status: 302,
          headers: {
            location: 'https://en.wikipedia.org/wiki/JavaScript',
            'cache-control': 'no-store',
          },
          body: '',
        })

        return
      }

      /*
       * Captured resources.
       */
      const entry = manifest.resources[url]

      if (entry) {
        const headers = {
          ...entry.headers,
        }

        /*
         * These headers describe the original network
         * transport and should not be replayed.
         */
        delete headers['content-length']
        delete headers['content-encoding']
        delete headers['transfer-encoding']
        delete headers['connection']

        await route.fulfill({
          status: entry.status,
          headers: {
            ...headers,
            'content-type': contentType(entry.headers),
          },
          body: fs.readFileSync(path.join(FIXTURE, entry.file)),
        })

        return
      }

      /*
       * Absolutely no live network during benchmark.
       */
      await route.abort()
    })

    const page = await context.newPage()
    const cdp = await context.newCDPSession(page)

    page.setDefaultTimeout(15_000)
    page.setDefaultNavigationTimeout(30_000)

    log('New page created')

    page.on('requestfailed', request =>
      log('Request failed', {
        url: request.url(),
        error: request.failure()?.errorText,
      }),
    )

    /*
     * MAIN PAGE
     */

    currentStage = 'open Wikipedia main page'

    const mainURL = 'https://en.wikipedia.org/wiki/Main_Page'

    log('Navigating to Wikipedia fixture', {
      url: mainURL,
    })

    const mainResponse = await page.goto(mainURL, {
      waitUntil: 'domcontentloaded',
    })

    assert.ok(mainResponse)
    assert.equal(mainResponse.status(), 200)

    await page.locator('#firstHeading').waitFor({ state: 'attached' })

    log('Wikipedia main page loaded', {
      status: mainResponse.status(),
      title: await page.title(),
      heading: (await page.locator('#firstHeading').innerText()).trim(),
    })

    /*
     * SEARCH
     */

    currentStage = 'search for JavaScript'

    const searchInput = page.locator('input[name="search"]').first()

    await measure('Search fill', () => searchInput.fill('JavaScript'))

    const searchValue = await measure('Search inputValue', () => searchInput.inputValue())

    assert.equal(searchValue, 'JavaScript')

    await measure('Search press + navigation', () =>
      Promise.all([page.waitForURL(/\/wiki\/JavaScript(?:$|[#?])/), searchInput.press('Enter')]),
    )

    const traceBeforeVisible = await cdp.send('Mimic.getTrace')

    await measure('JavaScript heading visible', () =>
      page.locator('#firstHeading').waitFor({ state: 'visible' }),
    )

    const traceAfterVisible = await cdp.send('Mimic.getTrace')

    require('node:fs').writeFileSync(
      'heading-visible-trace.json',
      JSON.stringify(
        {
          beforeCount: traceBeforeVisible.events.length,
          events: traceAfterVisible.events.slice(traceBeforeVisible.events.length),
        },
        null,
        2,
      ),
    )

    /*
     * ARTICLE INSPECTION
     */

    currentStage = 'inspect JavaScript article'

    assert.equal((await page.locator('#firstHeading').innerText()).trim(), 'JavaScript')

    const firstParagraph = page
      .locator('#mw-content-text p')
      .filter({
        hasText: /programming language/i,
      })
      .first()

    await firstParagraph.waitFor({
      state: 'visible',
    })

    const paragraphText = (await firstParagraph.innerText()).replace(/\s+/g, ' ').trim()

    assert.match(paragraphText, /programming language/i)

    log('Article DOM inspected', {
      paragraphPreview: paragraphText.slice(0, 180),

      bodyTextLength: (await page.locator('body').innerText()).length,
    })

    /*
     * ECMASCRIPT LINK
     */

    currentStage = 'follow ECMAScript link'

    const ecmaScriptLink = page.getByRole('link', { name: 'ECMAScript', exact: true }).first()

    await measure('ECMAScript scrollIntoViewIfNeeded', () =>
      ecmaScriptLink.scrollIntoViewIfNeeded(),
    )

    const ecmaText = await measure('ECMAScript innerText', () => ecmaScriptLink.innerText())

    const ecmaHref = await measure('ECMAScript getAttribute href', () =>
      ecmaScriptLink.getAttribute('href'),
    )

    log('Internal article link located', {
      text: ecmaText.trim(),
      href: ecmaHref,
    })

    await measure('ECMAScript click + navigation', () =>
      Promise.all([page.waitForURL(/\/wiki\/ECMAScript(?:$|[#?])/), ecmaScriptLink.click()]),
    )

    await measure('ECMAScript heading visible', () =>
      page.locator('#firstHeading').waitFor({ state: 'visible' }),
    )

    const ecmaHeading = await measure('ECMAScript heading innerText', () =>
      page.locator('#firstHeading').innerText(),
    )

    assert.equal(ecmaHeading.trim(), 'ECMAScript')

    const ecmaTitle = await measure('ECMAScript page.title', () => page.title())

    log('Internal link navigation completed', {
      url: page.url(),
      title: ecmaTitle,
      heading: ecmaHeading.trim(),
    })

    /*
     * BACK
     */

    currentStage = 'browser back navigation'

    await page.goBack({
      waitUntil: 'domcontentloaded',
    })

    await page.waitForURL(/\/wiki\/JavaScript(?:$|[#?])/)

    assert.equal((await page.locator('#firstHeading').innerText()).trim(), 'JavaScript')

    log('Back navigation restored JavaScript article', {
      url: page.url(),
      heading: (await page.locator('#firstHeading').innerText()).trim(),
    })

    currentStage = 'close page'

    await page.close()

    log('Page closed normally')

    console.log(`RESULT: PASS (${Date.now() - startedAt} ms)`)
  } catch (error) {
    console.error(`RESULT: FAIL at stage: ${currentStage}`)

    console.error(error?.stack || error)

    process.exitCode = 1
  } finally {
    if (browser) await browser.close().catch(() => {})

    if (server) await new Promise(resolve => server.close(resolve))
  }
})()
