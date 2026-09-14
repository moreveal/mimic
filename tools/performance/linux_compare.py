"""Linux process adapter for unchanged frozen workloads; separate from the baseline.

All engines use the same flattened CDP transport and completion predicates.
The frozen source, expectations and fixture bytes are imported, never rewritten.
Linux RSS is reported with USS/PSS; it is not Windows private working set.
"""
import argparse
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import signal
import statistics
import subprocess
import sys
import time
import types
import urllib.request

import psutil
from websockets.sync.client import connect

ROOT = Path(__file__).resolve().parents[2]
# Only the frozen runner's Windows process launcher is platform-specific.
# Its workload server, expected results and fingerprint function are reused.
sys.modules['windows_metrics'] = types.SimpleNamespace(Tree=None)
spec = importlib.util.spec_from_file_location('frozen_workloads', ROOT/'benchmark/run.py')
frozen = importlib.util.module_from_spec(spec)
spec.loader.exec_module(frozen)


def save(path, data):
    def portable(value):
        if isinstance(value, dict):
            return {key:portable(item) for key, item in value.items()}
        if isinstance(value, (list, tuple)):
            return [portable(item) for item in value]
        if isinstance(value, Path):
            value = str(value)
        if isinstance(value, str):
            for prefix, label in ((ROOT, '<repo>'), (Path.home(), '<user>')):
                value = value.replace(str(prefix), label)
        return value
    path.write_text(json.dumps(portable(data), indent=2, default=str), encoding='utf-8')


def source_state():
    git = ['git', '-c', 'core.safecrlf=false']
    changed = subprocess.check_output(git+['diff', '--name-only', '-z', 'HEAD'], cwd=ROOT).decode().split('\0')
    added = subprocess.check_output(git+['ls-files', '--others', '--exclude-standard', '-z'], cwd=ROOT).decode().split('\0')
    files = {name:frozen.digest(ROOT/name) if (ROOT/name).is_file() else None for name in sorted(set(changed+added)) if name}
    return dict(revision=frozen.shell(git+['rev-parse','HEAD']), changed_files=files,
                tracked_diff_sha256=hashlib.sha256(subprocess.check_output(git+['diff','HEAD'], cwd=ROOT)).hexdigest())


class CDP:
    def __init__(self, url, timeout=30):
        self.ws = connect(url, open_timeout=timeout, max_size=None)
        self.seq, self.session, self.timeout = 0, None, timeout

    def call(self, method, params=None):
        self.seq += 1
        payload = dict(id=self.seq, method=method, params=params or {})
        if self.session and not method.startswith(('Target.', 'Browser.')):
            payload['sessionId'] = self.session
        self.ws.send(json.dumps(payload))
        deadline = time.perf_counter()+self.timeout
        while True:
            result = json.loads(self.ws.recv(timeout=max(.001, deadline-time.perf_counter())))
            if result.get('id') != self.seq:
                continue
            if 'error' in result:
                raise RuntimeError(str(result['error']))
            return result.get('result', {})

    def evaluate(self, expression):
        result = self.call('Runtime.evaluate', dict(expression=expression, returnByValue=True))
        if result.get('exceptionDetails'):
            raise RuntimeError(str(result['exceptionDetails']))
        return result.get('result', {}).get('value')

    def close(self):
        self.ws.close()


