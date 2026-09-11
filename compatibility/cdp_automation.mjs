// Real pinned Puppeteer client. Python supplies the same fixture and wire proxy
// used by Pyppeteer; no client protocol behavior is mocked or patched.
import {readFile, writeFile} from 'node:fs/promises';
import {dirname, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {isDeepStrictEqual} from 'node:util';

const args = Object.fromEntries(process.argv.slice(2).reduce((pairs, value, index, values) => {
  if (value.startsWith('--')) pairs.push([value.slice(2), values[index + 1]]);
  return pairs;
}, []));
const {default: puppeteer} = await import(pathToFileURL(args.module));
const {version} = JSON.parse(await readFile(resolve(dirname(args.module), '../../package.json'), 'utf8'));
if (version !== '25.10.0') throw new Error(`Expected puppeteer-core 25.10.0, got ${version}`);

const timeoutMs = 8000;
const report = {client: 'puppeteer', version, checks: []};
const base = args['base-url'];
const pages = [];
const contexts = [];
let browser;

async function timeout(promise, ms = timeoutMs) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(`Operation timeout after ${ms}ms`)), ms);
    })]);
  } finally {
    clearTimeout(timer);
  }
}
async function save() {
  await writeFile(args.output, JSON.stringify(report, null, 2) + '\n');
}
async function check(name, operation, expected) {
  const start = performance.now();
  let row;
  try {
    const observed = await timeout(operation());
    row = expected !== undefined && !isDeepStrictEqual(observed, expected)
      ? {status: 'fail', observed, expected}
      : {status: 'pass', observed};
  } catch (error) {
    row = {status: 'fail', error: String(error)};
  }
  report.checks.push({name, ...row, durationMs: Math.round(performance.now() - start)});
  await save();
  return row.status === 'pass';
}
async function newPage() {
  const page = await browser.newPage();
  page.setDefaultTimeout(6400);
  page.setDefaultNavigationTimeout(6400);
  pages.push(page);
  return page;
}

