"""Focused tests for measurement correctness, independent of production results."""
import os
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from windows_metrics import Tree
from report import stats
from compare import compare
from run import Server

class HarnessTests(unittest.TestCase):
    def test_origins_use_high_ports_and_are_never_reused(self):
        a=Server('static');first=a.port;a.close()
        b=Server('static')
        try:self.assertGreaterEqual(first,49152);self.assertGreaterEqual(b.port,49152);self.assertNotEqual(first,b.port)
        finally:b.close()
    def test_comparison_percent_and_harness_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            def fixture(name,rss,fingerprint='same'):
                folder=root/name;folder.mkdir()
                metadata={k:'same' for k in ['fixture_sha256','os','cpu','logical_cpus','physical_cpus','total_ram','power_mode','power_overlay']}
                metadata.update(harness_sha256=fingerprint,mimic_commit=name,binaries={'chrome':{'sha256':'pinned'}},arguments={'smoke':False,'timeout':30})
                (folder/'raw.json').write_text(json.dumps({'metadata':metadata,'rows':[]}))
                (folder/'summary.json').write_text(json.dumps({'single_session':[],'concurrency':[dict(system='mimic',workload='static',n=25,stop='',success_rate=1,rss_mib=rss)]}))
                (folder/'manifest.json').write_text(json.dumps({'sha256':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in folder.iterdir()}}))
                return folder
            before=fixture('before',1636);after=fixture('after',940)
            result=compare(before,after)['comparisons'][0]
            self.assertAlmostEqual(result['change_percent'],(940-1636)/1636*100)
            with self.assertRaisesRegex(ValueError,'harness_sha256'):compare(before,fixture('changed',940,'different'))
    def test_statistics_keep_slow_samples_and_suppress_small_tail(self):
        s=stats([1]*19+[100]);self.assertEqual(s['n'],20);self.assertEqual(s['max'],100);self.assertGreater(s['sd'],0);self.assertIsNone(s['p99'])
        self.assertEqual(stats([1,2,3,4])['median'],2.5)
    def test_job_includes_exited_child_cpu_and_does_not_own_harness(self):
        with tempfile.TemporaryFile() as log:
            child='import time; end=time.perf_counter()+.25\nwhile time.perf_counter()<end: pass'
            parent='import subprocess,sys,time; subprocess.run([sys.executable,"-c",'+repr(child)+']);time.sleep(.3)'
            tree=Tree([sys.executable,'-c',parent],os.environ.copy(),log)
            try:
                tree.process.wait(timeout=10);s=tree.snapshot()
                self.assertGreaterEqual(s['total_processes'],2)
                self.assertGreater(s['cpu_s'],.1)
                self.assertAlmostEqual(s['cpu_s'],s['user_s']+s['kernel_s'])
                self.assertTrue(any(x.get('rss',0)>0 for x in tree.samples))
            finally:tree.close()

if __name__=='__main__':unittest.main()
