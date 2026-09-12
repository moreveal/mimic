// Run with: node compatibility/playwright_cli.cjs <cli.js> <cdp-url> <session>
const http = require('node:http');
const { spawn } = require('node:child_process');
const [cli, endpoint, session] = process.argv.slice(2);
if (!session) throw new Error('Expected CLI entry point, CDP URL and unique session name');
const server = http.createServer((req, res) => {
  res.setHeader('Content-Type', 'text/html');
  res.end('<!doctype html><title>Automation fixture</title><h1>Fixture</h1><svg><title>Logo label</title></svg><label>Name<input id="name"></label><button onclick="document.querySelector(\'h1\').textContent=document.querySelector(\'input\').value">Apply</button>');
});
async function run(...args) {
  const result = await new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [cli, `-s=${session}`, ...args], { windowsHide: true });
    let output = '';
    child.stdout.on('data', data => output += data);
    child.stderr.on('data', data => output += data);
    const timer = setTimeout(() => { child.kill(); reject(new Error(`Timed out: ${args[0]}`)); }, 45000);
    child.on('error', reject);
    child.on('close', code => { clearTimeout(timer); resolve({ code, output }); });
  });
  console.log(JSON.stringify({ command: args[0], ...result }));
  if (result.code || result.output.includes('### Error')) throw new Error(`Failed: ${args[0]}`);
  return result.output;
}
(async () => {
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const url = `http://127.0.0.1:${server.address().port}/`;
  try {
    await run('attach', `--cdp=${endpoint}`);
    await run('goto', url);
    const value = await run('eval', '() => document.title');
    if (!value.includes('Automation fixture')) throw new Error('Title mismatch');
    await run('run-code', `async page => { const cdp=await page.context().newCDPSession(page);await cdp.send('Emulation.setFocusEmulationEnabled',{enabled:true});await page.evaluate(() => {
      const style=document.createElement('style');style.textContent='#probe {width:20px} #probe:checked {width:40px} #probe:focus {height:30px}';document.head.append(style);
      const probe=document.createElement('input');probe.id='probe';probe.type='checkbox';document.body.append(probe);
      const width=()=>getComputedStyle(probe).width;
      if(width()!=='20px')throw new Error('Initial style');
      probe.checked=true;if(width()!=='40px')throw new Error('Checked invalidation');
      probe.checked=false;if(width()!=='20px')throw new Error('Unchecked invalidation');
      probe.style.width='60px';if(width()!=='60px')throw new Error('Inline style invalidation');
      probe.focus();if(getComputedStyle(probe).height!=='30px')throw new Error('Focus invalidation');
      probe.style.display='none';if(probe.checkVisibility())throw new Error('Visibility invalidation');
      probe.remove();style.remove();
    });await cdp.detach(); }`);
    await run('run-code', 'async page => { await page.getByRole("textbox", {name:"Name"}).fill("Verified"); await page.getByRole("button", {name:"Apply"}).click(); if (await page.locator("h1").textContent() !== "Verified") throw new Error("Input/click mismatch"); }');
    const snapshot = await run('snapshot', '--raw');
    if (!snapshot.includes('Verified')) throw new Error('Snapshot mismatch');
    await run('reload');
    await run('tab-new', url);
    await run('tab-list');
    await run('tab-close');
    await run('detach');
    await run('attach', `--cdp=${endpoint}`);
    await run('snapshot');
  } finally {
    await run('detach').catch(console.error);
    server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
