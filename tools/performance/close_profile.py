"""Attribute connection teardown tails using unchanged frozen static work."""
import argparse
import concurrent.futures as futures
import datetime
import json
from pathlib import Path
import subprocess
import sys
import threading
import time
import traceback

from fast_gate import ROOT, frozen, verify_binary


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--pages', type=int, default=100)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    args.mimic = output / 'mimic.exe'
    args.timeout = 30
    command = ['go', 'build', '-o', str(args.mimic), './cmd/mimic']
    subprocess.run(command, cwd=ROOT, check=True)
    expected = frozen.digest(args.mimic)
    fingerprint = frozen.harness_fingerprint()[0]
    baseline = json.loads((ROOT/'benchmark/results/raw.json').read_text(encoding='utf-8'))
    if fingerprint != baseline['metadata']['harness_sha256']:
        raise RuntimeError('Frozen harness changed')
    data = dict(diagnostic=True, command=command, sha256=expected,
                revision=subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
                status=subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True),
                harness_sha256=fingerprint, closes=[], waves=[])
    lock = threading.Lock()
    original = frozen.CDP

    class ObservedCDP(original):
        def __init__(self, url, *a, **kw):
            self.url = url
            self.last_method = None
            super().__init__(url, *a, **kw)

        def call(self, method, params=None):
            self.last_method = method
            return super().call(method, params)

        def close(self):
            row = dict(url=self.url, last_method=self.last_method)
            def waiting():
                frame = sys._current_frames().get(self.ws.recv_events_thread.ident)
                row['receiver_stack_at_2s'] = traceback.format_stack(frame) if frame else []
                row['protocol_state_at_2s'] = str(self.ws.protocol.state)
            timer = threading.Timer(2, waiting)
            started = time.perf_counter()
            timer.start()
            try:
                return super().close()
            finally:
                timer.cancel()
                timer.join()
                row['close_ms'] = (time.perf_counter()-started)*1000
                with lock:
                    data['closes'].append(row)

    frozen.CDP = ObservedCDP
    runtime = None
    try:
        verify_binary(args.mimic, expected)
        if frozen.harness_fingerprint()[0] != fingerprint:
            raise RuntimeError('Frozen harness changed before launch')
        data['launch'] = dict(executable=str(args.mimic), sha256=expected,
                              verified_at=datetime.datetime.now().astimezone().isoformat())
        runtime = frozen.Runtime('mimic', args)
        for index in range(3):
            servers = [frozen.Server('static') for _ in range(args.pages)]
            barrier = threading.Barrier(args.pages)
            release = threading.Event()
            done = [threading.Event() for _ in servers]
            start = time.perf_counter()
            try:
                with futures.ThreadPoolExecutor(max_workers=args.pages) as pool:
                    jobs = [pool.submit(frozen.execute, runtime, 'static', barrier, (event, release), server)
                            for event, server in zip(done, servers)]
                    paging_since = None
                    try:
                        while not all(event.is_set() for event in done):
                            current = runtime.tree.snapshot()
                            paging = current.get('system_pages_input_s') or 0
                            paging_since = (paging_since or time.perf_counter()) if paging > 1024 else None
                            if (time.perf_counter()-start > 180 or
                                current['system_available'] < max(2*1024**3, .15*frozen.psutil.virtual_memory().total) or
                                (paging_since and time.perf_counter()-paging_since >= 3)):
                                barrier.abort()
                                raise RuntimeError('diagnostic timeout or system memory pressure')
                            time.sleep(.05)
                        completed = time.perf_counter()
                    finally:
                        release.set()
                    rows = [job.result() for job in jobs]
                data['waves'].append(dict(index=index, rows=rows,
                    completion_s=completed-start, elapsed_s=time.perf_counter()-start))
                if any(row['status'] != 'VALID' for row in rows):
                    raise RuntimeError('Frozen workload correctness failure')
            finally:
                release.set()
                for server in servers:
                    server.close()
    finally:
        if runtime:
            runtime.close()
        frozen.CDP = original
        (output/'raw.json').write_text(json.dumps(data, indent=2), encoding='utf-8')


if __name__ == '__main__':
    main()
