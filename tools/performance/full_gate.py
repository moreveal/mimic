"""Milestone-only full frozen matrix with per-launch executable verification."""
import argparse
import datetime
import json
from pathlib import Path
import subprocess
import sys
from fast_gate import ROOT, frozen, verify_binary


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--chrome',type=Path,required=True)
    args=parser.parse_args()
    output=args.output.resolve();chrome=args.chrome.resolve()
    output.mkdir(parents=True,exist_ok=False)
    executable=ROOT/'.build'/('milestone-'+output.name+'.exe')
    command=['go','build','-o',str(executable),'./cmd/mimic']
    subprocess.run(command,cwd=ROOT,check=True)
    expected={'mimic':frozen.digest(executable),'chrome':frozen.digest(chrome)}
    paths={'mimic':executable,'chrome':chrome}
    fingerprint,files=frozen.harness_fingerprint()
    baseline=json.loads((ROOT/'benchmark/results/raw.json').read_text(encoding='utf-8'))
    if fingerprint!=baseline['metadata']['harness_sha256']:
        raise RuntimeError('Frozen harness differs from immutable baseline')
    receipt=dict(command=command,revision=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),
        status=subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True),
        sha256=expected,executables={k:str(v) for k,v in paths.items()},harness_sha256=fingerprint,
        harness_files=files,built_at=datetime.datetime.now().astimezone().isoformat())
    (output/'build.json').write_text(json.dumps(receipt,indent=2),encoding='utf-8')
    original=frozen.Runtime

    class VerifiedRuntime(original):
        def __init__(self,system,runargs):
            path=getattr(runargs,system).resolve()
            if path!=paths[system]:raise RuntimeError('Unexpected executable launch path')
            actual=verify_binary(path,expected[system])
            if frozen.harness_fingerprint()[0]!=fingerprint:
                raise RuntimeError('Frozen harness changed before launch')
            with (output/'launches.jsonl').open('a',encoding='utf-8') as log:
                log.write(json.dumps(dict(system=system,executable=str(path),sha256=actual,
                    verified_at=datetime.datetime.now().astimezone().isoformat()))+'\n')
            super().__init__(system,runargs)

    frozen.Runtime=VerifiedRuntime
    sys.argv=[str(ROOT/'benchmark/run.py'),'--skip-build','--mimic',str(executable),
        '--chrome',str(chrome),'--output',str(output)]
    try:frozen.main()
    finally:frozen.Runtime=original
    subprocess.run([sys.executable,str(ROOT/'tools/performance/readme_charts.py'),
        str(output/'raw.json'),str(output/'presentation'),
        '--receipt',str(output/'presentation/provenance.json')],check=True)

if __name__=='__main__':main()
