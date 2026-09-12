"""Independent observational live-site comparison; never edits frozen benchmarks.

Uses fresh browser processes/profiles, native identities, no challenge bypass,
and the same CDP observations. Windows/WSL timing is deliberately end-to-end.
"""
import argparse
import asyncio
import contextlib
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time
from datetime import datetime, timezone

import aiohttp
import psutil

ROOT = Path(__file__).resolve().parents[2]
LP = '/home/moreveal/.local/bin/lightpanda'
MIMIC = ROOT / '.build/mimic-live-comparison-20260912.exe'
CHROME = ROOT / 'compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe'
HARNESS_SOURCE = Path(__file__).read_bytes()
HARNESS_HASH = hashlib.sha256(HARNESS_SOURCE).hexdigest()
SITES = [
    ('example', 'https://example.com/', 'h1', 80),
    ('hackernews', 'https://news.ycombinator.com/', '.titleline a', 300),
    ('wikipedia', 'https://en.wikipedia.org/wiki/Web_browser', '#firstHeading', 1000),
    ('github', 'https://github.com/lightpanda-io/browser', 'article', 1000),
    ('react', 'https://react.dev/learn', 'h1', 1000),
    ('amiibo', 'https://demo-browser.lightpanda.io/amiibo/', 'a', 100),
    ('lowendtalk', 'https://lowendtalk.com/', '.Discussion', 300),
    ('lowendbox', 'https://lowendbox.com/', 'body', 1000),
    ('spigotmc', 'https://www.spigotmc.org/', 'body', 1000),
    ('modrinth', 'https://modrinth.com/mods', 'a[href*="/mod/"]', 300),
    ('mangadex', 'https://mangadex.org/', 'a[href*="/title/"]', 200),
    ('browserscan', 'https://www.browserscan.net/bot-detection', 'body', 200),
    ('iroshop', 'https://iroshop.tech/mimic-e2e', 'body', 200),
    ('iroshop-home', 'https://iroshop.tech/', 'body', 200),
    ('todomvc', 'https://demo.playwright.dev/todomvc/', '.new-todo', 30),
    ('cf-lab', 'https://www.scrapingcourse.com/cloudflare-challenge', 'body', 40),
    ('nowsecure', 'https://nowsecure.nl/', 'body', 40),
]

def capture(cmd):
    return subprocess.check_output(cmd, cwd=ROOT, text=True, encoding='utf-8', errors='replace').strip()

def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding='utf-8')

class CDP:
    def __init__(self, ws):
        self.ws, self.pending, self.events, self.seq = ws, {}, [], 0
        self.reader = asyncio.create_task(self.read())
        self.origin = time.perf_counter()

    async def read(self):
        try:
            async for msg in self.ws:
                if msg.type != aiohttp.WSMsgType.TEXT:
                    continue
                obj = json.loads(msg.data)
                if 'id' in obj:
                    future = self.pending.pop(obj['id'], None)
                    if future and not future.done():
                        future.set_result(obj)
                else:
                    self.events.append({'elapsed_ms': (time.perf_counter()-self.origin)*1000, **obj})
        finally:
            for f in self.pending.values():
                if not f.done():
                    f.set_exception(ConnectionError('CDP disconnected'))

    async def send(self, method, params=None, session=None, timeout=6):
        self.seq += 1
        ident = self.seq
        f = asyncio.get_running_loop().create_future()
        self.pending[ident] = f
        payload = {'id': ident, 'method': method, 'params': params or {}}
        if session:
            payload['sessionId'] = session
        await self.ws.send_json(payload)
        try:
            obj = await asyncio.wait_for(f, timeout)
        finally:
            self.pending.pop(ident, None)
        if 'error' in obj:
            raise RuntimeError(str(obj['error']))
        return obj.get('result', {})

