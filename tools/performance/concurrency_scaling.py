"""Additional scaling probes using the unchanged frozen workload runner."""
import argparse,concurrent.futures as futures,json,math,statistics,sys,threading,time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'benchmark'))
import run as frozen

def wave(runtime,work,n):
 servers=[frozen.Server(work) for _ in range(n)];barrier=threading.Barrier(n);release=threading.Event();done=[threading.Event() for _ in range(n)]
 start=time.perf_counter();before=runtime.tree.snapshot();stop=None
 try:
  with futures.ThreadPoolExecutor(max_workers=n) as pool:
   jobs=[pool.submit(frozen.execute,runtime,work,barrier,(event,release),server) for event,server in zip(done,servers)]
   try:
    while not all(event.is_set() for event in done):
     snap=runtime.tree.snapshot()
     if snap['system_available']<max(2*1024**3,.15*frozen.psutil.virtual_memory().total):stop='memory pressure'
     if time.perf_counter()-start>120:stop='wave timeout'
     if stop:barrier.abort();break
     time.sleep(.05)
    active=runtime.tree.snapshot();completed=time.perf_counter()
   finally:release.set()
   rows=[job.result() for job in jobs]
  elapsed=time.perf_counter()-start;after=runtime.tree.snapshot();time.sleep(.25);recovered=runtime.tree.snapshot()
  good=[row for row in rows if row['status']=='VALID'];latencies=sorted(row['completion_ms'] for row in good)
  return dict(n=n,workload=work,rows=rows,stop_reason=stop,before=before,active=active,after_teardown=after,after_recovery=recovered,elapsed_s=elapsed,completion_window_s=completed-start,throughput=len(good)/elapsed,p50_ms=statistics.median(latencies) if latencies else None,p95_ms=latencies[min(len(latencies)-1,math.ceil(len(latencies)*.95)-1)] if latencies else None)
 finally:
  release.set()
  for server in servers:server.close()

def main():
 p=argparse.ArgumentParser();p.add_argument('--before',type=Path,required=True);p.add_argument('--after',type=Path,required=True);p.add_argument('--chrome',type=Path,required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--repeats',type=int,default=3);p.add_argument('--cases',default='static:10,static:25,static:50,static:100,react:50');p.add_argument('--order',nargs='+',choices=['before','after','chrome']);a=p.parse_args();a.output.mkdir(parents=True,exist_ok=False)
 fingerprint,_=frozen.harness_fingerprint();data={'harness_sha256':fingerprint,'binaries':{name:{'path':str(path.resolve()),'sha256':frozen.digest(path)} for name,path in [('before',a.before),('after',a.after),('chrome',a.chrome)]},'waves':[]};blocked=set()
 def save(): (a.output/'raw.json').write_text(json.dumps(data,indent=2),encoding='utf-8')
 save()
 for case in a.cases.split(','):
  work,level=case.split(':');n=int(level)
  if work not in frozen.WORKLOADS or n<1:raise ValueError('invalid scaling case')
  for label in (a.order or (['before','after','chrome'] if n==10 else ['chrome','after','before'])):
   if (work,label) in blocked:continue
   binary=a.chrome if label=='chrome' else getattr(a,label)
   if frozen.digest(binary)!=data['binaries'][label]['sha256']:raise RuntimeError('binary changed')
   args=argparse.Namespace(mimic=binary,chrome=a.chrome,timeout=30)
   runtime=frozen.Runtime('chrome' if label=='chrome' else 'mimic',args)
   try:
    for iteration in range(-1,a.repeats):
     result=wave(runtime,work,n);result.update(label=label,iteration=iteration,excluded=iteration<0);data['waves'].append(result);save()
     print(label,work,n,iteration,'valid',sum(row['status']=='VALID' for row in result['rows']),'throughput',round(result['throughput'],2),'p95',result['p95_ms'],'stop',result['stop_reason'],flush=True)
     if result['stop_reason'] or any(row['status']!='VALID' for row in result['rows']):blocked.add((work,label));break
   finally:runtime.close()
 if frozen.harness_fingerprint()[0]!=fingerprint:raise RuntimeError('frozen harness changed')
 for record in data['binaries'].values():
  if frozen.digest(record['path'])!=record['sha256']:raise RuntimeError('binary changed')
 save()
if __name__=='__main__':main()
