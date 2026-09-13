"""Local cross-realm diagnostic. Does not modify or replace frozen workloads."""

import argparse
import asyncio
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import sys

import aiohttp
import psutil
from aiohttp import web

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools/compatibility"))
from live_browser_comparison import CDP

EXPRESSION = r"""(options=>{
 const frame=document.createElement('iframe');document.body.appendChild(frame);
 const child=frame.contentWindow;
 child.eval('globalThis.sample={value:7,extra:11};globalThis.add=(a,b)=>a+b;globalThis.localLoop=n=>{let sum=0;for(let i=0;i<n;i++)sum+=sample.value;return sum}');
 const remote=child.sample,foreign=child.add,inside=child.localLoop,proto=Object.getPrototypeOf(remote),n=options.iterations,rows=[];
 const measure=(name,fn,want)=>{
  if(fn()!==want)throw Error(name+' warmup checksum');
  const samples=[];
  for(let i=0;i<options.samples;i++){const start=performance.now(),sum=fn();samples.push(performance.now()-start);if(sum!==want)throw Error(name+' checksum')}
  rows.push({name,iterations:n,samples_ms:samples,checksum:want});
 };
 measure('local-call',()=>{const add=(a,b)=>a+b;let sum=0;for(let i=0;i<n;i++)sum+=add(i,7);return sum},n*(n-1)/2+7*n);
 measure('foreign-call',()=>{let sum=0;for(let i=0;i<n;i++)sum+=foreign(i,7);return sum},n*(n-1)/2+7*n);
 measure('foreign-get',()=>{let sum=0;for(let i=0;i<n;i++)sum+=remote.value;return sum},7*n);
 measure('foreign-local-loop',()=>inside(n),7*n);
 measure('foreign-has',()=>{let sum=0;for(let i=0;i<n;i++)sum+=('value' in remote);return sum},n);
 measure('foreign-prototype',()=>{let sum=0;for(let i=0;i<n;i++)sum+=Object.getPrototypeOf(remote)===proto;return sum},n);
 measure('foreign-descriptor',()=>{let sum=0;for(let i=0;i<n;i++)sum+=Object.getOwnPropertyDescriptor(remote,'value').value;return sum},7*n);
 measure('foreign-keys',()=>{let sum=0;for(let i=0;i<n;i++)sum+=Reflect.ownKeys(remote).length;return sum},2*n);
 measure('foreign-set',()=>{for(let i=0;i<n;i++)remote.value=i;return remote.value},n-1);
 frame.remove();return rows;
})"""


async def run(args):
    binary = args.binary.resolve()
    args.output.mkdir(parents=True, exist_ok=False)
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    command = [str(binary), "-listen", f"127.0.0.1:{port}"]
    if args.chrome:
        command = [str(binary), f"--remote-debugging-port={port}",
                   f"--user-data-dir={args.output.resolve() / 'profile'}",
                   "--no-first-run", "--no-default-browser-check",
                   "--window-size=1280,800", "about:blank"]
    result = {"binary": str(binary), "sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
              "command": command, "iterations": args.iterations, "samples": args.samples,
              "probe_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}

    async def fixture(_):
        return web.Response(text="<!doctype html><body>Frame bridge diagnostic", content_type="text/html")

    app = web.Application()
    app.router.add_get("/{path:.*}", fixture)
    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, "127.0.0.1", 0)
    await site.start()
    fixture_port = runner.addresses[0][1]
    process = None
    try:
        with (args.output / "process.log").open("wb") as log:
            process = subprocess.Popen(command, cwd=ROOT, stdout=log, stderr=log,
                                       creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0)
            async with aiohttp.ClientSession() as http:
                for _ in range(100):
                    try:
                        async with http.get(f"http://127.0.0.1:{port}/json/version") as response:
                            result["version"] = await response.json()
                        break
                    except (aiohttp.ClientError, OSError):
                        await asyncio.sleep(.2)
                else:
                    raise TimeoutError("browser startup")
                async with http.ws_connect(result["version"]["webSocketDebuggerUrl"], max_msg_size=64 << 20) as ws:
                    cdp = CDP(ws)
                    target = await cdp.send("Target.createTarget", {"url": "about:blank"})
                    sid = (await cdp.send("Target.attachToTarget", {"targetId": target["targetId"], "flatten": True}))["sessionId"]
                    await cdp.send("Page.enable", {}, sid)
                    await cdp.send("Page.navigate", {"url": f"http://127.0.0.1:{fixture_port}/"}, sid)
                    await asyncio.sleep(.5)
                    options = json.dumps({"iterations": args.iterations, "samples": args.samples})
                    reply = await cdp.send("Runtime.evaluate", {"expression": EXPRESSION + "(" + options + ")", "returnByValue": True}, sid, timeout=120)
                    if "exceptionDetails" in reply:
                        raise RuntimeError(reply["exceptionDetails"])
                    result["rows"] = reply["result"]["value"]
                    await cdp.send("Target.closeTarget", {"targetId": target["targetId"]})
    except BaseException as error:
        result["error"] = repr(error)
        raise
    finally:
        if process:
            children = []
            if process.poll() is None:
                try:
                    children = psutil.Process(process.pid).children(recursive=True)
                except psutil.NoSuchProcess:
                    pass
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
            for child in children:
                try:
                    child.kill()
                except psutil.NoSuchProcess:
                    pass
        await runner.cleanup()
        (args.output / "result.json").write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(result["rows"], ensure_ascii=False))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--chrome", action="store_true")
    parser.add_argument("--iterations", type=int, default=1000)
    parser.add_argument("--samples", type=int, default=5)
    args = parser.parse_args()
    if args.iterations < 1 or args.samples < 1:
        parser.error("iterations and samples must be positive")
    asyncio.run(run(args))
