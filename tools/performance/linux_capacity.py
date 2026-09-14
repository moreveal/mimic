"""Linux capacity checkpoint using the unchanged frozen semantic workloads.

This is an additional adapter, not the frozen Windows benchmark. All engines
use the same flattened CDP client, barrier, live-page hold and recovery policy.
"""
import argparse
import concurrent.futures as futures
import datetime
import json
from pathlib import Path
import platform
import statistics
import threading
import time

import psutil
import linux_compare as harness


def wave(runtime, work, count, index):
    servers = [harness.frozen.Server(work) for _ in range(count)]
    barrier, release = threading.Barrier(count), threading.Event()
    all_finished, completion_lock = threading.Event(), threading.Lock()
    remaining, completion_time = count, None

    class CompletionEvent(threading.Event):
        def set(self):
            nonlocal remaining, completion_time
            with completion_lock:
                if not self.is_set():
                    super().set()
                    remaining -= 1
                    if remaining == 0:
                        completion_time = time.perf_counter()
                        all_finished.set()

    finished = [CompletionEvent() for _ in range(count)]
    before = runtime.snapshot()
    samples, reason, active = [], None, before
    started = time.perf_counter()
    initial_swap = psutil.swap_memory().sin
    paging_since, previous_swap, previous_time = None, initial_swap, started
    try:
        with futures.ThreadPoolExecutor(max_workers=count) as pool:
            jobs = [pool.submit(harness.execute, runtime, work, barrier, (done, release), server)
                    for done, server in zip(finished, servers)]
            try:
                while not all_finished.is_set():
                    now = time.perf_counter()
                    sample = runtime.snapshot()
                    sample['elapsed_s'] = now-started
                    samples.append(sample)
                    swap = psutil.swap_memory().sin
                    sample['swap_in_bytes_s'] = (swap-previous_swap)/max(.001, now-previous_time)
                    previous_swap, previous_time = swap, now
                    if sample['swap_in_bytes_s'] > 1024*4096:
                        paging_since = paging_since or now
                        if now-paging_since >= 3:
                            reason = 'sustained swap input'
                    else:
                        paging_since = None
                    if sample['system_available'] < max(2*1024**3, .15*psutil.virtual_memory().total):
                        reason = 'memory pressure'
                    if now-started > 180:
                        reason = 'wave timeout'
                    if reason:
                        barrier.abort()
                        break
                    # Completion wakes the controller immediately. Periodic
                    # memory sampling must not impose a 50 ms latency floor.
                    all_finished.wait(.05)
                completed = completion_time or time.perf_counter()
                active = runtime.snapshot()
            finally:
                release.set()
            rows = [job.result() for job in jobs]
        elapsed = time.perf_counter()-started
        after = runtime.snapshot()
        time.sleep(.25)
        recovered = runtime.snapshot()
        valid = sum(row['status'] == 'VALID' for row in rows)
        return dict(workload=work, n=count, wave=index, excluded=index < 0, rows=rows,
                    status='VALID' if valid == count and not reason else 'UNSUPPORTED_OR_FAILED',
                    stop_reason=reason, before_wave=before, active_memory=active,
                    after_teardown=after, after_recovery=recovered, samples=samples,
                    elapsed_s=elapsed, completion_window_s=completed-started,
                    throughput=valid/elapsed, cpu_s=after['cpu_s']-before['cpu_s'],
                    # The live increment belongs to this wave. Subtracting
                    # process-ready memory double-counts allocator high-water
                    # retained by an excluded warmup or earlier measured wave.
                    marginal_uss_mib=(active['uss']-before['uss'])/count/2**20,
                    marginal_pss_mib=(active['pss']-before['pss'])/count/2**20,
                    active_over_ready_uss_mib=(active['uss']-runtime.receipt['ready_memory']['uss'])/count/2**20,
                    active_over_ready_pss_mib=(active['pss']-runtime.receipt['ready_memory']['pss'])/count/2**20,
                    retained_uss_mib=(recovered['uss']-before['uss'])/count/2**20,
                    retained_pss_mib=(recovered['pss']-before['pss'])/count/2**20,
                    swap_in_bytes=psutil.swap_memory().sin-initial_swap)
    finally:
        release.set()
        for server in servers:
            server.close()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--engine', action='append', required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--workloads', nargs='+', default=['static', 'cpu', 'react'], choices=harness.frozen.WORKLOADS)
    parser.add_argument('--levels', nargs='+', type=int, default=[1, 5, 10, 25, 50, 100])
    parser.add_argument('--rounds', type=int, default=2)
    parser.add_argument('--waves', type=int, default=3)
    parser.add_argument('--timeout', type=float, default=30)
    args = parser.parse_args()
    if platform.system() != 'Linux':
        parser.error('Run all engines and the controller on Linux')
    if any(n < 1 or n > 100 for n in args.levels):
        parser.error('Capacity levels must be between 1 and 100')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    engines = {name:Path(path).resolve() for name, path in (item.split('=', 1) for item in args.engine)}
    fingerprint, files = harness.frozen.harness_fingerprint()
    baseline = json.loads((harness.ROOT/'benchmark/results/raw.json').read_text())
    if fingerprint != baseline['metadata']['harness_sha256']:
        raise RuntimeError('Frozen harness differs from original baseline')
    data = dict(metadata=dict(date=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                             platform=platform.platform(), cpu_count=psutil.cpu_count(),
                             total_ram=psutil.virtual_memory().total, harness_sha256=fingerprint,
                             harness_files=files, adapter_sha256=harness.frozen.digest(__file__),
                             transport_sha256=harness.frozen.digest(harness.__file__),
                             controller_source=harness.source_state(),
                             levels=args.levels, waves=args.waves, rounds=args.rounds,
                             policy='Unique origins, full resources, HTTP cache disabled, no forced GC, 250 ms recovery. USS/PSS are Linux metrics. CPU includes only sampled descendants; exited children can be undercounted.'),
                binaries={name:dict(path=str(path), sha256=harness.frozen.digest(path)) for name, path in engines.items()},
                gates=[], launches=[], concurrency=[], summary={})
    harness.save(output/'raw.json', data)
    for turn in range(args.rounds):
        order = list(engines) if turn % 2 == 0 else list(reversed(engines))
        for name in order:
            for work in args.workloads:
                for count in args.levels:
                    directory = output/f'{turn}-{name}-{work}-{count}'
                    if harness.frozen.digest(engines[name]) != data['binaries'][name]['sha256']:
                        raise RuntimeError('Executable changed before launch')
                    runtime = harness.Runtime(name, engines[name], directory, args.timeout)
                    data['launches'].append(dict(round=turn, system=name, workload=work, n=count, **runtime.receipt))
                    failed = False
                    try:
                        gate = harness.execute(runtime, work)
                        data['gates'].append(dict(round=turn, n=count, **gate))
                        if gate['status'] != 'VALID':
                            failed = True
                            break
                        failed = False
                        for index in range(-1, args.waves):
                            row = wave(runtime, work, count, index)
                            row.update(round=turn, system=name)
                            data['concurrency'].append(row)
                            harness.save(output/'raw.json', data)
                            print(name, turn, work, count, index, row['status'], round(row['throughput'], 2), flush=True)
                            if row['status'] != 'VALID':
                                failed = True
                                break
                        if failed:
                            # Preserve unsupported/failed levels; never report
                            # fast partial completion as a successful capacity.
                            break
                    finally:
                        runtime.close()
                        data['launches'][-1].update(runtime.receipt)
                        harness.save(output/'raw.json', data)
    for name in engines:
        data['summary'][name] = {}
        for work in args.workloads:
            data['summary'][name][work] = {}
            for count in args.levels:
                rows = [r for r in data['concurrency'] if r['system'] == name and r['workload'] == work and r['n'] == count and not r['excluded']]
                valid = len(rows) == args.rounds*args.waves and all(r['status'] == 'VALID' for r in rows)
                item = dict(status='VALID' if valid else 'UNSUPPORTED_OR_FAILED', samples=len(rows))
                if valid:
                    for key in ('throughput', 'elapsed_s', 'completion_window_s', 'cpu_s',
                                'marginal_uss_mib', 'marginal_pss_mib',
                                'active_over_ready_uss_mib', 'active_over_ready_pss_mib',
                                'retained_uss_mib', 'retained_pss_mib'):
                        item[key] = statistics.median(r[key] for r in rows)
                data['summary'][name][work][count] = item
    if harness.frozen.harness_fingerprint()[0] != fingerprint:
        raise RuntimeError('Frozen harness changed during measurement')
    harness.save(output/'raw.json', data)


if __name__ == '__main__':
    main()
