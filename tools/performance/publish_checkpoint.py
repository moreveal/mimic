"""Export numerical checkpoint data and a concise report, without rerunning work.

The frozen runner and its raw observations are never modified. All failed and
excluded samples remain represented; a partially successful series has no
comparative latency headline.
"""
import argparse
import hashlib
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'benchmark'))
from report import stats, table

SYSTEMS = ('mimic', 'chrome')
WORKLOADS = ('static', 'cpu', 'dom', 'async', 'react', 'wasm')
LABELS = dict(static='Static DOM', cpu='JavaScript / crypto', dom='DOM mutations',
              async_='Async / networking', react='React', wasm='WebAssembly')
LABELS['async'] = LABELS.pop('async_')


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('run', type=Path)
    args = parser.parse_args()
    directory = args.run.resolve()
    raw_path, summary_path = directory/'raw.json', directory/'summary.json'
    raw = json.loads(raw_path.read_text(encoding='utf-8'))
    summary = json.loads(summary_path.read_text(encoding='utf-8'))
    manifest = json.loads((directory/'manifest.json').read_text(encoding='utf-8'))
    for path in (raw_path, summary_path):
        if digest(path) != manifest['sha256'][path.name]:
            raise ValueError('Checkpoint hash mismatch: '+path.name)
    meta = raw['metadata']
    if not raw.get('finished') or meta['arguments'].get('smoke'):
        raise ValueError('A completed full checkpoint is required')
    calibration = raw['startup_calibration']
    if not calibration['complete']:
        raise ValueError('Startup calibration is incomplete')
    startup = {}
    for system in SYSTEMS:
        rows = [r for r in calibration['rows'] if r['system']==system and not r['excluded']]
        if len(rows) != 10:
            raise ValueError('Expected ten measured startup samples')
        startup[system] = dict(cdp_ready_ms=stats([r['cdp_ready_ms'] for r in rows]),
            rss_mib=stats([r['ready_memory']['rss']/2**20 for r in rows]),
            private_mib=stats([r['ready_memory']['private']/2**20 for r in rows]))
    sample_fields = ('system', 'workload', 'mode', 'iteration', 'excluded', 'status',
                     'session_create_ms', 'navigation_ms', 'execution_ms', 'completion_ms',
                     'total_cold_ms', 'teardown_ms', 'session_cpu_s')
    data = dict(schema_version=1, checkpoint=meta['date'][:10], started=meta['date'],
        finished=raw['finished'], environment=dict(os=meta['os'], cpu=json.loads(meta['cpu']),
            ram_gib=meta['total_ram']/2**30, chrome_version=meta['target']['chrome_version'],
            mimic_v8=meta['mimic_v8'], chrome_v8=meta['chrome_version']['jsVersion'],
            power_plan=meta['power_mode'], background_caveat=meta['background_caveat']),
        provenance=dict(binary_sha256={s:meta['binaries'][s]['sha256'] for s in SYSTEMS},
            harness_sha256=meta['harness_sha256'], private_raw_sha256=digest(raw_path),
            exporter_sha256=digest(Path(__file__))), gates=raw['gates'], startup=startup,
        **summary, completion_samples=[{k:r[k] for k in sample_fields if k in r} for r in raw['rows']],
        startup_samples=[dict(system=r['system'], iteration=r['iteration'], excluded=r['excluded'],
            cdp_ready_ms=r['cdp_ready_ms'], rss_mib=r['ready_memory']['rss']/2**20,
            private_mib=r['ready_memory']['private']/2**20) for r in calibration['rows']])
    (directory/'public-results.json').write_text(json.dumps(data, indent=2)+'\n', encoding='utf-8')

    def fmt(value):
        return 'Unavailable' if value is None else f'{value:.2f}'

    def group(system, workload, mode):
        return [r for r in raw['rows'] if r['system']==system and r['workload']==workload
                and r['mode']==mode and not r.get('excluded')]

    def metric(system, workload, mode, key):
        rows = group(system, workload, mode)
        if len(rows) != (20 if mode=='warm' else 10) or any(r['status']!='VALID' for r in rows):
            return 'Withheld: incomplete or failed series'
        return fmt(stats([r.get(key) for r in rows])['median'])

    measured = [r for r in raw['rows'] if r['mode'] in ('cold','warm') and not r.get('excluded')]
    failures = [r for r in measured if r['status']!='VALID']
    stopped = [r for r in data['concurrency'] if r['stop'] or r['success_rate']!=1]
    sections = [f"# Mimic vs. Chrome: {data['checkpoint']}",
        f"Fresh builds on one Windows workstation, using the unchanged frozen workloads. "
        f"{sum(v=='VALID' for v in raw['gates'].values())}/{len(raw['gates'])} correctness gates "
        f"and {len(measured)-len(failures)}/{len(measured)} measured single-page attempts passed. "
        f"{len(stopped)} concurrency series stopped or contained a failure. All observations, "
        "including failed attempts and excluded warmups, remain in the data. These are controlled "
        "fixtures and do not establish general website compatibility.",
        "## Environment", table(['Item','Value'], [
            ['Platform', meta['os']], ['CPU', data['environment']['cpu']['Name']],
            ['RAM', fmt(data['environment']['ram_gib'])+' GiB'],
            ['Chrome', data['environment']['chrome_version']+' (headless=new)'],
            ['Mimic / Chrome V8', meta['mimic_v8']+' / '+data['environment']['chrome_v8']],
            ['Started / completed', data['started']+' / '+data['finished']]]),
        "Executable hashes are checked before every launch. This is an interactive workstation "
        "with background applications and antivirus enabled; cache and scheduler variation remain possible.",
        "## Startup and ready memory",
        "Ten fresh processes per runtime, alternating order, after an excluded warmup. Both "
        "answer the same Target.getTargets readiness probe. Summed process-tree RSS includes the "
        "initial page and can count shared pages more than once; it is not marginal Page memory.",
        table(['Runtime','CDP ready p50, ms','p95, ms','Ready RSS, MiB','Ready private bytes, MiB'],
            [[s.title(), fmt(startup[s]['cdp_ready_ms']['median']), fmt(startup[s]['cdp_ready_ms']['p95']),
              fmt(startup[s]['rss_mib']['median']), fmt(startup[s]['private_mib']['median'])] for s in SYSTEMS]),
        "## Warm execution and completion",
        "Twenty retained samples per workload/runtime. Each iteration creates a new Page and origin "
        "in the warm process. Execution includes invoking the workload and detecting its validated "
        "result. Completion also includes navigation; Page creation and teardown are excluded. Lower is better.",
        table(['Workload','Mimic execution, ms','Chrome execution, ms','Mimic completion, ms','Chrome completion, ms'],
            [[LABELS[w], *[metric(s,w,'warm',k) for k in ('execution_ms','completion_ms') for s in SYSTEMS]] for w in WORKLOADS]),
        "## Cold end-to-end completion",
        "Ten fresh-process samples per workload/runtime. Includes process startup, Page creation, "
        "navigation, execution, teardown and process exit. OS caches are not flushed. Server maintenance "
        "and temporary-profile removal are excluded. A failed series is withheld from comparisons.",
        table(['Workload','Mimic p50, ms','Chrome p50, ms'],
            [[LABELS[w], *[metric(s,w,'cold','total_cold_ms') for s in SYSTEMS]] for w in WORKLOADS]),
        "## CPU and memory during warm work",
        "Process-tree user plus kernel time per session; sampled peaks may miss short-lived allocations.",
        table(['Workload','Runtime','CPU, ms/session','Peak RSS, MiB','Peak private bytes, MiB'],
            [[LABELS[w],s.title(), *[fmt(stats([r.get(k) for r in group(s,w,'warm')])['median']*factor)
              for k,factor in (('session_cpu_s',1000),('peak_rss',1/2**20),('peak_private',1/2**20))]]
             for w in WORKLOADS for s in SYSTEMS]),
        "## Concurrent Pages",
        "Fresh process per level, one excluded warmup, then max(5, ceil(20/N)) measured waves. "
        "Throughput includes setup and teardown but excludes the separate 250 ms recovery wait. "
        "Every attempted level is shown. A stopped series is not a stable successful result.",
        table(['Workload','Runtime','Pages','Waves','Success','Sessions/s','Active RSS, MiB','Recovered RSS, MiB','Status'],
            [[LABELS[r['workload']],r['system'].title(),r['n'],r['waves'],f"{100*r['success_rate']:.1f}%",
              fmt(r['throughput']),fmt(r['rss_mib']),fmt(r['recovery_mib']),r['stop'] or 'Completed']
             for r in data['concurrency']]),
        "Recovery uses no forced collection. Allocator pools and shared runtime artifacts can remain "
        "resident; this table alone cannot prove leak absence. Private memory, marginal slopes, CPU "
        "and latency distributions are included in the numerical data.",
        "## Measurement boundaries",
        "Identical local fixtures, unique origins, HTTP cache disabled, full supported resource loading. "
        "The six fixtures cover static DOM, JavaScript/crypto, 3,000 DOM elements, asynchronous networking "
        "and Workers, React 18.3.1 and WebAssembly. Completion requires the exact expected result. "
        "No paint or network-idle delay is included. External high-resolution clocks, 5 ms result polling "
        "and 50 ms memory sampling are unchanged. Slow samples are retained; p95 from 10–20 samples "
        "is unstable. These measurements do not establish feature parity or costs on arbitrary sites.",
        "## Data and provenance",
        "[Numerical export](public-results.json) includes all summary metrics, numerical single-page "
        "and startup samples, concurrency outcomes and executable/harness hashes. "
        "[Full report](report.md) and [raw observations](raw.json) retain detailed evidence. "
        "[Optimization decisions](../../../docs/performance/optimization-campaign-20260914.md) distinguish "
        "these Windows observations from the primary paired Linux experiments."]
    (directory/'public-summary.md').write_text('\n\n'.join(sections)+'\n', encoding='utf-8')


if __name__ == '__main__':
    main()
