"""Observe the frozen readiness probe without changing its polling policy."""
import argparse
import datetime
import json
from pathlib import Path
import subprocess
import threading
import time

from fast_gate import ROOT, frozen, verify_binary


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    args.mimic = output/'mimic.exe'
    command = ['go', 'build', '-o', str(args.mimic), './cmd/mimic']
    subprocess.run(command, cwd=ROOT, check=True)
    expected = frozen.digest(args.mimic)
    fingerprint = frozen.harness_fingerprint()[0]
    baseline = json.loads((ROOT/'benchmark/results/raw.json').read_text(encoding='utf-8'))
    if fingerprint != baseline['metadata']['harness_sha256']:
        raise RuntimeError('Frozen harness changed')
    data = dict(diagnostic=True, command=command, executable=str(args.mimic), sha256=expected,
                revision=subprocess.check_output(['git','rev-parse','HEAD'], cwd=ROOT, text=True).strip(),
                status=subprocess.check_output(['git','status','--porcelain'], cwd=ROOT, text=True),
                harness_sha256=fingerprint, rows=[])
    original_tree, original_open = frozen.Tree, frozen.urllib.request.urlopen
    active = {}

    class ObservedTree(original_tree):
        def __init__(self, command, env, log, *a, **kw):
            super().__init__(command, env, log, *a, **kw)
            active['process_start'] = self.start
            active['tree_ready_ms'] = (time.perf_counter()-self.start)*1000
            def observe():
                deadline = time.perf_counter()+2
                while time.perf_counter() < deadline:
                    if 'Mimic listening' in Path(log.name).read_text(encoding='utf-8', errors='replace'):
                        active['stdout_listening_ms'] = (time.perf_counter()-self.start)*1000
                        return
                    time.sleep(.001)
            self.observer = threading.Thread(target=observe)
            self.observer.start()

    def observed_open(*a, **kw):
        row = dict(start_ms=(time.perf_counter()-active['process_start'])*1000)
        started = time.perf_counter()
        try:
            return original_open(*a, **kw)
        except Exception as error:
            row['error'] = repr(error)
            raise
        finally:
            row['elapsed_ms'] = (time.perf_counter()-started)*1000
            active.setdefault('http_attempts', []).append(row)

    frozen.Tree = ObservedTree
    frozen.urllib.request.urlopen = observed_open
    try:
        for i in range(5):
            active = dict(iteration=i)
            verify_binary(args.mimic, expected)
            if frozen.harness_fingerprint()[0] != fingerprint:
                raise RuntimeError('Frozen harness changed before launch')
            active['verified_at'] = datetime.datetime.now().astimezone().isoformat()
            runtime = frozen.Runtime('mimic', args)
            try:
                runtime.tree.observer.join()
                active['cdp_ready_ms'] = runtime.cdp_ready_ms
                active['ready_memory'] = runtime.ready_memory
                data['rows'].append(active)
            finally:
                runtime.close()
    finally:
        frozen.Tree, frozen.urllib.request.urlopen = original_tree, original_open
        (output/'raw.json').write_text(json.dumps(data, indent=2), encoding='utf-8')


if __name__ == '__main__':
    main()