async function run() {
  if (!await check('browser.connect', async () => {
    browser = await puppeteer.connect({browserWSEndpoint: args.endpoint, defaultViewport: null, protocolTimeout: 6400});
    return true;
  }, true)) {
    report.blocked = 'browser.connect';
    return;
  }
  await check('browser.version', async () => (await browser.version()).endsWith('/152.0.7977.82'), true);
  let page;
  if (!await check('page.create', async () => {
    page = await newPage();
    return page.url();
  }, 'about:blank')) {
    report.blocked = 'page.create';
    return;
  }
  async function navigate(path = 'a', waitUntil = 'load') {
    const response = await page.goto(base + '/page/' + path, {waitUntil});
    return {status: response.status(), title: await page.title()};
  }
  await check('navigation.goto', () => navigate(), {status: 200, title: 'Automation a'});
  await check('runtime.by_value', () => page.evaluate(() => ({number: 42, nested: [true, null, {text: 'value'}]})),
    {number: 42, nested: [true, null, {text: 'value'}]});
  await check('runtime.promise', () => page.evaluate(() => new Promise(resolve => setTimeout(() => resolve(42), 20))), 42);
  await check('runtime.handles_properties', async () => {
    const handle = await page.evaluateHandle(() => ({a: 7, nested: {label: 'nested'}}));
    const props = await handle.getProperties();
    const nested = await handle.getProperty('nested');
    try {
      return {keys: [...props.keys()].sort(), a: await props.get('a').jsonValue(), nested: await nested.jsonValue()};
    } finally {
      await Promise.all([...props.values(), nested, handle].map(value => value.dispose()));
    }
  }, {keys: ['a', 'nested'], a: 7, nested: {label: 'nested'}});
  await check('runtime.handle_identity', async () => {
    const first = await page.evaluateHandle(() => globalThis.identityObject = {x: 1});
    const second = await page.evaluateHandle(() => globalThis.identityObject);
    try { return await page.evaluate((a, b) => a === b, first, second); }
    finally { await first.dispose(); await second.dispose(); }
  }, true);
  await check('runtime.handle_dispose', async () => {
    const handle = await page.evaluateHandle(() => ({x: 1}));
    await handle.dispose();
    try { await page.evaluate(value => value.x, handle); return false; }
    catch (error) { return String(error).toLowerCase().includes('disposed'); }
  }, true);
  await check('runtime.exception', async () => {
    try { await page.evaluate(() => { throw new TypeError('automation failure'); }); return false; }
    catch (error) { return String(error).includes('automation failure') && String(error).includes('TypeError'); }
  }, true);
  await check('runtime.unserializable_arguments', async () => {
    const handles = await Promise.all(['NaN', 'Infinity', '-0'].map(value => page.evaluateHandle(value)));
    try {
      return await page.evaluate((a, b, c) => [Number.isNaN(a), b === Infinity, Object.is(c, -0)], ...handles);
    } finally { await Promise.all(handles.map(handle => handle.dispose())); }
  }, [true, true, true]);
  await check('dom.selectors_element_evaluation', async () => {
    const element = await page.$('#heading');
    try {
      return {heading: await element.evaluate(el => el.textContent),
        items: await page.$$eval('.item', els => els.map(el => el.textContent))};
    } finally { await element.dispose(); }
  }, {heading: 'Fixture a', items: ['first', 'second']});
  await check('dom.form_input_select_dom_click', async () => {
    await page.type('#name', 'Alpha 42');
    const selected = await page.select('#choice', 'b');
    await page.$eval('#button', el => el.click());
    return {value: await page.$eval('#name', el => el.value), selected, clicked: await page.evaluate(() => window.clicked)};
  }, {value: 'Alpha 42', selected: ['b'], clicked: 1});
  await check('input.pointer_click', async () => {
    await page.click('#button');
    return await page.evaluate(() => window.clicked);
  }, 2);
  await check('runtime.console_arguments', async () => {
    const messages = [];
    const listener = message => messages.push(message);
    page.on('console', listener);
    try {
      await page.evaluate(() => console.log('automation-console', {answer: 42}));
      for (let i = 0; i < 20 && !messages.length; i++) await new Promise(resolve => setTimeout(resolve, 10));
      const message = messages.find(m => m.text().includes('automation-console'));
      return {type: message.type(), arguments: await Promise.all(message.args().map(a => a.jsonValue()))};
    } finally { page.off('console', listener); }
  }, {type: 'log', arguments: ['automation-console', {answer: 42}]});
  await check('runtime.pageerror', async () => {
    const errors = [];
    const listener = error => errors.push(error);
    page.on('pageerror', listener);
    try {
      await page.evaluate(() => { setTimeout(() => { throw new TypeError('automation-pageerror'); }, 30); });
      for (let i = 0; i < 100 && !errors.length; i++) await new Promise(resolve => setTimeout(resolve, 20));
      if (!errors.length) throw new Error('No pageerror event for uncaught timer TypeError');
      return String(errors[0]).split('\n')[0];
    } finally { page.off('pageerror', listener); }
  }, 'Error: Uncaught TypeError: automation-pageerror');
  await check('page.init_script', async () => {
    await page.evaluateOnNewDocument(() => { window.automationInit = 'installed'; });
    await navigate('b');
    return await page.evaluate(() => window.fixtureInit);
  }, 'installed');
  await check('navigation.reload', async () => {
    await page.evaluate(() => window.transientValue = 1);
    const response = await page.reload({waitUntil: 'load'});
    return {status: response.status(), value: await page.evaluate(() => [typeof window.transientValue, window.fixtureInit])};
  }, {status: 200, value: ['undefined', 'installed']});
  await check('navigation.history', async () => {
    await navigate('a');
    await navigate('b');
    await page.goBack({waitUntil: 'load'});
    const back = await page.title();
    await page.goForward({waitUntil: 'load'});
    return [back, await page.title()];
  }, ['Automation a', 'Automation b']);
  await check('page.child_frame_events_evaluation', async () => {
    const events = [];
    page.on('frameattached', () => events.push('attached'));
    page.on('framenavigated', () => events.push('navigated'));
    await page.evaluate(url => {
      const frame = document.createElement('iframe');
      frame.name = 'child'; frame.src = url; document.body.append(frame);
    }, base + '/page/child');
    let child;
    for (let i = 0; i < 100; i++) {
      child = page.frames().find(f => f.name() === 'child' && f.url().endsWith('/child'));
      if (child) break;
      await new Promise(resolve => setTimeout(resolve, 20));
    }
    if (!child) throw new Error('child frame not discovered');
    return {title: await child.evaluate(() => document.title), parent: child.parentFrame() === page.mainFrame(),
      attached: events.includes('attached'), navigated: events.includes('navigated')};
  }, {title: 'Automation child', parent: true, attached: true, navigated: true});
  await check('network.cookies', async () => {
    await page.setCookie({name: 'automation', value: 'cookie', url: base});
    const values = await page.cookies(base);
    const visible = await page.evaluate(() => document.cookie);
    await page.deleteCookie({name: 'automation', url: base});
    return {cookie: values.find(c => c.name === 'automation').value, visible: visible.includes('automation=cookie'),
      removed: !(await page.evaluate(() => document.cookie)).includes('automation=cookie')};
  }, {cookie: 'cookie', visible: true, removed: true});
  await check('network.response_body_extra_headers', async () => {
    await page.setExtraHTTPHeaders({'x-automation': 'header'});
    const response = await page.goto(base + '/api/body');
    const value = await response.json();
    return {status: response.status(), header: value.header, path: value.path};
  }, {status: 200, header: 'header', path: '/api/body'});
  await check('network.interception_fulfill_body', async () => {
    const tasks = [];
    const errors = [];
    const listener = request => {
      const task = (request.url().endsWith('/api/intercept')
        ? request.respond({status: 201, contentType: 'application/json', body: '{"intercepted":true}'})
        : request.continue()).catch(error => errors.push(String(error)));
      tasks.push(task);
      return task;
    };
    await page.setRequestInterception(true);
    page.on('request', listener);
    try {
      const response = await page.goto(base + '/api/intercept');
      const result = {status: response.status(), body: await response.json()};
      await Promise.all(tasks);
      if (errors.length) throw new Error(errors.join('; '));
      return result;
    } finally {
      page.off('request', listener);
      await page.setRequestInterception(false);
      await Promise.allSettled(tasks);
    }
  }, {status: 201, body: {intercepted: true}});
  await check('navigation.networkidle0', () => navigate('a', 'networkidle0'), {status: 200, title: 'Automation a'});
  await check('page.set_content', async () => {
    await page.setContent("<!doctype html><title>Automation content</title><main id='content'>inserted</main>");
    return {title: await page.title(), text: await page.$eval('#content', el => el.textContent)};
  }, {title: 'Automation content', text: 'inserted'});
  await check('dom.wait_for_selector_mutation', async () => {
    await page.evaluate(() => setTimeout(() => {
      const element = document.createElement('p'); element.id = 'delayed';
      element.textContent = 'arrived'; document.body.append(element);
    }, 30));
    const element = await page.waitForSelector('#delayed', {timeout: 3000});
    try { return await element.evaluate(el => el.textContent); }
    finally { await element.dispose(); }
  }, 'arrived');
  await check('runtime.exposed_function_navigation', async () => {
    await page.exposeFunction('automationAdd', (a, b) => a + b);
    const first = await page.evaluate(async () => await window.automationAdd(8, 13));
    await navigate('exposed');
    return [first, await page.evaluate(async () => await window.automationAdd(4, 5))];
  }, [21, 9]);
  await check('page.init_script_removal', async () => {
    const script = await page.evaluateOnNewDocument(() => window.removableInit = true);
    await navigate('init-before');
    const before = await page.evaluate(() => window.removableInit);
    await page.removeScriptToEvaluateOnNewDocument(script.identifier);
    await navigate('init-after');
    return [before, await page.evaluate(() => typeof window.removableInit)];
  }, [true, 'undefined']);
  await check('emulation.viewport', async () => {
    await page.setViewport({width: 900, height: 620, deviceScaleFactor: 1.5});
    return await page.evaluate(() => [innerWidth, innerHeight, devicePixelRatio]);
  }, [900, 620, 1.5]);
  await check('network.user_agent_override', async () => {
    await page.setUserAgent('AutomationClient/1.0');
    const response = await page.goto(base + '/api/user-agent');
    return [await page.evaluate(() => navigator.userAgent), (await response.json()).userAgent];
  }, ['AutomationClient/1.0', 'AutomationClient/1.0']);
  await check('network.interception_continue_post', async () => {
    const tasks = [], errors = [];
    const listener = request => {
      const task = (request.url().endsWith('/api/continue')
        ? request.continue({method: 'POST', postData: 'posted=42'})
        : request.continue()).catch(error => errors.push(String(error)));
      tasks.push(task); return task;
    };
    await page.setRequestInterception(true);
    page.on('request', listener);
    try {
      const response = await page.goto(base + '/api/continue');
      const value = await response.json();
      await Promise.all(tasks);
      if (errors.length) throw new Error(errors.join('; '));
      return {method: value.method, body: value.body};
    } finally {
      page.off('request', listener);
      await page.setRequestInterception(false);
      await Promise.allSettled(tasks);
    }
  }, {method: 'POST', body: 'posted=42'});
  await check('network.interception_abort', async () => {
    const tasks = [], errors = [], failed = [];
    const listener = request => {
      const task = (request.url().endsWith('/api/abort') ? request.abort('failed') : request.continue())
        .catch(error => errors.push(String(error)));
      tasks.push(task); return task;
    };
    const failure = request => failed.push(request);
    await page.setRequestInterception(true);
    page.on('request', listener);
    page.on('requestfailed', failure);
    let rejected = false;
    try {
      try { await page.goto(base + '/api/abort'); } catch { rejected = true; }
      await Promise.all(tasks);
      if (errors.length) throw new Error(errors.join('; '));
      return {rejected, requestFailed: failed.some(request => request.url().endsWith('/api/abort'))};
    } finally {
      page.off('request', listener);
      page.off('requestfailed', failure);
      await page.setRequestInterception(false);
      await Promise.allSettled(tasks);
    }
  }, {rejected: true, requestFailed: true});
  await check('target.parallel_pages_isolation', async () => {
    const [first, second] = await Promise.all([newPage(), newPage()]);
    await Promise.all([first.goto(base + '/page/first'), second.goto(base + '/page/second')]);
    await Promise.all([first.evaluate(() => window.pageIdentity = 'first'), second.evaluate(() => window.pageIdentity = 'second')]);
    return await Promise.all([first.evaluate(() => [document.title, window.pageIdentity]),
      second.evaluate(() => [document.title, window.pageIdentity])]);
  }, [['Automation first', 'first'], ['Automation second', 'second']]);
  await check('target.browser_context_storage_isolation', async () => {
    const [first, second] = await Promise.all([browser.createBrowserContext(), browser.createBrowserContext()]);
    contexts.push(first, second);
    const [a, b] = await Promise.all([first.newPage(), second.newPage()]);
    await Promise.all([a.goto(base + '/page/a'), b.goto(base + '/page/b')]);
    await a.evaluate(() => { document.cookie = 'private=first; path=/'; localStorage.setItem('private', 'first'); });
    return [await a.evaluate(() => [document.cookie, localStorage.getItem('private')]),
      await b.evaluate(() => [document.cookie, localStorage.getItem('private')])];
  }, [['private=first', 'first'], ['', null]]);
}

try {
  await run();
} catch (error) {
  report.workerFailure = String(error);
} finally {
  for (const context of contexts) { try { await timeout(context.close(), 2000); } catch {} }
  for (const page of pages) { try { await timeout(page.close(), 2000); } catch {} }
  if (browser) { try { await timeout(browser.disconnect(), 2000); } catch {} }
  await save();
}
