"""All tables, conclusions and figures are derived from raw observations."""
import csv
import hashlib
import json
import math
from pathlib import Path
import statistics as st
import sys
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

MIB=2**20
def quantile(values,p):
    a=sorted(values);x=(len(a)-1)*p;lo=math.floor(x);hi=math.ceil(x);return a[lo]+(a[hi]-a[lo])*(x-lo)
def stats(values):
    a=[float(x) for x in values if x is not None]
    if not a:return dict(n=0,median=None,p50=None,p95=None,p99=None,min=None,max=None,sd=None,cv=None)
    mean=st.mean(a);sd=st.stdev(a) if len(a)>1 else 0
    return dict(n=len(a),median=st.median(a),p50=st.median(a),p95=quantile(a,.95) if len(a)>=10 else None,p99=quantile(a,.99) if len(a)>=100 else None,min=min(a),max=max(a),sd=sd,cv=sd/mean if mean else None)
def f(x):return '—' if x is None else f'{x:.2f}'
def table(headers,rows):return '\n'.join(['| '+' | '.join(headers)+' |','|'+'|'.join(['---']*len(headers))+'|']+['| '+' | '.join(str(x).replace('|','/').replace('\n',' ') for x in row)+' |' for row in rows])
def med(rows,key):return stats([r.get(key) for r in rows])['median']
def yaml_lines(value,indent=0):
    prefix=' '*indent;lines=[]
    if isinstance(value,dict):
        for key,item in value.items():
            if isinstance(item,(dict,list)) and item:lines.append(prefix+key+':');lines+=yaml_lines(item,indent+2)
            else:lines.append(prefix+key+': '+json.dumps(item,ensure_ascii=False))
    elif isinstance(value,list):
        for item in value:
            if isinstance(item,(dict,list)):lines.append(prefix+'-');lines+=yaml_lines(item,indent+2)
            else:lines.append(prefix+'- '+json.dumps(item,ensure_ascii=False))
    return lines

