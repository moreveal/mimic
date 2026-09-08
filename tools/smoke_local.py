#!/usr/bin/env python3
"""Launch the built binary on loopback and verify CDP on both retained backends.
Requires websockets 13.1; never loads an external URL.
"""
import argparse
import asyncio
import json
import os
from pathlib import Path
import subprocess
import urllib.request
import websockets

async def check(binary, engine):
    args = [str(binary), '-listen', '127.0.0.1:0']
    if engine != 'v8':
        args += ['-engine', engine]
    process = await asyncio.create_subprocess_exec(
        *args, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE,
        creationflags=subprocess.CREATE_NO_WINDOW if os.name == 'nt' else 0)
    try:
        line = (await asyncio.wait_for(process.stdout.readline(), 15)).decode().strip()
        if not line.startswith('Mimic listening on http://127.0.0.1:'):
            raise RuntimeError('Server did not announce a loopback listener')
        endpoint = line.split(' on ', 1)[1]
        with urllib.request.urlopen(endpoint + '/json/version', timeout=5) as response:
            discovery = json.load(response)
        async with websockets.connect(discovery['webSocketDebuggerUrl']) as client:
            await client.send(json.dumps({'id': 1, 'method': 'Runtime.evaluate',
                'params': {'expression': 'Promise.resolve(40).then(value=>value+2)'}}))
            while True:
                reply = json.loads(await asyncio.wait_for(client.recv(), 10))
                if reply.get('id') == 1:
                    assert 'error' not in reply, reply
                    assert reply['result']['result']['value'] == 42, reply
                    break
        print(f'PASS {engine}: discovery, CDP evaluation and Promise checkpoint')
    finally:
        if process.returncode is None:
            process.terminate()
        await asyncio.wait_for(process.wait(), 5)

async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', default='.build/mimic.exe')
    args = parser.parse_args()
    binary = Path(args.binary).resolve(strict=True)
    for engine in ['v8', 'quickjs']:
        await check(binary, engine)

if __name__ == '__main__':
    asyncio.run(main())