def probe_expression(selector):
    return """(() => {
      const b=document.body;
      let text='';
      if(b){
        const stack=[b],parts=[];
        while(stack.length){
          const n=stack.pop();
          if(n.nodeType===3){parts.push(n.nodeValue||'');continue;}
          if(n.nodeType!==1 || /^(SCRIPT|STYLE|NOSCRIPT|TEMPLATE)$/.test(String(n.nodeName).toUpperCase()))continue;
          const children=n.childNodes;
          for(let i=children.length-1;i>=0;i--)stack.push(children[i]);
        }
        text=parts.join('').replace(/\\s+/g,' ').trim();
      }
      return {url:location.href,title:document.title,readyState:document.readyState,
        selectorCount:document.querySelectorAll(SELECTOR).length,textLength:text.length,text:text.slice(0,14000),
        links:document.querySelectorAll('a[href]').length,frames:document.querySelectorAll('iframe').length,
        challengeFrames:Array.from(document.querySelectorAll('iframe')).map(e=>e.src).filter(s=>/challenges.cloudflare.com/.test(s)),
        userAgent:navigator.userAgent};
    })()""".replace('SELECTOR', json.dumps(selector))

def challenge(state):
    title = state.get('title', '').lower()
    text = state.get('text', '')[:1800].lower()
    return bool(re.search(r'just a moment|attention required|security verification|checking your browser|verifying your browser|один момент|проверка безопасности', title)
                or re.search(r'verify you are human|performing security verification|checking if the site connection is secure|enable javascript and cookies to continue|performing security checks', text))

