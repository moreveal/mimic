"""Run a complete, disjoint shard of the Windows reference test suite."""
import argparse
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[2]
BROWSER = 'github.com/moreveal/mimic/internal/browser'


def run(*args, capture=False):
    return subprocess.run(args, cwd=ROOT, check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


def partition(names, shard, count):
    if count < 1 or not 0 <= shard < count or len(names) != len(set(names)):
        raise ValueError('Invalid shard parameters or duplicate test names')
    return sorted(names)[shard::count]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--shard', type=int, required=True)
    parser.add_argument('--count', type=int, default=2)
    args = parser.parse_args()
    listed = run('go', 'test', '-list', '.', BROWSER, capture=True)
    names = [line for line in listed.splitlines() if re.fullmatch(r'(Test|Example|Fuzz)\w*', line)]
    if not names:
        raise RuntimeError('Browser test discovery returned no tests')
    selected = partition(names, args.shard, args.count)
    if not selected:
        raise RuntimeError('Empty browser test shard')
    output = ROOT / '.build/ci'
    output.mkdir(parents=True, exist_ok=True)
    plan = {'shard': args.shard, 'count': args.count, 'total': len(names), 'selected': selected}
    (output / f'windows-tests-{args.shard}.json').write_text(json.dumps(plan, indent=2) + '\n')
    print(f'Windows browser shard {args.shard + 1}/{args.count}: '
          f'{len(selected)} of {len(names)} discovered root tests, all their subtests included', flush=True)
    # Bound command-line length on Windows and avoid one aggregate package timer
    # covering the entire suite. Individual assertion/context deadlines are unchanged.
    for offset in range(0, len(selected), 100):
        batch = selected[offset:offset + 100]
        expression = '^(' + '|'.join(re.escape(name) for name in batch) + ')$'
        print(f'Browser batch {offset // 100 + 1}: {len(batch)} root tests', flush=True)
        run('go', 'test', '-timeout', '10m', '-run', expression, BROWSER)
    if args.shard == 0:
        packages = [line for line in run('go', 'list', './...', capture=True).splitlines()
                    if line != BROWSER]
        print(f'Windows remaining packages: {len(packages)}', flush=True)
        # Behavioral deadlines must not compete with unrelated package builds
        # and engine initializations on the small hosted runner.
        run('go', 'test', '-p', '1', *packages)


if __name__ == '__main__':
    main()
