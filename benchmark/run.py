"""Reproducible Windows x64 browser-JavaScript baseline; production code is untouched."""
import argparse
import concurrent.futures as futures
import hashlib
import http.server
import json
import math
import os
from pathlib import Path
import platform
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
from websockets.sync.client import connect
import psutil
from windows_metrics import Tree

ROOT=Path(__file__).resolve().parents[1]
FIX=ROOT/'benchmark/fixtures'
PIN='152.0.7977.82'
WORKLOADS=['static','cpu','dom','async','react','wasm']
EXPECTED={
 'static':{'text':'baseline','count':1},
 'cpu':{'sum':sum((i*17)%997 for i in range(4000))*30},
 'dom':{'count':3000,'active':3000,'last':'node-2999','children':3,'round':'3','textLength':sum(len('node-'+str(i)) for i in range(3000))+7},
 'async':{'chain':101,'timer':17,'channel':23,'worker':62,'posted':'bench-message','fetch':7,'xhr':7},
 'react':{'count':200,'phase':'3','first':'Task 010','last':'Task 199209'},
 'wasm':{'sum':704982704}}
EXPECTED['cpu']['digest']=hashlib.sha256(str(EXPECTED['cpu']['sum']).encode()).hexdigest()
USED_PORTS=set()
PORT_LOCK=threading.Lock()

def ms(start):return (time.perf_counter()-start)*1000
def digest(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()
def harness_fingerprint():
    files=[p for p in (ROOT/'benchmark').iterdir() if p.suffix in ('.py','.ps1','.go','.txt')]
    files+=list(FIX.rglob('*'))
    entries={str(p.relative_to(ROOT/'benchmark')).replace('\\','/'):digest(p) for p in files if p.is_file()}
    return hashlib.sha256(json.dumps(entries,sort_keys=True).encode()).hexdigest(),entries
def shell(cmd):
    r=subprocess.run(cmd,capture_output=True,text=True,encoding='utf-8',errors='replace');return r.stdout.strip() if r.returncode==0 else 'UNAVAILABLE: '+r.stderr.strip()
def ps(command):return shell(['powershell','-NoProfile','-Command',command])
def free_port():
    with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]

class Server:
    def __init__(self,workload):
        self.requests=[];self.workload=workload;owner=self
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self,*args):pass
            def do_GET(self):
                start=time.perf_counter();path=self.path.split('?')[0]
                if path=='/':
                    vendor='<script src="/vendor/react.production.min.js"></script><script src="/vendor/react-dom.production.min.js"></script>' if workload=='react' else ''
                    data=('<!doctype html><html><head><meta charset="utf-8"><link rel="icon" href="data:,"></head><body data-workload="'+workload+'"><div id="root">baseline</div>'+vendor+'<script src="/workload.js"></script></body></html>').encode();typ='text/html'
                elif path=='/data.json':data=b'{"value":7}';typ='application/json'
                elif path=='/worker.js':data=b'onmessage=e=>postMessage(e.data*2);';typ='text/javascript'
                elif path in ['/workload.js','/vendor/react.production.min.js','/vendor/react-dom.production.min.js']:data=(FIX/path[1:]).read_bytes();typ='text/javascript'
                else:self.send_error(404);return
                self.send_response(200);self.send_header('Content-Type',typ);self.send_header('Content-Length',str(len(data)));self.send_header('Cache-Control','no-store');self.end_headers()
                try:self.wfile.write(data)
                except (BrokenPipeError,ConnectionResetError):pass
                owner.requests.append(dict(path=path,bytes=len(data),start=start,server_ms=ms(start)))
        with PORT_LOCK:
            # Windows can have a customized ephemeral range starting at 1024.
            # Port 0 may then allocate a browser-blocked port (e.g. 6000).
            # Use only high ports and never repeat an origin in this run.
            for port in range(49152,65535):
                if port in USED_PORTS:continue
                USED_PORTS.add(port)
                try:self.http=http.server.ThreadingHTTPServer(('127.0.0.1',port),Handler)
                except OSError:continue
                break
            else:raise RuntimeError('Safe local benchmark port range exhausted')
        self.port=self.http.server_address[1];self.url=f'http://127.0.0.1:{self.port}/'
        self.thread=threading.Thread(target=lambda:self.http.serve_forever(poll_interval=.02),daemon=True);self.thread.start()
    def close(self):self.http.shutdown();self.http.server_close();self.thread.join()