async def trial(config, site, repetition, out, budget):
    name, url, selector, minimum = site
    tag = f'{name}-{config}-r{repetition}'
    folder = out / tag
    folder.mkdir(exist_ok=True)
    port = {'mimic':19341, 'lightpanda-default':19342, 'lightpanda-full':19343, 'chrome':19344}[config]
    lp_pid = None
    log = (folder / 'process.log').open('wb')
    if config == 'mimic':
        cmd = [str(MIMIC), '-listen', f'127.0.0.1:{port}', '-navigation-timeout', '40s']
    elif config == 'chrome':
        cmd = [str(CHROME), '--headless=new', f'--remote-debugging-port={port}', f'--user-data-dir={folder / "profile"}', '--no-first-run', '--no-default-browser-check', 'about:blank']
    else:
        args = [LP, 'serve', '--host', '127.0.0.1', '--port', str(port)]
        if config.endswith('full'):
            args += ['--load-resources', 'iframe', '--load-resources', 'stylesheet', '--load-resources', 'worker', '--load-resources', 'image', '--experimental-features', 'cors']
        # Paths and arguments are fixed by this harness. PID printed before exec.
        cmd = ['wsl', '-d', 'Ubuntu', '--', 'sh', '-c', 'echo $$; exec env LIGHTPANDA_DISABLE_TELEMETRY=true ' + ' '.join(args)]
    started = time.perf_counter()
    proc = subprocess.Popen(cmd, cwd=ROOT, stdout=log, stderr=subprocess.STDOUT, creationflags=subprocess.CREATE_NO_WINDOW)
    result = {'site':name, 'url':url, 'selector':selector, 'minimumTextLength':minimum, 'config':config,
              'repetition':repetition, 'started_utc':datetime.now(timezone.utc).isoformat(), 'command':cmd,
              'budget_seconds':budget,
              'probe_version':'read_only_traversal_v2',
              'samples':[], 'probeErrors':[], 'first_content_ms':None, 'outcome':'setup_error'}
    cdp = None
    try:
        async with aiohttp.ClientSession() as http:
            ws = None
            endpoint = f'http://127.0.0.1:{port}'
            for _ in range(80):
                try:
                    if config.startswith('lightpanda'):
                        ws = await http.ws_connect(endpoint.replace('http:', 'ws:')+'/', timeout=1, max_msg_size=32*1024*1024)
                    else:
                        async with http.get(endpoint+'/json/version', timeout=aiohttp.ClientTimeout(total=1)) as r:
                            version = await r.json()
                        ws = await http.ws_connect(version['webSocketDebuggerUrl'], max_msg_size=32*1024*1024)
                    break
                except Exception:
                    await asyncio.sleep(.1)
            if ws is None:
                raise RuntimeError('CDP startup failed')
            cdp = CDP(ws)
            result['version'] = await cdp.send('Browser.getVersion')
            result['cdp_ready_ms'] = (time.perf_counter()-started)*1000
            target = await cdp.send('Target.createTarget', {'url':'about:blank'})
            attached = await cdp.send('Target.attachToTarget', {'targetId':target['targetId'], 'flatten':True})
            sid = attached['sessionId']
            for method in ('Page.enable', 'Runtime.enable', 'Network.enable'):
                await cdp.send(method, session=sid)
            cdp.events.clear()
            cdp.origin = time.perf_counter()
            nav = asyncio.create_task(cdp.send('Page.navigate', {'url':url}, sid, timeout=budget+3))
            challenge_seen = False
            while time.perf_counter()-cdp.origin < budget:
                await asyncio.sleep(.25)
                try:
                    reply = await cdp.send('Runtime.evaluate', {'expression':probe_expression(selector), 'returnByValue':True}, sid, timeout=3)
                    if 'exceptionDetails' in reply:
                        raise RuntimeError(str(reply['exceptionDetails'])[:1000])
                    state = reply.get('result', {}).get('value')
                    if not isinstance(state, dict):
                        raise RuntimeError('No by-value DOM result')
                    ms = (time.perf_counter()-cdp.origin)*1000
                    state['elapsed_ms'] = ms
                    state['challenge'] = challenge(state)
                    result['samples'].append(state)
                    challenge_seen |= state['challenge']
                    documents = [e['params']['response'] for e in cdp.events if e.get('method')=='Network.responseReceived' and e.get('params',{}).get('type')=='Document']
                    top = [d for d in documents if d.get('url','').split('#')[0] == state['url'].split('#')[0]]
                    status = top[-1].get('status') if top else None
                    if top and any(k.lower()=='cf-mitigated' and str(v).lower()=='challenge' for k,v in top[-1].get('headers',{}).items()):
                        state['challenge'] = True
                        challenge_seen = True
                    good = state['url'] != 'about:blank' and state['selectorCount']>0 and state['textLength']>=minimum and not state['challenge'] and (status is None or status<400)
                    if name=='browserscan':
                        verdict=re.search(r'Test Results:\s*(Robot|Normal|Human|No bots detected)',state['text'])
                        state['bot_detection_verdict']=verdict.group(1) if verdict else None
                        good = good and bool(verdict)
                    if good and result['first_content_ms'] is None:
                        result['first_content_ms'] = ms
                    # Observe another 3 seconds for hydration/errors/late challenges.
                    if good and ms >= result['first_content_ms']+3000:
                        result['outcome'] = 'content_after_challenge' if challenge_seen else 'content_observed'
                        break
                    if status and status >= 400 and not state['challenge'] and ms>6000:
                        result['outcome'] = 'http_error'
                        break
                except Exception as exc:
                    result['probeErrors'].append({'elapsed_ms':(time.perf_counter()-cdp.origin)*1000,'error':repr(exc)[:1000]})
                    if ws.closed:
                        break
            if result['outcome']=='setup_error':
                result['outcome'] = 'challenge_unresolved' if challenge_seen else 'content_not_confirmed'
            result['challenge_seen'] = challenge_seen
            result['observation_ms'] = (time.perf_counter()-cdp.origin)*1000
            if nav.done():
                try:
                    result['navigation_reply'] = nav.result()
                except Exception as exc:
                    result['navigation_error'] = str(exc)
            else:
                nav.cancel()
                with contextlib.suppress(asyncio.CancelledError):
                    await nav
            result['final'] = result['samples'][-1] if result['samples'] else None
            if name == 'todomvc' and result['outcome']=='content_observed':
                action_start = time.perf_counter()
                try:
                    await cdp.send('Runtime.evaluate', {'expression':"document.querySelector('.new-todo').focus()"}, sid)
                    await cdp.send('Input.insertText', {'text':'Mimic comparison local test'}, sid)
                    await cdp.send('Input.dispatchKeyEvent', {'type':'keyDown','key':'Enter','code':'Enter','windowsVirtualKeyCode':13,'nativeVirtualKeyCode':13,'text':'\r'}, sid)
                    await cdp.send('Input.dispatchKeyEvent', {'type':'keyUp','key':'Enter','code':'Enter','windowsVirtualKeyCode':13,'nativeVirtualKeyCode':13}, sid)
                    await asyncio.sleep(.5)
                    reply = await cdp.send('Runtime.evaluate', {'expression':"JSON.stringify({count:document.querySelectorAll('.todo-list li').length,text:document.querySelector('.todo-list')?.textContent||''})",'returnByValue':True}, sid)
                    value = json.loads(reply['result']['value'])
                    result['interaction'] = {'action':'focus via DOM; CDP insertText and Enter; create local TodoMVC item', 'result':value,
                        'passed':value['count']==1 and 'Mimic comparison local test' in value['text'], 'elapsed_ms':(time.perf_counter()-action_start)*1000}
                except Exception as exc:
                    result['interaction'] = {'passed':False,'error':repr(exc)}
            result['events'] = cdp.events
            result['dcl_ms'] = next((e['elapsed_ms'] for e in cdp.events if e.get('method')=='Page.domContentEventFired'), None)
            result['load_ms'] = next((e['elapsed_ms'] for e in cdp.events if e.get('method')=='Page.loadEventFired'), None)
            # Record resource headers but never persist cookies or bearer values.
            for e in result['events']:
                params = e.get('params',{})
                for item in (params,params.get('request',{}),params.get('response',{})):
                    if 'headers' in item:
                        item['headers'] = {k:v for k,v in item['headers'].items() if k.lower() not in ('cookie','set-cookie','authorization','proxy-authorization')}
                    item.pop('postData',None)
            await ws.close()
            await cdp.reader
    except Exception as exc:
        result['error'] = str(exc)
    finally:
        if config.startswith('lightpanda'):
            log.flush()
            first = (folder/'process.log').read_text(errors='replace').splitlines()
            if first and first[0].strip().isdigit():
                lp_pid = int(first[0])
                subprocess.run(['wsl','-d','Ubuntu','--','kill','-TERM',str(lp_pid)], capture_output=True, timeout=5)
        try:
            root = psutil.Process(proc.pid)
            children = root.children(recursive=True)
            for child in children:
                with contextlib.suppress(psutil.Error):
                    child.kill()
            with contextlib.suppress(psutil.Error):
                root.kill()
        except psutil.Error:
            pass
        with contextlib.suppress(Exception):
            proc.wait(timeout=5)
        log.close()
        if cdp and not cdp.reader.done():
            cdp.reader.cancel()
        result['process_exit_code'] = proc.returncode
        result['harness_sha256'] = HARNESS_HASH
        write(folder/'result.json',result)
    summary = {k:result.get(k) for k in ('site','config','repetition','outcome','first_content_ms','dcl_ms','load_ms','error')}
    summary['title'] = (result.get('final') or {}).get('title')
    print(json.dumps(summary, ensure_ascii=False),flush=True)
    return summary

