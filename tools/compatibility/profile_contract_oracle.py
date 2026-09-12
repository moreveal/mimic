"""Measure standard CDP projections in an owned frozen headful Chrome context.

Does not alter the frozen harness or expectations. --mimic optionally captures
the same standard commands from a fresh Mimic binary for a focused differential.
"""
import argparse
import asyncio
import hashlib
import json
import pathlib
import socket
import subprocess
import tempfile
import time
import urllib.request

import websockets


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


async def capture(binary, mimic=False):
    port = free_port()
    with tempfile.TemporaryDirectory(prefix='mimic-profile-oracle-') as directory:
        command = [str(binary), '-listen', f'127.0.0.1:{port}'] if mimic else [
            str(binary), f'--remote-debugging-port={port}',
            f'--user-data-dir={directory}', '--no-first-run',
            '--no-default-browser-check', '--window-size=1280,800', 'about:blank']
        startup = subprocess.STARTUPINFO()
        startup.dwFlags |= subprocess.STARTF_USESHOWWINDOW
        startup.wShowWindow = 0
        proc = subprocess.Popen(command, stdout=subprocess.DEVNULL,
                                stderr=subprocess.DEVNULL, startupinfo=startup)
        try:
            for _ in range(200):
                try:
                    with urllib.request.urlopen(f'http://127.0.0.1:{port}/json/version', timeout=.5) as r:
                        version = json.load(r)
                    break
                except (OSError, ValueError):
                    if proc.poll() is not None:
                        raise RuntimeError('Oracle process exited')
                    await asyncio.sleep(.1)
            else:
                raise TimeoutError('Oracle CDP readiness')
            if not mimic and version['Browser'] != 'Chrome/152.0.7977.82':
                raise RuntimeError('Wrong reference: ' + version['Browser'])
            async with websockets.connect(version['webSocketDebuggerUrl']) as ws:
                seq = 0
                async def call(method, params=None, session=None):
                    nonlocal seq
                    seq += 1
                    message = dict(id=seq, method=method, params=params or {})
                    if session:
                        message['sessionId'] = session
                    await ws.send(json.dumps(message))
                    while True:
                        reply = json.loads(await ws.recv())
                        if reply.get('id') == seq:
                            if 'error' in reply:
                                raise RuntimeError(reply['error'])
                            return reply['result']
                context = (await call('Target.createBrowserContext'))['browserContextId']
                try:
                    target = (await call('Target.createTarget', {'url': 'about:blank', 'browserContextId': context}))['targetId']
                    session = (await call('Target.attachToTarget', {'targetId': target, 'flatten': True}))['sessionId']
                    await call('Target.activateTarget', {'targetId': target})
                    await call('Runtime.evaluate', {'expression': '''globalThis.profileEvents=[];
                        addEventListener('resize',()=>profileEvents.push('resize'));
                        addEventListener('languagechange',()=>profileEvents.push('languagechange'));
                        screen.orientation.addEventListener('change',()=>profileEvents.push('orientation'));
                        matchMedia('(prefers-reduced-motion: reduce)').addEventListener('change',()=>profileEvents.push('reduced-motion'))''', 'returnByValue': True}, session)
                    await call('Runtime.evaluate', {'expression': '''globalThis.profileWorker=new Worker(URL.createObjectURL(new Blob([
                        "onmessage=()=>postMessage({ua:navigator.userAgent,languages:[...navigator.languages]})"
                        ],{type:'text/javascript'})))''', 'returnByValue': True}, session)
                    expression = '''({viewport:[innerWidth,innerHeight],outer:[outerWidth,outerHeight],
                        screen:[screen.width,screen.height,screen.availWidth,screen.availHeight],
                        dpr:devicePixelRatio,ua:navigator.userAgent,languages:[...navigator.languages],
                        dark:matchMedia('(prefers-color-scheme: dark)').matches,
                        reduced:matchMedia('(prefers-reduced-motion: reduce)').matches,events:[...profileEvents]})'''
                    async def observe():
                        await call('Runtime.evaluate', {'expression': 'new Promise(r=>setTimeout(r,50))', 'awaitPromise': True}, session)
                        value = (await call('Runtime.evaluate', {'expression': expression, 'returnByValue': True}, session))['result']['value']
                        value['worker'] = (await call('Runtime.evaluate', {'expression': 'new Promise(r=>{profileWorker.onmessage=e=>r(e.data);profileWorker.postMessage(1)})', 'awaitPromise': True, 'returnByValue': True}, session))['result']['value']
                        return value
                    observations = {'initial': await observe()}
                    await call('Emulation.setDeviceMetricsOverride', {'width': 800, 'height': 600,
                        'screenWidth': 3000, 'screenHeight': 1800, 'deviceScaleFactor': 2, 'mobile': False}, session)
                    observations['largeMetrics'] = await observe()
                    await call('Emulation.setDeviceMetricsOverride', {'width': 800, 'height': 600,
                        'deviceScaleFactor': 2, 'mobile': False}, session)
                    observations['implicitScreen'] = await observe()
                    await call('Emulation.setDeviceMetricsOverride', {'width': 800, 'height': 600,
                        'screenWidth': 1000, 'screenHeight': 700, 'deviceScaleFactor': 2, 'mobile': False}, session)
                    observations['metrics'] = await observe()
                    await call('Emulation.setUserAgentOverride', {'userAgent': 'ProfileOracle/1',
                        'platform': 'Win32', 'acceptLanguage': 'fr-FR,en'}, session)
                    await call('Emulation.setEmulatedMedia', {'features': [
                        {'name': 'prefers-color-scheme', 'value': 'dark'},
                        {'name': 'prefers-reduced-motion', 'value': 'reduce'}]}, session)
                    observations['identityMedia'] = await observe()
                    await call('Emulation.clearDeviceMetricsOverride', {}, session)
                    observations['reset'] = await observe()
                finally:
                    await call('Target.disposeBrowserContext', {'browserContextId': context})
                native_version = await call('Browser.getVersion')
                return {'captureMetadata': {'browser': version['Browser'], 'version': native_version,
                    'platform': 'Windows x64', 'browserMode': 'headful', 'freshControlledProfile': True,
                    'profileId': 'profile-contract-headful-controlled-v1', 'command': command,
                    'binarySHA256': hashlib.sha256(binary.read_bytes()).hexdigest(),
                    'probeSHA256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
                    'secureContext': False, 'crossOriginIsolated': False,
                    'featureOverrides': [], 'capturedAtUnix': time.time()}, 'observations': observations}
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=10)


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--chrome', type=pathlib.Path, required=True)
    parser.add_argument('--mimic', type=pathlib.Path)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    args = parser.parse_args()
    output = {'chrome': await capture(args.chrome.resolve())}
    if args.mimic:
        output['mimic'] = await capture(args.mimic.resolve(), True)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(output, indent=2), encoding='utf-8')
    print(json.dumps({key: value['observations'] for key, value in output.items()}, indent=2))


if __name__ == '__main__':
    asyncio.run(main())