class CDP:
    def __init__(self,url,timeout=30):self.ws=connect(url,open_timeout=timeout,max_size=None);self.id=0;self.timeout=timeout
    def call(self,method,params=None):
        self.id+=1;self.ws.send(json.dumps(dict(id=self.id,method=method,params=params or {})));deadline=time.perf_counter()+self.timeout
        while True:
            reply=json.loads(self.ws.recv(timeout=max(.001,deadline-time.perf_counter())))
            if reply.get('id')!=self.id:continue
            if 'error' in reply:raise RuntimeError(str(reply['error']))
            return reply.get('result',{})
    def evaluate(self,expression):
        r=self.call('Runtime.evaluate',dict(expression=expression,returnByValue=True))
        if r.get('exceptionDetails'):raise RuntimeError(str(r['exceptionDetails']))
        return r.get('result',{}).get('value')
    def close(self):self.ws.close()

class Runtime:
    def __init__(self,system,args):
        self.system=system;self.args=args;self.temp=Path(tempfile.mkdtemp(prefix='mimic-benchmark-'));self.port=free_port();self.log=open(self.temp/'process.log','w');self.closed=False
        env=os.environ.copy();env['TEMP']=env['TMP']=str(self.temp)
        if system=='mimic':command=[str(args.mimic),'-listen',f'127.0.0.1:{self.port}','-engine','v8','-chrome','152','-browser-mode','headless']
        else:command=[str(args.chrome),'--headless=new',f'--user-data-dir={self.temp / "profile"}',f'--remote-debugging-port={self.port}','--remote-debugging-address=127.0.0.1','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-extensions','--disable-component-extensions-with-background-pages','about:blank']
        self.command=command;self.tree=Tree(command,env,self.log);self.version=None
        try:
            deadline=time.perf_counter()+30
            while time.perf_counter()<deadline:
                try:
                    with urllib.request.urlopen(f'http://127.0.0.1:{self.port}/json/version',timeout=.2) as r:self.version=json.load(r)
                    break
                except Exception:
                    if self.tree.process.poll() is not None:raise RuntimeError('process exited before CDP readiness')
                    time.sleep(.005)
            if not self.version:raise TimeoutError('CDP readiness')
            self.http_ready_ms=ms(self.tree.start);self.root=CDP(self.version['webSocketDebuggerUrl'])
            self.root.call('Target.getTargets')
            self.cdp_ready_ms=ms(self.tree.start)
            self.browser_version=self.root.call('Browser.getVersion') if system=='chrome' else self.version
            if system=='chrome' and self.version.get('Browser')!='Chrome/'+PIN:raise RuntimeError('Chrome pin mismatch: '+str(self.version))
            self.ready_memory=self.tree.snapshot()
        except BaseException:self.tree.close();self.log.close();raise
    def close(self):
        if self.closed:return {}
        self.closed=True;start=time.perf_counter();method='graceful';error=None
        try:
            if self.system=='chrome':
                try:self.root.call('Browser.close')
                except Exception:pass
            else:
                # The new hidden console belongs only to this benchmark process.
                code='import ctypes,sys; k=ctypes.WinDLL("kernel32"); k.FreeConsole(); assert k.AttachConsole(int(sys.argv[1])); k.SetConsoleCtrlHandler(None,True); assert k.GenerateConsoleCtrlEvent(0,0)'
                subprocess.run([sys.executable,'-c',code,str(self.tree.process.pid)],capture_output=True,timeout=3)
            self.tree.process.wait(timeout=5)
        except Exception as e:method='forced_job_termination';error=str(e)
        try:self.root.close()
        except Exception:pass
        final=self.tree.snapshot();self.tree.close();elapsed=ms(start);total_cold=ms(self.tree.start);self.log.close()
        # Only our TemporaryDirectory, verified by prefix and resolved parent.
        if self.temp.resolve().parent==Path(tempfile.gettempdir()).resolve() and self.temp.name.startswith('mimic-benchmark-'):
            shutil.rmtree(self.temp,ignore_errors=True)
        return dict(shutdown_ms=elapsed,shutdown_method=method,shutdown_error=error,final_accounting=final,total_cold_ms=total_cold)

