"""Windows-owned process trees: assign suspended parent before it can spawn children."""
import ctypes as C
from ctypes import wintypes as W
import subprocess
import threading
import time
import psutil

k=C.WinDLL('kernel32',use_last_error=True)
k.CreateJobObjectW.argtypes=[C.c_void_p,W.LPCWSTR];k.CreateJobObjectW.restype=W.HANDLE
k.AssignProcessToJobObject.argtypes=[W.HANDLE,W.HANDLE];k.AssignProcessToJobObject.restype=W.BOOL
k.QueryInformationJobObject.argtypes=[W.HANDLE,C.c_int,C.c_void_p,W.DWORD,C.c_void_p];k.QueryInformationJobObject.restype=W.BOOL
k.TerminateJobObject.argtypes=[W.HANDLE,W.UINT];k.CloseHandle.argtypes=[W.HANDLE]
k.SetInformationJobObject.argtypes=[W.HANDLE,C.c_int,C.c_void_p,W.DWORD];k.SetInformationJobObject.restype=W.BOOL
n=C.WinDLL('ntdll');n.NtResumeProcess.argtypes=[W.HANDLE];n.NtResumeProcess.restype=C.c_long
class Accounting(C.Structure):
    _fields_=[('user',C.c_int64),('kernel',C.c_int64),('period_user',C.c_int64),('period_kernel',C.c_int64),('faults',W.DWORD),('total',W.DWORD),('active',W.DWORD),('terminated',W.DWORD)]
class Pids(C.Structure):
    _fields_=[('assigned',W.DWORD),('count',W.DWORD),('pids',C.c_size_t*4096)]
class Limits(C.Structure):
    _fields_=[('process_time',C.c_int64),('job_time',C.c_int64),('flags',W.DWORD),('min_ws',C.c_size_t),('max_ws',C.c_size_t),('active_limit',W.DWORD),('affinity',C.c_size_t),('priority',W.DWORD),('scheduling',W.DWORD)]
class ExtendedLimits(C.Structure):
    _fields_=[('basic',Limits),('io',C.c_uint64*6),('process_memory',C.c_size_t),('job_memory',C.c_size_t),('peak_process',C.c_size_t),('peak_job',C.c_size_t)]

class Paging:
    """English PDH counter names work on localized Windows installations."""
    class Value(C.Structure):
        _fields_=[('status',W.DWORD),('value',C.c_double)]
    def __init__(self):
        self.dll=C.WinDLL('pdh');self.query=W.HANDLE();self.counter=W.HANDLE();self.value=None;self.error=None;self.last=0
        for name,types in [('PdhOpenQueryW',[W.LPCWSTR,C.c_size_t,C.POINTER(W.HANDLE)]),('PdhAddEnglishCounterW',[W.HANDLE,W.LPCWSTR,C.c_size_t,C.POINTER(W.HANDLE)]),('PdhCollectQueryData',[W.HANDLE]),('PdhGetFormattedCounterValue',[W.HANDLE,W.DWORD,C.c_void_p,C.c_void_p])]:
            getattr(self.dll,name).argtypes=types;getattr(self.dll,name).restype=W.LONG
        code=self.dll.PdhOpenQueryW(None,0,C.byref(self.query))
        if not code:code=self.dll.PdhAddEnglishCounterW(self.query,'\\Memory\\Pages Input/sec',0,C.byref(self.counter))
        if code:self.error=hex(code&0xffffffff)
        else:self.dll.PdhCollectQueryData(self.query)
    def read(self):
        if self.error:return None
        if time.perf_counter()-self.last>=1:
            self.last=time.perf_counter();self.dll.PdhCollectQueryData(self.query);v=self.Value()
            code=self.dll.PdhGetFormattedCounterValue(self.counter,0x200,None,C.byref(v))
            if code==0 and v.status in (0,1):self.value=v.value
        return self.value
PAGING=Paging()

class Tree:
    def __init__(self,command,env,log,interval=.05):
        self.job=k.CreateJobObjectW(None,None)
        if not self.job: raise C.WinError(C.get_last_error())
        limits=ExtendedLimits();limits.basic.flags=0x2000 # JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
        if not k.SetInformationJobObject(self.job,9,C.byref(limits),C.sizeof(limits)):raise C.WinError(C.get_last_error())
        self.start=time.perf_counter();si=subprocess.STARTUPINFO();si.dwFlags|=subprocess.STARTF_USESHOWWINDOW;si.wShowWindow=0
        self.process=subprocess.Popen(command,env=env,stdout=log,stderr=log,creationflags=0x4|subprocess.CREATE_NEW_CONSOLE,startupinfo=si)
        self.spawn_ms=(time.perf_counter()-self.start)*1000
        if not k.AssignProcessToJobObject(self.job,int(self.process._handle)):
            self.process.kill();raise C.WinError(C.get_last_error())
        if n.NtResumeProcess(int(self.process._handle))!=0:
            self.process.kill();raise RuntimeError('NtResumeProcess failed')
        self.samples=[];self.interval=interval;self.stop_event=threading.Event();self.lock=threading.Lock()
        self.thread=threading.Thread(target=self.loop,daemon=True);self.thread.start()
    def snapshot(self):
        with self.lock:
            a=Accounting();p=Pids()
            if not k.QueryInformationJobObject(self.job,1,C.byref(a),C.sizeof(a),None):raise C.WinError(C.get_last_error())
            if not k.QueryInformationJobObject(self.job,3,C.byref(p),C.sizeof(p),None):raise C.WinError(C.get_last_error())
            rss=private=0;missed=0
            for pid in p.pids[:p.count]:
                try:
                    m=psutil.Process(pid).memory_info();rss+=m.rss;private+=m.private
                except psutil.NoSuchProcess:pass
                except psutil.AccessDenied:missed+=1
            return dict(t=time.perf_counter(),rss=rss,private=private,user_s=a.user/1e7,kernel_s=a.kernel/1e7,
                        cpu_s=(a.user+a.kernel)/1e7,active_processes=a.active,total_processes=a.total,memory_access_denied=missed,
                        system_available=psutil.virtual_memory().available,system_memory_percent=psutil.virtual_memory().percent,
                        page_faults=a.faults,system_pages_input_s=PAGING.read(),paging_counter_error=PAGING.error)
    def loop(self):
        while not self.stop_event.is_set():
            try:self.samples.append(self.snapshot())
            except Exception as e:self.samples.append({'t':time.perf_counter(),'sampling_error':str(e)})
            self.stop_event.wait(self.interval)
    def close(self):
        self.stop_event.set();self.thread.join();k.TerminateJobObject(self.job,1);self.process.wait(timeout=10);k.CloseHandle(self.job)
