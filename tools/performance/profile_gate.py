"""Rebuild and hash-verify diagnostic replays of the unchanged workloads."""
import argparse
import datetime
import json
import os
from pathlib import Path
import subprocess
from fast_gate import ROOT, frozen, verify_binary


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--workloads', nargs='+', choices=list(frozen.WORKLOADS), default=['dom'])
    parser.add_argument('--iterations', type=int, default=5)
    profile = parser.add_mutually_exclusive_group()
    profile.add_argument('--native', action='store_true')
    profile.add_argument('--go-cpu', action='store_true')
    parser.add_argument('--detailed', action='store_true')
    parser.add_argument('--hosts', action='store_true', help='host timing without JS wrapper hooks or conversion timers')
    parser.add_argument('--density', action='store_true')
    parser.add_argument('--collect-v8', action='store_true', help='explicit density-only GC intervention')
    args = parser.parse_args()
    if args.hosts and args.detailed:
        parser.error('--hosts and --detailed are mutually exclusive')
    if args.collect_v8 and not args.density:
        parser.error('--collect-v8 requires --density')
    if not 1 <= args.iterations <= 100:
        parser.error('iterations must be 1..100')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    executable = output/'browser.test.exe'
    command = ['go', 'test', '-c', '-o', str(executable), './internal/browser']
    subprocess.run(command, cwd=ROOT, check=True)
    expected = frozen.digest(executable)
    fingerprint, files = frozen.harness_fingerprint()
    baseline = json.loads((ROOT/'benchmark/results/raw.json').read_text(encoding='utf-8'))
    if fingerprint != baseline['metadata']['harness_sha256']:
        raise RuntimeError('Frozen harness changed')
    receipt = dict(command=command, executable=str(executable), sha256=expected,
                   revision=subprocess.check_output(['git','rev-parse','HEAD'], cwd=ROOT, text=True).strip(),
                   status=subprocess.check_output(['git','status','--porcelain'], cwd=ROOT, text=True),
                   harness_sha256=fingerprint, harness_files=files)
    (output/'build.json').write_text(json.dumps(receipt, indent=2), encoding='utf-8')
    for work in (['density'] if args.density else args.workloads):
        directory = output/work
        directory.mkdir()
        env = os.environ.copy()
        # Do not inherit any diagnostic interventions from the caller.
        for key in list(env):
            if key.startswith('MIMIC_PROFILE_') or key in ('MIMIC_DIAGNOSTICS','MIMIC_V8_CPU_PROFILE','MIMIC_V8_HEAP_SNAPSHOT'):
                env.pop(key)
        env.update(MIMIC_PROFILE_DIR=str(directory), MIMIC_PROFILE_WORKLOAD=work,
                   MIMIC_PROFILE_ITERATIONS=str(args.iterations), MIMIC_V8_CPU_PROFILE='0',
                   MIMIC_PROFILE_WHOLE_CPU=str(int(args.native)),
                   MIMIC_PROFILE_EXEC_GO_CPU=str(int(args.go_cpu)),
                   MIMIC_DIAGNOSTICS=str(int(args.detailed)),
                   MIMIC_PROFILE_HOSTS=str(int(args.hosts)),
                   MIMIC_PROFILE_CONVERSIONS=str(int(args.detailed)),
                   MIMIC_PROFILE_V8_GC=str(int(args.collect_v8)))
        test = 'TestPerformanceDensityProfile' if args.density else 'TestPerformanceProfile'
        launch = [str(executable), '-test.run=^'+test+'$', '-test.v', '-test.timeout=180s',
                  '-test.memprofile='+str(directory/'alloc.pprof'),
                  '-test.mutexprofile='+str(directory/'mutex.pprof'),
                  '-test.blockprofile='+str(directory/'block.pprof')]
        actual = verify_binary(executable, expected)
        if frozen.harness_fingerprint()[0] != fingerprint:
            raise RuntimeError('Frozen harness changed before launch')
        with (output/'launches.jsonl').open('a', encoding='utf-8') as log:
            log.write(json.dumps(dict(command=launch, sha256=actual, workload=work,
                verified_at=datetime.datetime.now().astimezone().isoformat()))+'\n')
        with (directory/'test.log').open('w', encoding='utf-8') as log:
            subprocess.run(launch, cwd=ROOT/'internal/browser', env=env,
                           stdout=log, stderr=subprocess.STDOUT, check=True)
        print(work, 'PASS', flush=True)


if __name__ == '__main__':
    main()