def wait_value(cdp,expression,predicate,timeout):
    start=time.perf_counter();last=None
    while time.perf_counter()-start<timeout:
        last=cdp.evaluate(expression)
        if predicate(last):return last
        time.sleep(.005)
    raise TimeoutError('completion timeout; last='+repr(last))

def create_page(runtime,server):
    t=time.perf_counter();control=CDP(runtime.version['webSocketDebuggerUrl'])
    target=control.call('Target.createTarget',dict(url='about:blank'))['targetId']
    page=CDP(f'ws://127.0.0.1:{runtime.port}/devtools/page/{target}')
    page.call('Network.enable');page.call('Network.setCacheDisabled',dict(cacheDisabled=True))
    return control,page,target,ms(t)

def execute(runtime,workload,barrier=None,hold=None,server=None):
    row=dict(system=runtime.system,workload=workload,status='ERROR',cache='HTTP disabled; unique origin; OS file cache uncontrolled',settle_ms=0)
    own_server=server is None
    server=server or Server(workload);row['url']=server.url;page=control=None;target=None;start=time.perf_counter()
    try:
        if barrier is None:row['before_session']=runtime.tree.snapshot()
        control,page,target,row['session_create_ms']=create_page(runtime,server)
        if barrier is None:row['after_session']=runtime.tree.snapshot()
        if barrier:barrier.wait(timeout=120)
        nav=time.perf_counter();row['workload_start']=nav
        if barrier is None:row['before_workload']=runtime.tree.snapshot()
        row['navigation_ack']=page.call('Page.navigate',dict(url=server.url));row['navigate_ack_ms']=ms(nav)
        if row['navigation_ack'].get('errorText'):raise RuntimeError('Page.navigate failed: '+row['navigation_ack']['errorText'])
        wait_value(page,'({ready:document.readyState,url:location.href,runner:typeof window.__benchRun})',lambda v:v and v['ready']=='complete' and v['url']==server.url and v['runner']=='function',runtime.args.timeout)
        row['navigation_ms']=ms(nav);js=time.perf_counter()
        page.evaluate('void window.__benchRun()')
        value=wait_value(page,'window.__bench',lambda v:v and v.get('done'),runtime.args.timeout)
        row['execution_ms']=ms(js);row['completion_ms']=ms(nav);row['result']=value;row['expected']=EXPECTED[workload]
        row['status']='VALID' if value.get('result')==EXPECTED[workload] and not value.get('error') else 'INVALID — semantic mismatch'
        if barrier is None:row['after_workload']=runtime.tree.snapshot()
        row['session_latency_ms']=ms(start)
    except Exception as e:
        row['error']=str(e);row['session_latency_ms']=ms(start)
        if page and runtime.system=='mimic':
            try:row['failure_trace']=page.call('Mimic.getTrace')
            except Exception as trace_error:row['trace_error']=str(trace_error)
        if barrier:
            try:barrier.abort()
            except Exception:pass
    finally:
        row['finished']=time.perf_counter()
        if hold:
            hold[0].set();hold[1].wait(timeout=180)
        teardown=time.perf_counter()
        if page:
            try:page.close()
            except Exception:pass
        if target and control:
            try:
                closed=control.call('Target.closeTarget',dict(targetId=target))
                if not closed.get('success'):raise RuntimeError('Target.closeTarget returned false')
                deadline=time.perf_counter()+5
                while any(t['targetId']==target for t in control.call('Target.getTargets')['targetInfos']):
                    if time.perf_counter()>deadline:raise TimeoutError('target remains after teardown')
                    time.sleep(.005)
            except Exception as e:row['teardown_error']=str(e);row['status']='ERROR'
        if control:
            try:control.close()
            except Exception:pass
        row['teardown_ms']=ms(teardown)
        if barrier is None:row['after_teardown']=runtime.tree.snapshot()
        row['server_requests']=server.requests
        if own_server:server.close()
    if 'before_workload' in row and 'after_workload' in row:
        a,b=row['before_workload'],row['after_workload'];row['workload_cpu_s']=b['cpu_s']-a['cpu_s'];row['workload_user_s']=b['user_s']-a['user_s'];row['workload_kernel_s']=b['kernel_s']-a['kernel_s'];row['average_cpu_percent']=100*row['workload_cpu_s']/(row['completion_ms']/1000)
    return row