async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out',type=Path,required=True)
    parser.add_argument('--sites',default=','.join(s[0] for s in SITES))
    parser.add_argument('--configs',default='mimic,lightpanda-default,lightpanda-full,chrome')
    parser.add_argument('--rounds',type=int,default=1)
    parser.add_argument('--budget',type=float,default=30)
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True,exist_ok=True)
    (out / f'harness-{HARNESS_HASH[:12]}.py').write_bytes(HARNESS_SOURCE)
    if not (out/'manifest.json').exists():
        write(out/'manifest.json',{'created_utc':datetime.now(timezone.utc).isoformat(), 'git_head':capture(['git','rev-parse','HEAD']),
          'git_status':capture(['git','status','--short']), 'lightpanda_version':capture(['wsl','-d','Ubuntu','--',LP,'version']),
          'lightpanda_sha256':capture(['wsl','-d','Ubuntu','--','sha256sum',LP]),
          'mimic_sha256':hashlib.sha256(MIMIC.read_bytes()).hexdigest(), 'chrome_sha256':hashlib.sha256(CHROME.read_bytes()).hexdigest(),
          'harness_sha256':HARNESS_HASH, 'sites':SITES,
          'notes':'Fresh processes; no cache/profile reuse; native UA; Windows Mimic/Chrome versus WSL Lightpanda. Read-only DOM traversal: no cloneNode, removals, or injected scripts. DOM-content criterion is not full functional equivalence. No CAPTCHA interaction, clearance import or proxy rotation.'})
        (out/'working-tree.patch').write_text(capture(['git','diff','--binary']),encoding='utf-8')
    configs = args.configs.split(',')
    for repetition in range(1,args.rounds+1):
        for index, site in enumerate(SITES):
            if site[0] not in args.sites.split(','):
                continue
            order = configs if (index+repetition)%2 else list(reversed(configs))
            for config in order:
                if (out/f'{site[0]}-{config}-r{repetition}'/'result.json').exists():
                    continue
                await trial(config,site,repetition,out,args.budget)

if __name__=='__main__':
    asyncio.run(main())