class Runtime:
    def __init__(self, name, binary, directory, timeout):
        self.name, self.timeout = name, timeout
        self.kind = 'chrome' if name == 'chrome' else 'lightpanda' if name == 'lightpanda' else 'mimic'
        self.directory = directory
        directory.mkdir()
        self.port = frozen.free_port()
        self.log = (directory/'process.log').open('w')
        env = os.environ.copy()
        for key in list(env):
            if key.startswith('MIMIC_PROFILE_') or key in ('MIMIC_DIAGNOSTICS', 'MIMIC_V8_CPU_PROFILE', 'MIMIC_V8_HEAP_SNAPSHOT'):
                env.pop(key)
        env['LIGHTPANDA_DISABLE_TELEMETRY'] = 'true'
        if self.kind == 'mimic':
            command = [str(binary), '-listen', f'127.0.0.1:{self.port}', '-engine', 'v8', '-chrome', '152', '-browser-mode', 'headless']
        elif self.kind == 'chrome':
            command = [str(binary), '--headless=new', f'--user-data-dir={directory/"profile"}', f'--remote-debugging-port={self.port}', '--remote-debugging-address=127.0.0.1', '--no-first-run', '--no-default-browser-check', '--disable-background-networking', '--disable-extensions', '--disable-component-extensions-with-background-pages', 'about:blank']
        else:
            command = [str(binary), 'serve', '--host', '127.0.0.1', '--port', str(self.port), '--load-resources', 'iframe', '--load-resources', 'stylesheet', '--load-resources', 'worker', '--load-resources', 'image', '--experimental-features', 'cors']
        # Verify before launch; hashing the executable is controller setup,
        # not part of process-to-first-result latency.
        digest = frozen.digest(binary)
        self.started = time.perf_counter()
        self.process = subprocess.Popen(command, stdout=self.log, stderr=subprocess.STDOUT, env=env, start_new_session=True)
        self.receipt = dict(command=command, sha256=digest, pid=self.process.pid)
        self.seen_cpu = {}
        try:
            deadline = time.perf_counter()+30
            while time.perf_counter() < deadline:
                if self.process.poll() is not None:
                    raise RuntimeError('Process exited before readiness; see process.log')
                try:
                    if self.kind == 'lightpanda':
                        self.url = f'ws://127.0.0.1:{self.port}/'
                    else:
                        with urllib.request.urlopen(f'http://127.0.0.1:{self.port}/json/version', timeout=.2) as response:
                            self.url = json.load(response)['webSocketDebuggerUrl']
                    self.root = CDP(self.url)
                    break
                except (OSError, TimeoutError):
                    time.sleep(.005)
            else:
                raise TimeoutError('CDP readiness')
            self.receipt['version'] = self.root.call('Browser.getVersion')
            if self.kind == 'chrome' and self.receipt['version']['product'] != 'Chrome/'+frozen.PIN:
                raise RuntimeError('Chrome pin mismatch')
            self.receipt['cdp_ready_ms'] = frozen.ms(self.started)
            self.receipt['ready_memory'] = self.snapshot()
        except BaseException:
            self.close()
            raise

    def snapshot(self):
        row = dict(rss=0, uss=0, pss=0, cpu_s=0, processes=0, system_available=psutil.virtual_memory().available)
        try:
            parent = psutil.Process(self.process.pid)
            processes = [parent]+parent.children(recursive=True)
        except psutil.Error:
            processes = []
        for process in processes:
            try:
                memory, cpu = process.memory_full_info(), process.cpu_times()
                self.seen_cpu[(process.pid, process.create_time())] = cpu.user+cpu.system
                for field in ('rss', 'uss', 'pss'):
                    row[field] += getattr(memory, field, 0)
                row['processes'] += 1
            except psutil.Error:
                pass
        row['cpu_s'] = sum(self.seen_cpu.values())
        return row

    def close(self):
        started = time.perf_counter()
        if hasattr(self, 'root'):
            try:
                if self.kind == 'chrome':
                    self.root.call('Browser.close')
                self.root.close()
            except Exception:
                pass
        if self.process.poll() is None:
            os.killpg(self.process.pid, signal.SIGTERM)
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(self.process.pid, signal.SIGKILL)
            self.process.wait()
        self.log.close()
        self.receipt['shutdown_ms'] = frozen.ms(started)
        self.receipt['exit_code'] = self.process.returncode