def samples_summary(samples):
    valid=[s for s in samples if 'rss' in s]
    peaks=[100*max(0,b['cpu_s']-a['cpu_s'])/(b['t']-a['t']) for a,b in zip(valid,valid[1:]) if b['t']-a['t']>=.025]
    return dict(peak_rss=max((s['rss'] for s in valid),default=0),peak_private=max((s['private'] for s in valid),default=0),peak_cpu_percent=max(peaks,default=0),min_system_available=min((s['system_available'] for s in valid),default=0),sampling_errors=[s for s in samples if 'sampling_error' in s])

def metadata(args):
    fingerprint,files=harness_fingerprint()
    return dict(harness_sha256=fingerprint,harness_files=files,harness_commit=shell(['git','log','-1','--format=%H','--','benchmark']),date=__import__('datetime').datetime.now().astimezone().isoformat(),os=platform.platform(),machine=platform.machine(),python=sys.version,
        cpu=ps('Get-CimInstance Win32_Processor | Select-Object Name,NumberOfCores,NumberOfLogicalProcessors | ConvertTo-Json -Compress'),logical_cpus=psutil.cpu_count(),physical_cpus=psutil.cpu_count(logical=False),total_ram=psutil.virtual_memory().total,
        power_mode=ps('powercfg /getactivescheme'),power_overlay=ps('$s=\'[DllImport("powrprof.dll")] public static extern uint PowerGetEffectiveOverlayScheme(out Guid g);\'; Add-Type -MemberDefinition $s -Name P -Namespace B; $g=[Guid]::Empty; [B.P]::PowerGetEffectiveOverlayScheme([ref]$g); $g.ToString()'),
        antivirus=ps('Get-CimInstance -Namespace root/SecurityCenter2 -ClassName AntiVirusProduct | Select-Object displayName,productState | ConvertTo-Json -Compress'),
        background_processes=sorted({p.info['name'] for p in psutil.process_iter(['name']) if p.info['name']}),background_caveat='Interactive workstation; antivirus and other processes remain enabled and may affect results.',
        mimic_commit=shell(['git','rev-parse','HEAD']),git_status=shell(['git','status','--short']),go=shell(['go','version']),mimic_build=shell(['go','version','-m',str(args.mimic)]),mimic_v8=shell(['go','run','./benchmark/v8_version.go']),
        binaries={s:dict(path=str(p),sha256=digest(p)) for s,p in [('chrome',args.chrome),('mimic',args.mimic)]},target=json.loads((ROOT/'chrome/152/target.json').read_text()),
        fixture_sha256={str(p.relative_to(FIX)):digest(p) for p in FIX.rglob('*') if p.is_file()},arguments=vars(args)|{'chrome':str(args.chrome),'mimic':str(args.mimic),'output':str(args.output)},
        timer='time.perf_counter (Windows QPC)',readiness_probe='WebSocket Target.getTargets reply',cpu_method='Windows Job Object accounting, including exited descendants; percent of one logical CPU',memory='sum of instantaneous Windows working sets/private bytes for job members, sampled every 50 ms; shared pages may be counted more than once')