def generate(path):
    d=json.loads(path.read_text(encoding='utf-8'));out=path.parent;m=d['metadata'];measured=[r for r in d['rows'] if not r.get('excluded') and r.get('mode') in ('cold','warm')]
    # Recompute derived CPU peaks: adjacent control snapshots can be microseconds
    # apart while Windows CPU accounting ticks are much coarser.
    for row in measured:
        samples=sorted((s for s in row.get('samples',[]) if 'cpu_s' in s),key=lambda s:s['t'])
        peaks=[100*max(0,b['cpu_s']-a['cpu_s'])/(b['t']-a['t']) for a,b in zip(samples,samples[1:]) if b['t']-a['t']>=.025]
        row['peak_cpu_percent']=max(peaks,default=0)
        if row.get('mode')=='cold' and row.get('final_accounting'):
            for k in ['cpu_s','user_s','kernel_s']:row['cold_'+k]=row['final_accounting'][k]
    groups={}
    for r in measured:groups.setdefault((r['system'],r['workload'],r['mode']),[]).append(r)
    metrics=['process_start_ms','http_ready_ms','cdp_ready_ms','runtime_initialization_ms','session_create_ms','navigate_ack_ms','navigation_ms','execution_ms','completion_ms','total_cold_ms','teardown_ms','shutdown_ms','cold_cpu_s','cold_user_s','cold_kernel_s','session_cpu_s','session_user_s','session_kernel_s','workload_cpu_s','workload_user_s','workload_kernel_s','average_cpu_percent','peak_cpu_percent','peak_rss','peak_private','lifecycle_peak_rss']
    summary=[]
    for key,rows in groups.items():
        for metric in metrics:summary.append(dict(system=key[0],workload=key[1],mode=key[2],metric=metric,**stats([r.get(metric) for r in rows])))
    with (out/'summary.csv').open('w',newline='',encoding='utf-8') as file:
        w=csv.DictWriter(file,fieldnames=list(summary[0]) if summary else ['system']);w.writeheader();w.writerows(summary)
    flat=[]
    for r in measured:flat.append({k:v for k,v in r.items() if v is None or isinstance(v,(str,int,float,bool))})
    with (out/'iterations.csv').open('w',newline='',encoding='utf-8') as file:
        w=csv.DictWriter(file,fieldnames=sorted({k for r in flat for k in r}));w.writeheader();w.writerows(flat)
    scaling=[]
    for entry in d['concurrency']:
        waves=[x for x in entry['waves'] if not x['excluded']];diagnostic=False
        if not waves and entry.get('stop_reason') and entry['waves']:waves=entry['waves'][-1:];diagnostic=True
        if not waves:continue
        rows=[r for wave in waves for r in wave['rows']];successful=sum(r['status']=='VALID' for r in rows)
        lat=stats([r['session_latency_ms'] for r in rows]);rss=st.median(x['active_memory']['rss'] for x in waves)/MIB
        scaling.append(dict(system=entry['system'],workload=entry['workload'],n=entry['n'],waves=0 if diagnostic else len(waves),diagnostic_warmup_failure=diagnostic,attempts=len(rows),success_rate=successful/len(rows),rss_mib=rss,rss_per_session_mib=rss/entry['n'],peak_rss_mib=max(max(x['peak_rss'],x['active_memory']['rss'],x['after_teardown']['rss']) for x in waves)/MIB,private_mib=st.median(x['active_memory']['private'] for x in waves)/MIB,
          cpu_s=sum(x['cpu_s'] for x in waves),cpu_per_success_s=sum(x['cpu_s'] for x in waves)/successful if successful else None,cpu_percent=100*sum(x['cpu_s'] for x in waves)/sum(x['elapsed_s'] for x in waves),throughput=successful/sum(x['elapsed_s'] for x in waves),
          p50=lat['p50'],p95=lat['p95'],p99=lat['p99'],min=lat['min'],max=lat['max'],sd=lat['sd'],cv=lat['cv'],recovery_mib=st.median(x.get('after_recovery',x['after_teardown'])['rss'] for x in waves)/MIB,
          active_increment_per_session_mib=st.median((x['active_memory']['rss']-x.get('before_wave',entry['fixed_memory'])['rss'])/entry['n']/MIB for x in waves),
          fixed_ready_mib=entry['fixed_memory']['rss']/MIB,stop=entry.get('stop_reason','')))
    if scaling:
        with (out/'concurrency.csv').open('w',newline='',encoding='utf-8') as file:w=csv.DictWriter(file,fieldnames=list(scaling[0]));w.writeheader();w.writerows(scaling)
    fits=[];marginal=[]
    for work in sorted({x['workload'] for x in scaling}):
        for system in ['chrome','mimic']:
            a=sorted([x for x in scaling if x['system']==system and x['workload']==work and x['success_rate']==1 and not x['stop']],key=lambda x:x['n'])
            for lo,hi in zip(a,a[1:]):marginal.append(dict(system=system,workload=work,lo=lo['n'],hi=hi['n'],delta_mib=hi['rss_mib']-lo['rss_mib'],per_session_mib=(hi['rss_mib']-lo['rss_mib'])/(hi['n']-lo['n'])))
            if len(a)>=2:
                slope,intercept=st.linear_regression([x['n'] for x in a],[x['rss_mib'] for x in a]);res=sum((x['rss_mib']-(intercept+slope*x['n']))**2 for x in a);total=sum((x['rss_mib']-st.mean(x['rss_mib'] for x in a))**2 for x in a)
                fits.append(dict(system=system,workload=work,intercept_mib=intercept,marginal_mib=slope,r2=1-res/total if total else None,levels=len(a),max_n=max(x['n'] for x in a)))
    (out/'summary.json').write_text(json.dumps(dict(single_session=summary,concurrency=scaling,marginal=marginal,fits=fits),indent=2),encoding='utf-8')
    sections=['# Mimic V8 and Chrome 152: Windows x64 baseline',
      'This is '+('a smoke run, insufficient for conclusions.' if m['arguments']['smoke'] else 'a measurement of the build identified below, using the frozen benchmark suite.')+(" The original version's correctness check is saved separately in `pre-fix/raw.json`." if (out/'pre-fix/raw.json').exists() else ''),
      '## Environment',table(['Parameter','Value'],[
        ['Start / end date',m['date']+' / '+d.get('finished','NOT FINISHED')],['Chrome',m.get('chrome_version',{})],['Chromium',str(m['target']['main_branch_revision'])+' / '+m['target']['chromium_commit']],['Mimic commit',m['mimic_commit']],['Mimic V8',m.get('mimic_v8','unknown')],['OS',m['os']],['CPU',m['cpu']],['CPU physical / logical',f"{m['physical_cpus']} / {m['logical_cpus']}"],['RAM GiB',f(m['total_ram']/2**30)],['Power plan',m['power_mode'].replace('\u0047\u0055\u0049\u0044 \u0441\u0445\u0435\u043c\u044b \u043f\u0438\u0442\u0430\u043d\u0438\u044f:', 'Power Scheme GUID:').replace('\u0421\u0431\u0430\u043b\u0430\u043d\u0441\u0438\u0440\u043e\u0432\u0430\u043d\u043d\u0430\u044f', 'Balanced')],['Power overlay',m['power_overlay']],['Antivirus',m['antivirus']],['Chrome SHA-256',m['binaries']['chrome']['sha256']],['Mimic SHA-256',m['binaries']['mimic']['sha256']]]),
      'The workstation was not dedicated exclusively to the test: background applications and antivirus were enabled. Process lists, tool versions, arguments, and fixture/binary SHA-256 hashes are in raw.json.',
      '## Methodology and correctness',
      'Both systems create a fresh page and a unique loopback origin. HTTP cache is disabled, responses use no-store, and cookies are not used. Contexts, transport, and the process persist in warm runs; page state is not reused. This provides isolation for this controlled corpus, not a test of tenant/security isolation. Cold runs create a new process and profile. Windows file, DLL, and OS DNS caches are not cleared; “cold” means a fresh process, not a cold disk. Warm HTTP-cache behavior was not measured.',
      'Navigation completes only at the exact URL, with document.readyState === "complete" and __benchRun present. Both systems then explicitly invoke __benchRun; completion requires done and an exact match of the deterministic result. Settle = 0. Paint/networkidle are not awaited. Chrome uses headless=new; results do not automatically apply to headful mode.',
      "External perf_counter/QPC clock; 5 ms polling plus CDP/OS scheduler latency. navigation_ms includes HTML parsing and script loading; execution_ms includes application startup/execution and marker detection. These are not isolated JIT or pure JavaScript timings. The page's js_ms is diagnostic: Mimic's virtual time is often zero.",
      'The process starts suspended, is assigned to a Windows Job Object, and then resumes. process_start_ms measures the process-creation call; CDP readiness runs from the start of creation to the protocol response. runtime_initialization_ms is the remainder between them; internal V8 phases are not separately instrumented. Cold total includes page creation, execution, teardown, and process exit; temporary-profile removal and local-server maintenance are excluded.',
      'CPU is user+kernel for the entire Job Object, including exited descendants. Working set/private bytes sum all current Job Object members every 50 ms and at checkpoints. Short memory peaks may be missed, and shared DLL pages may be counted multiple times. CPU % is relative to one logical core; 100% of the machine = '+str(m['logical_cpus']*100)+'%. CPU peaks are sensitive to Windows counter granularity.',
      'One correctness check and one initial warmup per series are explicitly excluded. Main series: 10 cold and 20 warm runs, without removing slow observations. p95 is published at n≥10 and p99 at n≥100; empirical quantiles use linear interpolation, and tails at n=10–20 are particularly unstable. SD, CV, and min/max for all series are available in summary.csv. Speedups are not aggregated into a single ratio.',
      table(['System / workload','Correctness gate'],sorted(d['gates'].items()))]
    startup=[r for r in d.get('startup_calibration',{}).get('rows',[]) if not r['excluded']]
    if startup:
        sections+=['### CDP readiness: separate series with an identical probe',
          'In the final series, both systems respond to Target.getTargets after the WebSocket handshake. Each uses 10 fresh processes, alternating system order; warmup is excluded. Date: '+d['startup_calibration']['date']+'. '+('In the original series below, Mimic readiness was recorded after the handshake and Chrome readiness after Browser.getVersion. These original values are retained for audit; comparative readiness conclusions use only the new shared probe.' if not m.get('readiness_probe') else 'The main series uses the same shared probe.'),
          table(['System','n','CDP p50 ms','p95','Min','Max','SD','Ready RSS MiB'],[[s,z['n'],f(z['p50']),f(z['p95']),f(z['min']),f(z['max']),f(z['sd']),f(st.median(r['ready_memory']['rss'] for r in startup if r['system']==s)/MIB)] for s in ['mimic','chrome'] for z in [stats([r['cdp_ready_ms'] for r in startup if r['system']==s])]])]
    interrupted={}
    for row in d['rows']:
        if row.get('exclusion_reason'):interrupted.setdefault((row['system'],row['workload'],row['mode']),[]).append(row)
    if interrupted:
        sections+=['### Interrupted-series observations (retained without filtering by speed)',
          'A checkpoint write failure required restarting the incomplete series. These values are excluded from the final series with continuous process history; original observations are retained with exclusion_reason. Statistics for the interrupted portion are shown separately below.',
          table(['System','Workload','Mode','n','Completion p50 ms','Min','Max','SD'],[[s,w,mode,z['n'],f(z['median']),f(z['min']),f(z['max']),f(z['sd'])] for (s,w,mode),rows in interrupted.items() for z in [stats([r.get('completion_ms') for r in rows])]])]
    sections+=['## Cold startup (medians, ms)',table(['System','Workload','n','Process create','CDP ready','Runtime init','Cold total','Shutdown'],[[s,w,len(r),f(med(r,'process_start_ms')),f(med(r,'cdp_ready_ms')),f(med(r,'runtime_initialization_ms')),f(med(r,'total_cold_ms')),f(med(r,'shutdown_ms'))] for (s,w,mode),r in groups.items() if mode=='cold']),
      '## Warm session startup / teardown (medians)',table(['System','Workload','n','Create ms','Teardown ms','RSS after teardown MiB'],[[s,w,len(r),f(med(r,'session_create_ms')),f(med(r,'teardown_ms')),f(st.median(x['after_teardown']['rss'] for x in r)/MIB)] for (s,w,mode),r in groups.items() if mode=='warm']),
      '## Single-session workload latency (ms)',table(['System','Workload','Mode','n','Nav p50','Execution p50','Completion p50','p95','Min','Max','SD','CV'],[[s,w,mode,len(r),f(med(r,'navigation_ms')),f(med(r,'execution_ms')),f(z['median']),f(z['p95']),f(z['min']),f(z['max']),f(z['sd']),f(z['cv'])] for (s,w,mode),r in groups.items() for z in [stats([x.get('completion_ms') for x in r])]]),
      '## Single-session memory / CPU (medians)',table(['System','Workload','Mode','Before page MiB','After create MiB','Peak RSS MiB','Peak private MiB','CPU/session ms','CPU/workload ms'],[[s,w,mode,f(st.median(x['before_session']['rss'] for x in r)/MIB),f(st.median(x['after_session']['rss'] for x in r)/MIB),f(med(r,'peak_rss')/MIB),f(med(r,'peak_private')/MIB),f(med(r,'session_cpu_s')*1000),f(med(r,'workload_cpu_s')*1000)] for (s,w,mode),r in groups.items()]),
      '## Concurrency / density',
      'Each level uses a separate process, one excluded warmup, and max(5, ceil(20/N)) measured waves. Pages are created concurrently and all begin navigation after a barrier. Completed pages are held until the wave ends to measure simultaneous RSS. Throughput = successful sessions / time from create to final teardown, including the barrier and measurements but excluding HTTP-server setup/cleanup. Latency = create→completion, including barrier wait. This is batch throughput, not an optimized continuous request stream. Holding pages adds to throughput time but not latency. CPU per session = total wave CPU / successes; overlapping per-page CPU intervals are not summed.',
      'Stopping limits: any error, <15% or <2 GiB available RAM, >1024 pages input/s for 3 s, or a 180 s timeout. These protect the workstation; the highest passing level is a lower bound on capacity supported here, not proof of an absolute maximum.',
      'Mimic serializes CDP commands with a shared mutex. The measurement includes this behavior; its cost was not profiled. In stopped rows, RSS may have been sampled before all pages completed, and 0 waves means stopping during the excluded warmup. Such rows are diagnostic and excluded from stable-level fits/charts. Between waves, an additional 250 ms recovery, server cleanup, and checkpoint writing are outside batch throughput.',
      table(['System','Workload','N','Waves','Success %','RSS MiB','RSS/N MiB','Peak MiB','CPU/session ms','Sessions/s','p50 ms','p95 ms','p99 ms','Stop'],[[x['system'],x['workload'],x['n'],x['waves'],f(x['success_rate']*100),f(x['rss_mib']),f(x['rss_per_session_mib']),f(x['peak_rss_mib']),f(x['cpu_per_success_s']*1000 if x['cpu_per_success_s'] is not None else None),f(x['throughput']),f(x['p50']),f(x['p95']),f(x['p99']),x['stop']] for x in scaling]),
      '## Marginal RAM/session',
      'The finite difference between adjacent tested N values is measured and divided by ΔN; this is not a direct measurement of every N→N+1 step. The linear model is a descriptive OLS fit to medians of successful levels only. The intercept is an extrapolation; actual startup overhead includes the initial blank page. Total RSS includes processes and caches retained from earlier waves, and the number of waves depends on N. The fit therefore combines active-page costs with process history. RSS growth relative to the start of the same wave is also shown; it can include background activity and GC.',
      table(['System','Workload','N','(Active RSS − before wave RSS)/N MiB'],[[x['system'],x['workload'],x['n'],f(x['active_increment_per_session_mib'])] for x in scaling]),
      table(['System','Workload','N low→high','ΔRSS MiB','ΔRSS/ΔN MiB'],[[x['system'],x['workload'],f"{x['lo']}→{x['hi']}",f(x['delta_mib']),f(x['per_session_mib'])] for x in marginal]),
      table(['System','Workload','Fixed fit MiB','Marginal fit MiB','R²','Max stable N'],[[x['system'],x['workload'],f(x['intercept_mib']),f(x['marginal_mib']),f(x['r2']),x['max_n']] for x in fits]),
      '## Teardown / recovery',table(['System','Workload','N','Ready RSS MiB','RSS after waves MiB','Whole-series CPU s','CPU %'],[[x['system'],x['workload'],x['n'],f(x['fixed_ready_mib']),f(x['recovery_mib']),f(x['cpu_s']),f(x['cpu_percent'])] for x in scaling])]
    sections+=['## Local server (measured independently)',
      'HTTP handler time runs from the handler receiving a request to completion of response writing; it excludes TCP/server scheduler queues and network roundtrip. Before the main workload, the server is created outside the measured interval. Requests and bytes for each resource are recorded in raw.json.',
      table(['System','Workload','Mode','Requests','Server p50 ms','Server p95 ms','Server max ms'],[[s,w,mode,z['n'],f(z['p50']),f(z['p95']),f(z['max'])] for (s,w,mode),rows in groups.items() for z in [stats([q['server_ms'] for row in rows for q in row['server_requests']])]]),
      '## Validity of measured series',
      table(['System','Workload','Mode','Successes / attempts','Comparison use'],[[s,w,mode,str(sum(x['status']=='VALID' for x in rows))+'/'+str(len(rows)),'VALID' if all(x['status']=='VALID' for x in rows) else 'INVALID — error or semantic mismatch; no speed claim'] for (s,w,mode),rows in groups.items()])]
    worklist=sorted({x['workload'] for x in scaling})
    for metric,title,ylabel in [('rss_mib','total-rss','Total working set (MiB)'),('marginal','marginal-rss','Marginal working set (MiB/session)'),('throughput','throughput','Successful sessions / second'),('latency','latency','Session latency (ms)'),('cpu_percent','cpu','CPU (% of one logical core)')]:
        if not worklist:continue
        fig,axes=plt.subplots(1,len(worklist),figsize=(5*len(worklist),4.3),squeeze=False)
        for ax,work in zip(axes[0],worklist):
            for system in ['chrome','mimic']:
                a=sorted([x for x in scaling if x['system']==system and x['workload']==work and x['success_rate']==1 and not x['stop']],key=lambda x:x['n'])
                if metric=='marginal':
                    a=[x for x in marginal if x['system']==system and x['workload']==work];ax.plot([x['hi'] for x in a],[x['per_session_mib'] for x in a],'-o',label=system)
                elif metric=='latency':
                    for q,style in [('p50','-o'),('p95','--x')]:ax.plot([x['n'] for x in a],[x[q] if x[q] is not None else float('nan') for x in a],style,label=system+' '+q)
                else:ax.plot([x['n'] for x in a],[x[metric] for x in a],'-o',label=system)
            ax.set(title=work,xlabel='Concurrent sessions',ylabel=ylabel);ax.grid(alpha=.25);ax.legend()
        fig.tight_layout();fig.savefig(out/(title+'.png'),dpi=150);plt.close(fig);sections+=['!['+ylabel+']('+title+'.png)']
    faster={'mimic':[],'chrome':[]}
    for work in sorted({r['workload'] for r in measured}):
        a=groups.get(('mimic',work,'warm'),[]);b=groups.get(('chrome',work,'warm'),[])
        if a and b and all(r['status']=='VALID' for r in a+b):
            ma,mb=med(a,'completion_ms'),med(b,'completion_ms');faster['mimic' if ma<mb else 'chrome'].append(f'{work} ({f(ma)} / {f(mb)} ms Mimic/Chrome)')
    sections+=['## Engineering conclusions',
      '1. By median warm navigation→completion, Mimic is faster on: '+('; '.join(faster['mimic']) or 'none of the measured workloads')+'. '+('CDP readiness (shared probe, separate cold runs): '+('; '.join(s+' '+f(st.median(r['cdp_ready_ms'] for r in startup if r['system']==s))+' ms' for s in ['mimic','chrome'])) if startup else 'No comparable shared readiness probe was run.'),
      '2. Chrome is faster by the same metric on: '+('; '.join(faster['chrome']) or 'none of the measured workloads')+'.',
      '3. Fixed process overhead (CDP ready, including the initial page): '+('; '.join(s+' '+f(st.median(r['ready_memory']['rss'] for r in measured if r['system']==s)/MIB)+' MiB' for s in ['mimic','chrome']) if measured else 'not measured')+'. Startup latency and the OLS intercept are reported separately above.',
      '4. Estimated marginal RAM/session: '+('; '.join(x['system']+'/'+x['workload']+' '+f(x['marginal_mib'])+' MiB' for x in fits) or 'insufficient successful levels for a model')+'.',
      '5. Maximum stable N by workload: '+('; '.join(s+'/'+w+' '+str(max((x['n'] for x in scaling if x['system']==s and x['workload']==w and x['success_rate']==1 and not x['stop']),default=0)) for w in worklist for s in ['mimic','chrome']) or 'not measured')+'.',
      '6. Throughput at the highest common stable level:']
    claims=[]
    for work in worklist:
        levels=set(x['n'] for x in scaling if x['system']=='mimic' and x['workload']==work and x['success_rate']==1 and not x['stop'])&set(x['n'] for x in scaling if x['system']=='chrome' and x['workload']==work and x['success_rate']==1 and not x['stop'])
        if levels:
            n=max(levels);a=next(x for x in scaling if x['system']=='mimic' and x['workload']==work and x['n']==n);b=next(x for x in scaling if x['system']=='chrome' and x['workload']==work and x['n']==n)
            claims.append(f"{work}, N={n}: Mimic {f(a['throughput'])} and Chrome {f(b['throughput'])} successful sessions/s; RSS {f(a['rss_mib'])} and {f(b['rss_mib'])} MiB; CPU/session {f(a['cpu_per_success_s']*1000)} and {f(b['cpu_per_success_s']*1000)} ms.")
    sections+=claims+['7. Invalid comparisons in this run: '+(', '.join(k for k,v in d['gates'].items() if v!='VALID') or 'none at the correctness gate; individual iteration errors remain in raw.json')+'.',
      '8. Limitations: one busy workstation; headless Chrome; different V8 builds; HTTP cache disabled; limited corpus sizes and semantic checks; a synthetic React fixture, not a Next.js/production application. Polling and sampling add system load. RSS sums working sets, not unique physical RAM; no financial model of session cost is provided. Paging is a system-wide counter, not attribution of hard faults to a specific process. Mimic virtual-clock values are not used for comparisons.',
      '9. Supported scope: “On Windows x64, local controlled workloads passed result validation in Mimic V8 and Chrome '+m['target']['chrome_version']+'; a reproducible harness, raw observations, and separate latency, CPU, and memory metrics are published. '+(' '.join(claims[:1]))+'”',
      '10. The data do NOT support claims that “Mimic is X times faster than Chrome in general,” full browser compatibility, tenant-isolation security, an advantage in pure V8/JIT execution, gains on arbitrary sites, or monetary savings without an operating model.']
    if d['notes']:sections+=['## Run notes']+d['notes']
    (out/'report.md').write_text('\n\n'.join(sections)+'\n',encoding='utf-8')
    baseline=dict(benchmark='baseline',commit=m['mimic_commit'],harness_commit=m.get('harness_commit'),harness_sha256=m.get('harness_sha256'),date=m['date'],chrome=m['target']['chrome_version'],concurrency_levels=[1,5,10,25,50,100],units=dict(memory='MiB (summed working sets; private bytes separately)',cpu='seconds',latency='milliseconds',throughput='successful sessions/second'),cold=[],warm=[],concurrency=scaling,common_cdp_startup={s:stats([r['cdp_ready_ms'] for r in startup if r['system']==s]) for s in ['mimic','chrome']})
    for (s,w,mode),rows in groups.items():
        baseline[mode].append(dict(system=s,workload=w,iterations=len(rows),latency_ms=stats([r.get('completion_ms') for r in rows]),total_cold_ms=stats([r.get('total_cold_ms') for r in rows]),session_creation_ms=stats([r.get('session_create_ms') for r in rows]),rss_peak_mib=med(rows,'peak_rss')/MIB,private_bytes_peak_mib=med(rows,'peak_private')/MIB,cpu_per_session_s=med(rows,'session_cpu_s')))
    (out/'baseline.yaml').write_text('\n'.join(yaml_lines(baseline))+'\n',encoding='utf-8')
    artifacts={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file() and p.name!='manifest.json'}
    (out/'manifest.json').write_text(json.dumps(dict(sha256=artifacts,harness_sha256=m.get('harness_sha256')),indent=2),encoding='utf-8')

if __name__=='__main__':generate(Path(sys.argv[1]))
