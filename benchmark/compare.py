"""Compare frozen benchmark runs; refuse harness, fixture or environment drift."""
import argparse
import hashlib
import json
from pathlib import Path

def read(folder):
    raw=folder/'raw.json';manifest=json.loads((folder/'manifest.json').read_text(encoding='utf-8'))
    if hashlib.sha256(raw.read_bytes()).hexdigest()!=manifest['sha256']['raw.json']:raise ValueError('Raw data hash mismatch: '+str(folder))
    summary=folder/'summary.json'
    if hashlib.sha256(summary.read_bytes()).hexdigest()!=manifest['sha256']['summary.json']:raise ValueError('Summary hash mismatch: '+str(folder))
    return json.loads(raw.read_text(encoding='utf-8')),json.loads(summary.read_text(encoding='utf-8'))

def compare(before,after):
    a,sa=read(before);b,sb=read(after);ma,mb=a['metadata'],b['metadata']
    for field in ['harness_sha256','fixture_sha256','os','cpu','logical_cpus','physical_cpus','total_ram','power_mode','power_overlay']:
        if ma.get(field) is None or ma[field]!=mb.get(field):raise ValueError('Comparison refused: '+field+' differs or is absent')
    if ma['binaries']['chrome']['sha256']!=mb['binaries']['chrome']['sha256']:raise ValueError('Chrome binary changed')
    for field in ['smoke','timeout']:
        if ma['arguments'][field]!=mb['arguments'][field]:raise ValueError('Iteration/timeout policy changed')
    result=[]
    for x in sa['concurrency']:
        y=next((v for v in sb['concurrency'] if all(v[k]==x[k] for k in ['system','workload','n'])),None)
        if not y or x['stop'] or y['stop'] or x['success_rate']!=1 or y['success_rate']!=1:continue
        for metric in ['rss_mib','private_mib','cpu_per_success_s','p50','p95','throughput']:
            old,new=x.get(metric),y.get(metric)
            if old is None or new is None or old==0:continue
            result.append(dict(system=x['system'],workload=x['workload'],sessions=x['n'],metric=metric,before=old,after=new,change_percent=(new-old)/old*100))
    singles=[]
    for x in sa['single_session']:
        y=next((v for v in sb['single_session'] if all(v[k]==x[k] for k in ['system','workload','mode','metric'])),None)
        if not y or not x['n'] or x['n']!=y['n'] or x['median'] in (None,0) or y['median'] is None:continue
        matching=lambda r:all(r[k]==x[k] for k in ['system','workload','mode']) and not r.get('excluded')
        if any(r['status']!='VALID' for data in [a,b] for r in data['rows'] if r.get('mode') in ('cold','warm') and matching(r)):continue
        singles.append({k:x[k] for k in ['system','workload','mode','metric']}|dict(before=x['median'],after=y['median'],change_percent=(y['median']-x['median'])/x['median']*100))
    return dict(before_commit=ma['mimic_commit'],after_commit=mb['mimic_commit'],harness_sha256=ma['harness_sha256'],comparisons=result,single_session=singles,limitation='Same harness and recorded environment; background load and OS memory pressure can still vary. Negative throughput change is worse, negative latency/memory/CPU change is better.')

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('before',type=Path);p.add_argument('after',type=Path);p.add_argument('--output',type=Path,required=True);args=p.parse_args()
    if args.output.exists():p.error('Refusing to overwrite comparison artifact')
    args.output.write_text(json.dumps(compare(args.before,args.after),indent=2,ensure_ascii=False),encoding='utf-8')