def save(data,args):
    def portable(value):
        if isinstance(value,dict):return {k:portable(v) for k,v in value.items()}
        if isinstance(value,list):return [portable(v) for v in value]
        if isinstance(value,str):
            for path,label in [(ROOT,'<repo>'),(ROOT.parent,'<workspace-parent>'),(Path.home(),'<user>')]:
                for prefix in [str(path),str(path).replace('\\','/')]:value=value.replace(prefix,label)
        return value
    args.output.mkdir(parents=True,exist_ok=True);p=args.output/'raw.json';tmp=p.with_suffix('.json.tmp');tmp.write_text(json.dumps(portable(data),indent=2,ensure_ascii=False),encoding='utf-8')
    for attempt in range(100):
        try:tmp.replace(p);break
        except PermissionError:
            if attempt==99:raise
            time.sleep(.1)

def startup_calibration(data,args):
    """Common readiness probe, with interleaved systems and excluded warmup."""
    if data.get('startup_calibration',{}).get('complete'):return
    section=dict(probe='WebSocket Target.getTargets reply',date=__import__('datetime').datetime.now().astimezone().isoformat(),rows=[])
    data['startup_calibration']=section
    for iteration in range(-1,1 if args.smoke else 10):
        for system in (['chrome','mimic'] if iteration%2 else ['mimic','chrome']):
            runtime=Runtime(system,args)
            row=dict(system=system,iteration=iteration,excluded=iteration<0,process_start_ms=runtime.tree.spawn_ms,http_ready_ms=runtime.http_ready_ms,cdp_ready_ms=runtime.cdp_ready_ms,ready_memory=runtime.ready_memory,command=runtime.command)
            row.update(runtime.close());section['rows'].append(row);save(data,args)
    section['complete']=True;save(data,args)