def execute(runtime, work, barrier=None, hold=None, server=None):
    row = dict(system=runtime.name, workload=work, status='ERROR')
    owned_server = server is None
    server = server or frozen.Server(work)
    page, target = None, None
    try:
        if barrier is None:
            row['before_session'] = runtime.snapshot()
        started = time.perf_counter()
        page = CDP(runtime.url, runtime.timeout)
        target = page.call('Target.createTarget', dict(url='about:blank'))['targetId']
        page.session = page.call('Target.attachToTarget', dict(targetId=target, flatten=True))['sessionId']
        page.call('Network.enable')
        page.call('Network.setCacheDisabled', dict(cacheDisabled=True))
        row['session_create_ms'] = frozen.ms(started)
        if barrier is None:
            row['after_session'] = runtime.snapshot()
        else:
            barrier.wait(timeout=120)
        nav = time.perf_counter()
        ack = page.call('Page.navigate', dict(url=server.url))
        if ack.get('errorText'):
            raise RuntimeError(ack['errorText'])
        frozen.wait_value(page, '({ready:document.readyState,url:location.href,runner:typeof window.__benchRun})', lambda v:v and v['ready']=='complete' and v['url']==server.url and v['runner']=='function', runtime.timeout)
        row['navigation_ms'] = frozen.ms(nav)
        start = time.perf_counter()
        page.evaluate('void window.__benchRun()')
        value = frozen.wait_value(page, 'window.__bench', lambda v:v and v.get('done'), runtime.timeout)
        row.update(execution_ms=frozen.ms(start), completion_ms=frozen.ms(nav), result=value, expected=frozen.EXPECTED[work])
        row['first_result_from_process_ms'] = frozen.ms(runtime.started)
        row['status'] = 'VALID' if value.get('result') == frozen.EXPECTED[work] and not value.get('error') else 'INVALID'
        if barrier is None:
            row['after_workload'] = runtime.snapshot()
    except Exception as error:
        row['error'] = repr(error)
        if barrier:
            barrier.abort()
    finally:
        if hold:
            hold[0].set()
            hold[1].wait(timeout=180)
        start = time.perf_counter()
        if page and target:
            try:
                closed = page.call('Target.closeTarget', dict(targetId=target))
                if not closed.get('success'):
                    raise RuntimeError('Target.closeTarget returned false')
            except Exception as error:
                row['teardown_error'] = repr(error)
                row['status'] = 'ERROR'
        if page:
            page.close()
        row['teardown_ms'] = frozen.ms(start)
        if barrier is None:
            time.sleep(.25)
            row['after_recovery'] = runtime.snapshot()
        row['server_requests'] = server.requests
        if owned_server:
            server.close()
    return row


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--engine', action='append', required=True, help='name=/absolute/binary; use chrome and lightpanda for comparators')
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--workloads', nargs='+', default=frozen.WORKLOADS, choices=frozen.WORKLOADS)
    parser.add_argument('--rounds', type=int, default=4)
    parser.add_argument('--iterations', type=int, default=3)
    parser.add_argument('--timeout', type=float, default=30)
    args = parser.parse_args()
    if platform.system() != 'Linux':
        parser.error('Run every engine and the controller on Linux')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    engines = dict(item.split('=', 1) for item in args.engine)
    engines = {name: Path(path).resolve() for name, path in engines.items()}
    fingerprint, files = frozen.harness_fingerprint()
    baseline = json.loads((ROOT/'benchmark/results/raw.json').read_text())
    if fingerprint != baseline['metadata']['harness_sha256']:
        raise RuntimeError('Frozen harness differs from immutable baseline')
    data = dict(metadata=dict(date=datetime.datetime.now(datetime.timezone.utc).isoformat(), platform=platform.platform(), processor=platform.processor(), harness_sha256=fingerprint, harness_files=files, adapter_sha256=frozen.digest(__file__), controller_source=source_state(), resource_policy='Full supported resource loading; HTTP cache disabled; unique origins; OS file cache uncontrolled; no forced GC'), binaries={name:dict(path=str(path),sha256=frozen.digest(path)) for name,path in engines.items()}, launches=[], rows=[])
    save(output/'raw.json', data)
    for index in range(args.rounds):
        order = list(engines) if index%2 == 0 else list(reversed(engines))
        for name in order:
            if frozen.digest(engines[name]) != data['binaries'][name]['sha256']:
                raise RuntimeError('Binary changed during comparison')
            runtime = Runtime(name, engines[name], output/f'{index}-{name}', args.timeout)
            data['launches'].append(dict(round=index, system=name, **runtime.receipt))
            try:
                for work in args.workloads:
                    for iteration in range(-1, args.iterations):
                        row = execute(runtime, work)
                        row.update(round=index, iteration=iteration, excluded=iteration<0)
                        data['rows'].append(row)
                        save(output/'raw.json', data)
                        print(name, index, work, iteration, row['status'], round(row.get('execution_ms', 0), 2), flush=True)
                        if row['status'] != 'VALID':
                            break
            finally:
                runtime.close()
                data['launches'][-1].update(runtime.receipt)
                save(output/'raw.json', data)
    data['summary'] = {}
    for name in engines:
        data['summary'][name] = {}
        for work in args.workloads:
            rows = [r for r in data['rows'] if r['system']==name and r['workload']==work and not r['excluded']]
            gates = [r for r in data['rows'] if r['system']==name and r['workload']==work and r['excluded']]
            valid = (len(rows) == args.rounds*args.iterations and len(gates) == args.rounds
                     and all(r['status']=='VALID' for r in rows+gates))
            data['summary'][name][work] = dict(status='VALID' if valid else 'UNSUPPORTED_OR_FAILED', samples=len(rows))
            if valid:
                for key in ('session_create_ms','navigation_ms','execution_ms','completion_ms','teardown_ms'):
                    data['summary'][name][work][key] = statistics.median(r[key] for r in rows)
    if frozen.harness_fingerprint()[0] != fingerprint:
        raise RuntimeError('Frozen harness changed during comparison')
    save(output/'raw.json', data)
    print(json.dumps(data['summary'], indent=2))


if __name__ == '__main__':
    main()
