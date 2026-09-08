"""Summarize execution-only host counters from profile_gate --hosts."""
import argparse
import json
import statistics
from pathlib import Path


def summarize(path):
    phases = json.loads(path.read_text(encoding='utf-8'))
    samples = []
    for end in phases:
        if end['phase'] != 'execute':
            continue
        start = next(p for p in phases if p['phase'] == 'navigate' and p['iteration'] == end['iteration'])
        before = start['v8']['detail']['costs']
        after = end['v8']['detail']['costs']
        costs = {name[5:]: {key: value[key] - before.get(name, {}).get(key, 0)
                           for key in ('count', 'ns')}
                 for name, value in after.items() if name.startswith('host:')}
        costs = {name: value for name, value in costs.items() if value['count']}
        crossings = end['v8']['host_crossings'] - start['v8']['host_crossings']
        assert sum(c['count'] for c in costs.values()) == crossings
        assert 'profileWrapper' not in costs
        samples.append(dict(iteration=end['iteration'], execution_ms=end['ms'], crossings=crossings, costs=costs))
    names = set().union(*(s['costs'] for s in samples))
    rows = []
    for name in names:
        counts = [s['costs'].get(name, {}).get('count', 0) for s in samples]
        times = [s['costs'].get(name, {}).get('ns', 0) for s in samples]
        rows.append(dict(name=name, count_min=min(counts), count_max=max(counts),
                         count_total=sum(counts), total_ms=sum(times)/1e6,
                         mean_ms=sum(times)/len(times)/1e6, median_ms=statistics.median(times)/1e6))
    rows.sort(key=lambda r: (-r['total_ms'], r['name']))
    return dict(iterations=len(samples), execution_median_ms=statistics.median(s['execution_ms'] for s in samples),
                crossings_total=sum(s['crossings'] for s in samples), rows_by_time=rows, samples=samples)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('phases', type=Path)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    result = summarize(args.phases)
    args.output.write_text(json.dumps(result, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({k: v for k, v in result.items() if k != 'samples'}, indent=2))