def main():
    p=argparse.ArgumentParser();p.add_argument('--chrome',type=Path);p.add_argument('--mimic',type=Path,default=ROOT/'.build/mimic-benchmark.exe');p.add_argument('--output',type=Path,default=ROOT/'benchmark/results');p.add_argument('--smoke',action='store_true');p.add_argument('--gate-only',action='store_true');p.add_argument('--resume',action='store_true');p.add_argument('--timeout',type=float,default=30);p.add_argument('--skip-build',action='store_true');args=p.parse_args()
    if sys.platform!='win32' or platform.machine().lower() not in ('amd64','x86_64'):p.error('Windows x64 required')
    if not args.chrome:
        candidates=[ROOT/'compatibility/.chrome-for-testing'/PIN/'chrome-win64/chrome.exe',ROOT.parent/'mimic-cleanup-private-archive-20260908/compatibility/.chrome-for-testing'/PIN/'chrome-win64/chrome.exe']
        args.chrome=next((x for x in candidates if x.exists()),None)
    if not args.chrome or not args.chrome.exists():p.error('Supply --chrome PATH to exact Chrome '+PIN)
    if (args.output/'raw.json').exists() and not args.resume:p.error('Output already contains raw.json; choose a new --output or --resume. Baselines are not overwritten.')
    if not args.skip_build:subprocess.run(['go','build','-o',str(args.mimic),'./cmd/mimic'],cwd=ROOT,check=True)
    data=dict(schema_version=1,metadata=metadata(args),expected=EXPECTED,rows=[],runs=[],concurrency=[],gates={},notes=[])
    if args.resume:
        previous=json.loads((args.output/'raw.json').read_text(encoding='utf-8'))
        for system in ['chrome','mimic']:
            if previous['metadata']['binaries'][system]['sha256']!=data['metadata']['binaries'][system]['sha256']:raise RuntimeError('Cannot resume with different binary: '+system)
        if previous['metadata']['fixture_sha256']!=data['metadata']['fixture_sha256']:raise RuntimeError('Cannot resume with changed fixtures')
        if previous['metadata'].get('harness_sha256')!=data['metadata']['harness_sha256']:raise RuntimeError('Cannot resume with a changed harness')
        if previous['metadata']['arguments']['smoke']!=args.smoke or previous['metadata']['arguments']['timeout']!=args.timeout:raise RuntimeError('Cannot resume with different iteration or timeout policy')
        previous.setdefault('resumptions',[]).append(data['metadata']);data=previous
        data['notes'].append('Resumed after checkpoint interruption. Complete series retained; incomplete series re-run in a fresh runtime. Interrupted observations retained with exclusion_reason, separately summarized.')
    save(data,args)
    # Dedicated excluded warmup also acts as mandatory independent correctness gate.
    for system in ['chrome','mimic']:
        if args.resume and all(system+'/'+w in data['gates'] for w in WORKLOADS):continue
        runtime=Runtime(system,args)
        try:
            data['metadata'][system+'_version']=runtime.browser_version
            for work in WORKLOADS:
                row=execute(runtime,work);row.update(mode='gate',iteration=0,excluded=True);data['rows'].append(row);data['gates'][system+'/'+work]=row['status'];print('GATE',system,work,row['status'],row.get('error',row.get('result')),flush=True);save(data,args)
        finally:runtime.close()
    if args.gate_only:return
    valid=[w for w in WORKLOADS if all(data['gates'][s+'/'+w]=='VALID' for s in ['chrome','mimic'])]
    # Alternate system order for each workload/mode. All samples retained.
    for wi,work in enumerate(WORKLOADS):
        if work not in valid:
            data['notes'].append(work+': INVALID comparison; no timed repetitions because correctness gate failed');save(data,args);continue
        for mode,count in [('cold',1 if args.smoke else 10),('warm',2 if args.smoke else 20)]:
            # Failed semantic gates still get repeated diagnostics, not speed claims.
            for system in (['mimic','chrome'] if wi%2 else ['chrome','mimic']):
                prior=[r for r in data['rows'] if r.get('system')==system and r.get('workload')==work and r.get('mode')==mode and not r.get('excluded')]
                if len(prior)==count:continue
                if prior:
                    for row in prior:row['excluded']=True;row['exclusion_reason']='interrupted incomplete series; restarted with fresh runtime'
                runtime=None
                try:
                    if mode=='warm':runtime=Runtime(system,args)
                    for iteration in range(-1,count):
                        server=Server(work)
                        if mode=='cold':runtime=Runtime(system,args)
                        sample_start=len(runtime.tree.samples);before=runtime.tree.snapshot();row=execute(runtime,work,server=server);after=runtime.tree.snapshot()
                        row.update(mode=mode,iteration=iteration,excluded=iteration<0,process_start_ms=runtime.tree.spawn_ms if mode=='cold' else None,cdp_ready_ms=runtime.cdp_ready_ms if mode=='cold' else None,http_ready_ms=runtime.http_ready_ms if mode=='cold' else None,runtime_initialization_ms=runtime.cdp_ready_ms-runtime.tree.spawn_ms if mode=='cold' else None,ready_memory=runtime.ready_memory,
                                   session_cpu_s=after['cpu_s']-before['cpu_s'],session_user_s=after['user_s']-before['user_s'],session_kernel_s=after['kernel_s']-before['kernel_s'])
                        samples=runtime.tree.samples[sample_start:]+[row[k] for k in ['before_session','after_session','before_workload','after_workload','after_teardown'] if k in row];samples.sort(key=lambda x:x['t']);row.update(samples_summary(samples));row['samples']=samples
                        if mode=='cold':row['lifecycle_peak_rss']=samples_summary(runtime.tree.samples)['peak_rss']
                        if mode=='cold':row.update(runtime.close());runtime=None
                        else:
                            recovery=time.perf_counter();time.sleep(.25);row['after_recovery']=runtime.tree.snapshot();row['recovery_ms']=ms(recovery)
                        server.close()
                        data['rows'].append(row);save(data,args)
                        print(mode,system,work,iteration,row['status'],round(row.get('completion_ms',0),2),flush=True)
                finally:
                    if runtime:
                        data['runs'].append(dict(system=system,workload=work,mode=mode,after_repeated_iterations=runtime.tree.snapshot(),shutdown=runtime.close()));save(data,args)
    for work in ['static','cpu','react']:
        if work not in valid:data['notes'].append('Concurrency '+work+' skipped: semantic gate failed');continue
        for level in ([1,5] if args.smoke else [1,5,10,25,50,100]):
            for system in ['chrome','mimic']:
                matching=[x for x in data['concurrency'] if x['system']==system and x['workload']==work and x['n']==level]
                if matching:
                    old=matching[-1];required=1 if args.smoke else max(5,math.ceil(20/level))
                    if old.get('stop_reason') or len([w for w in old['waves'] if not w['excluded']])==required:continue
                    data.setdefault('interrupted_concurrency',[]).extend(matching)
                    data['concurrency']=[x for x in data['concurrency'] if x not in matching]
                if any(x['system']==system and x['workload']==work and x.get('stop_reason') for x in data['concurrency']):continue
                runtime=Runtime(system,args);entry=dict(system=system,workload=work,n=level,waves=[],fixed_memory=runtime.ready_memory);data['concurrency'].append(entry)
                try:
                    waves=1 if args.smoke else max(5,math.ceil(20/level))
                    for wave in range(-1,waves):
                        servers=[Server(work) for _ in range(level)]
                        before=runtime.tree.snapshot();idx=len(runtime.tree.samples);barrier=threading.Barrier(level);release=threading.Event();finished=[threading.Event() for _ in range(level)];start=time.perf_counter();paging_since=None
                        with futures.ThreadPoolExecutor(max_workers=level) as pool:
                            jobs=[pool.submit(execute,runtime,work,barrier,(done,release),server) for done,server in zip(finished,servers)]
                            stop=None
                            while not all(x.is_set() for x in finished):
                                current=runtime.tree.snapshot()
                                if current.get('system_pages_input_s') is not None and current['system_pages_input_s']>1024:
                                    paging_since=paging_since or time.perf_counter()
                                    if time.perf_counter()-paging_since>=3:stop='sustained paging (>1024 pages/s for 3 seconds)';barrier.abort();release.set();break
                                else:paging_since=None
                                if current['system_available']<max(2*1024**3,.15*psutil.virtual_memory().total):stop='memory pressure (<15% or 2 GiB available)';barrier.abort();release.set();break
                                if time.perf_counter()-start>180:stop='wave timeout';barrier.abort();release.set();break
                                time.sleep(.05)
                            completed=time.perf_counter();active=runtime.tree.snapshot();release.set();rows=[j.result() for j in jobs]
                        after=runtime.tree.snapshot();elapsed=time.perf_counter()-start;samples=runtime.tree.samples[idx:]
                        recovery=time.perf_counter();time.sleep(.25);recovered=runtime.tree.snapshot();recovery_ms=ms(recovery)
                        for server in servers:server.close()
                        r=dict(wave=wave,excluded=wave<0,rows=rows,before_wave=before,active_memory=active,after_teardown=after,after_recovery=recovered,recovery_ms=recovery_ms,cpu_s=after['cpu_s']-before['cpu_s'],user_s=after['user_s']-before['user_s'],kernel_s=after['kernel_s']-before['kernel_s'],elapsed_s=elapsed,throughput=sum(x['status']=='VALID' for x in rows)/elapsed,
                               completion_window_s=completed-start,success_rate=sum(x['status']=='VALID' for x in rows)/level,samples=samples,**samples_summary(samples))
                        entry['waves'].append(r)
                        if any(x['status']!='VALID' for x in rows):stop=stop or 'non-zero failure rate'
                        if stop:entry['stop_reason']=stop
                        save(data,args);print('density',system,work,level,wave,'success',r['success_rate'],'RSS MiB',round(active['rss']/2**20),stop or '',flush=True)
                        if stop:break
                finally:entry['shutdown']=runtime.close();save(data,args)
    startup_calibration(data,args)
    if harness_fingerprint()[0]!=data['metadata']['harness_sha256']:raise RuntimeError('Harness changed during measurement; baseline rejected')
    data['finished']=__import__('datetime').datetime.now().astimezone().isoformat();save(data,args)
    subprocess.run([sys.executable,str(ROOT/'benchmark/report.py'),str(args.output/'raw.json')],check=True)

if __name__=='__main__':main()
