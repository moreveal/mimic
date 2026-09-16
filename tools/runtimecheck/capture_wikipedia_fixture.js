const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const { chromium } = require('playwright-core')

const OUT = path.join(__dirname, 'wikipedia-fixture')

const PAGES = [
  'https://en.wikipedia.org/wiki/Main_Page',
  'https://en.wikipedia.org/wiki/JavaScript',
  'https://en.wikipedia.org/wiki/ECMAScript',
]

function sha256 (value) {
  return crypto.createHash('sha256').update(value).digest('hex')
}

function resourcePath (url) {
  return path.join(OUT, 'resources', sha256(url))
}

;(async () => {
  fs.rmSync(OUT, { recursive: true, force: true })
  fs.mkdirSync(path.join(OUT, 'resources'), { recursive: true })

  if (!process.env.PW_CHROME_EXECUTABLE) {
    console.log('You need to set chromium path with PW_CHROME_EXECUTABLE')
    return
  }

  const browser = await chromium.launch({
    executablePath: process.env.PW_CHROME_EXECUTABLE,
    headless: true,
  })

  const context = await browser.newContext({
    serviceWorkers: 'block',
  })

  const resources = new Map()

  context.on('response', async response => {
    const request = response.request()

    if (request.method() !== 'GET') return

    const url = response.url()

    if (!/^https?:/.test(url)) return

    try {
      const body = await response.body()

      if (!body) return

      const headers = await response.allHeaders()

      resources.set(url, {
        url,
        status: response.status(),
        headers,
        file: path.relative(OUT, resourcePath(url)).replaceAll('\\', '/'),
      })

      fs.writeFileSync(resourcePath(url), body)
    } catch {
      // Some aborted/opaque responses cannot expose a body.
    }
  })

  const pages = {}

  for (const url of PAGES) {
    console.log(`Capturing ${url}`)

    const page = await context.newPage()

    const response = await page.goto(url, {
      waitUntil: 'domcontentloaded',
      timeout: 60_000,
    })

    if (!response || response.status() !== 200) throw new Error(`Navigation failed: ${url}`)

    // Give normal immediately-following resources a bounded opportunity
    // to finish. We deliberately do not use networkidle because Wikipedia
    // can have background traffic.
    await page.waitForTimeout(2000)

    const html = await page.content()

    const key = url.endsWith('/Main_Page')
      ? 'main'
      : url.endsWith('/JavaScript')
      ? 'javascript'
      : 'ecmascript'

    const filename = `pages/${key}.html`

    fs.mkdirSync(path.join(OUT, 'pages'), {
      recursive: true,
    })

    fs.writeFileSync(path.join(OUT, filename), html, 'utf8')

    pages[url] = {
      url,
      file: filename,
      title: await page.title(),
    }

    console.log(`  ${html.length} chars, resources so far: ${resources.size}`)

    await page.close()
  }

  fs.writeFileSync(
    path.join(OUT, 'manifest.json'),
    JSON.stringify(
      {
        capturedAt: new Date().toISOString(),
        pages,
        resources: Object.fromEntries(resources),
      },
      null,
      2,
    ),
  )

  console.log('')
  console.log(`Fixture written to ${OUT}`)
  console.log(`Resources: ${resources.size}`)

  await browser.close()
})().catch(error => {
  console.error(error)
  process.exitCode = 1
})
