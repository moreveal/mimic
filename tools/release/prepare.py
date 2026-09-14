"""Build and verify one native release archive; never publish private source."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import tarfile
import tempfile
import zipfile
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[2]


def run(*args, cwd=ROOT, capture=False):
    return subprocess.run(args, cwd=cwd, check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def clean_revision(path):
    if run('git', 'status', '--porcelain', cwd=path, capture=True).strip():
        raise RuntimeError(f'Commit changes before preparing a release: {path}')
    return run('git', 'rev-parse', 'HEAD', cwd=path, capture=True).strip()


def json_stream(text):
    decoder = json.JSONDecoder()
    pos = 0
    while pos < len(text):
        if text[pos].isspace():
            pos += 1
            continue
        value, pos = decoder.raw_decode(text, pos)
        yield value


def validate_document_links(stage):
    for document in stage.rglob('*.md'):
        for link in re.findall(r'\]\(([^)]+)\)', document.read_text(encoding='utf-8-sig')):
            target = urlsplit(link)
            if target.scheme or target.netloc or not target.path:
                continue
            path = (document.parent / unquote(target.path)).resolve()
            if not path.is_relative_to(stage.resolve()) or not path.exists():
                raise RuntimeError(f'Missing bundled document link: {document.name}: {link}')


def notices(target):
    """Use the actual native command dependency graph, including local replacements."""
    deps = list(json_stream(run('go', 'list', '-deps', '-json', './cmd/mimic', capture=True)))
    modules = {d['Module']['Path']: d['Module'] for d in deps if 'Module' in d}
    modules.pop('github.com/moreveal/mimic')
    chunks = ['# Third-party notices\n\nThird-party components retain the licenses below. '
              'PolyForm Shield applies to Mimic-owned material, not these components.\n']
    inventory = []
    def include(label, path):
        data = path.read_text(encoding='utf-8-sig')
        chunks.append(f'\n## {label}\n\n{data}\n')
    goroot = Path(run('go', 'env', 'GOROOT', capture=True).strip())
    include('Go toolchain and standard library', goroot / 'LICENSE')
    pattern = re.compile(r'(license|licence|copying|copyright|notice|patents)', re.I)
    for name, module in sorted(modules.items()):
        base = Path(module.get('Replace', module)['Dir'])
        files = sorted(p for p in base.rglob('*') if p.is_file()
                       and pattern.search(p.name)
                       and p.suffix.lower() in ('', '.txt', '.md', '.rst', '.bsd', '.mit')
                       and '.git' not in p.parts)
        # This Go net/http fork retains the Go Authors' BSD headers, but its
        # published module omits the referenced LICENSE file. Supply that text.
        if not files and name == 'github.com/bogdanfinn/fhttp':
            header = (base / 'transport.go').read_text(encoding='utf-8')[:250]
            if 'The Go Authors' not in header or 'BSD-style' not in header:
                raise RuntimeError('Re-audit the fhttp license provenance')
            include(f'{name} — Go Authors BSD license referenced by source headers', goroot / 'LICENSE')
        elif not files:
            raise RuntimeError(f'Missing dependency license: {name}')
        version = module.get('Version', 'local')
        inventory.append({'module': name, 'version': version,
                          'modified': 'Replace' in module})
        for path in files:
            include(f'{name} {version} — {path.relative_to(base).as_posix()}', path)
    extras = {
        'HTML tokenizer': ROOT / 'internal/htmlstream/LICENSE',
        'Web Streams polyfill': ROOT / 'internal/webapi/vendor/web-streams-polyfill/LICENSE',
        'CSS selector dependencies': ROOT / 'internal/webapi/vendor/css-select/THIRD_PARTY_LICENSES.txt',
        'RustFFT': ROOT / 'internal/webapi/vendor/rustfft/NOTICE',
    }
    for name, path in extras.items():
        include(name, path)
    include('Pinned V8 native dependencies', ROOT / 'tools/release/licenses/native-notices.txt')
    if platform.system() == 'Windows':
        include('GCC runtime library terms and exception', ROOT / 'tools/release/licenses/gcc-runtime.txt')
    target.write_text('\n'.join(chunks), encoding='utf-8', newline='\n')
    return inventory


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--overview', type=Path, required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'v\d+\.\d+\.\d+(?:-beta\.\d+)?', args.version):
        parser.error('Expected vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-beta.NUMBER')
    host = platform.system().lower()
    if host not in ('windows', 'linux') or platform.machine().lower() not in ('amd64', 'x86_64'):
        parser.error('Only native Windows/Linux amd64 builds are packaged')
    overview = args.overview.resolve()
    source_revision = clean_revision(ROOT)
    overview_revision = clean_revision(overview)
    if (ROOT / 'LICENSE.md').read_bytes() != (overview / 'LICENSE.md').read_bytes():
        raise RuntimeError('Private/public licenses differ')
    output = ROOT / '.build/releases' / args.version
    stage = output / f'mimic-{args.version}-{host}-amd64'
    stage.mkdir(parents=True, exist_ok=False)
    binary = stage / ('mimic.exe' if host == 'windows' else 'mimic')
    env = os.environ.copy()
    env['CGO_ENABLED'] = '1'
    subprocess.run(['go', 'build', '-trimpath', '-ldflags=-s -w', '-o', str(binary), './cmd/mimic'],
                   cwd=ROOT, env=env, check=True)
    for name in ('LICENSE.md', 'NOTICE', 'QUICKSTART.md', 'BETA.md',
                 'FAQ.md', 'BENCHMARKS.md', 'RELEASE_NOTES.md'):
        shutil.copyfile(overview / name, stage / name)
    (stage / 'benchmarks').mkdir()
    shutil.copyfile(overview / 'benchmarks/results.json', stage / 'benchmarks/results.json')
    for name in run('git', 'ls-files', '-z', '--', 'examples', cwd=overview, capture=True).split('\0'):
        if not name:
            continue
        source = overview / name
        if source.is_symlink() or (source.suffix not in ('.md', '.mjs', '.json') and source.name != 'LICENSE'):
            raise RuntimeError(f'Unexpected public example asset: {name}')
        dest = stage / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, dest)
    inventory = notices(stage / 'THIRD_PARTY_NOTICES.txt')
    (stage / 'README.txt').write_text(
        f'Mimic {args.version} Public Beta\n\n'
        'Start here: QUICKSTART.md. Runnable examples: examples/README.md.\n'
        'License: LICENSE.md (PolyForm Shield 1.0.0). Third-party terms: THIRD_PARTY_NOTICES.txt.\n'
        'Downloads and documentation: https://github.com/moreveal/mimic-overview\n'
        'Windows amd64 or Linux amd64 (glibc 2.39+, libgcc_s, installed fonts).\n'
        'No Go, Rust, Chromium, display server, or GPU is needed to run Mimic.\n'
        'Node.js 22+ is only needed for the example clients.\n', encoding='utf-8')
    (stage / 'README.md').write_text(
        '# Mimic Public Beta\n\n[Start here](QUICKSTART.md) · [Examples](examples/README.md) · '
        '[Release notes](RELEASE_NOTES.md) · [License](LICENSE.md)\n\n'
        'Product overview: https://github.com/moreveal/mimic-overview\n', encoding='utf-8')
    validate_document_links(stage)
    stamp = int(run('git', 'show', '-s', '--format=%ct', source_revision, capture=True).strip())
    files = sorted(p for p in stage.rglob('*') if p.is_file())
    if host == 'windows':
        import time
        archive = Path(str(stage) + '.zip')
        with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as z:
            for path in files:
                info = zipfile.ZipInfo(path.relative_to(stage.parent).as_posix(), time.gmtime(stamp)[:6])
                info.compress_type = zipfile.ZIP_DEFLATED
                z.writestr(info, path.read_bytes())
    else:
        import gzip
        archive = Path(str(stage) + '.tar.gz')
        with archive.open('wb') as raw, gzip.GzipFile(fileobj=raw, mode='wb', mtime=stamp, filename='') as gz:
            with tarfile.open(fileobj=gz, mode='w') as tar:
                for path in files:
                    info = tar.gettarinfo(str(path), path.relative_to(stage.parent).as_posix())
                    info.uid = info.gid = 0
                    info.uname = info.gname = ''
                    info.mtime = stamp
                    info.mode = 0o755 if path == binary else 0o644
                    with path.open('rb') as src:
                        tar.addfile(info, src)
    # Test what users actually unpack, including the packaged public examples.
    with tempfile.TemporaryDirectory(prefix='mimic-release-') as temp:
        extracted = Path(temp)
        if host == 'windows':
            with zipfile.ZipFile(archive) as z:
                z.extractall(extracted)
        else:
            with tarfile.open(archive) as tar:
                tar.extractall(extracted, filter='data')
        ready = extracted / stage.name / binary.name
        for engine in ('v8', 'quickjs', 'goja'):
            run('go', 'run', './tools/runtimecheck', '-binary', str(ready), '-engine', engine)
        examples = ready.parent / 'examples'
        npm = shutil.which('npm.cmd' if host == 'windows' else 'npm')
        run(npm, 'ci', '--ignore-scripts', cwd=examples)
        run('node', 'verify.mjs', str(ready), cwd=examples)
    if clean_revision(ROOT) != source_revision or clean_revision(overview) != overview_revision:
        raise RuntimeError('Checkout changed during preparation')
    receipt = {
        'version': args.version, 'platform': f'{host}-amd64',
        'sourceRevision': source_revision, 'overviewRevision': overview_revision,
        'goVersion': run('go', 'version', capture=True).strip(),
        'archive': archive.name, 'sha256': digest(archive), 'size': archive.stat().st_size,
        'binarySha256': digest(binary), 'dependencies': inventory,
        'checks': ['runtimecheck:v8', 'runtimecheck:quickjs', 'runtimecheck:goja',
                   'examples:puppeteer', 'examples:playwright', 'examples:concurrency'],
        'verified': True,
    }
    (output / f'{host}-amd64.receipt.json').write_text(json.dumps(receipt, indent=2) + '\n', encoding='utf-8')
    print(f'VERIFIED {archive.name}: {receipt["sha256"]}')


if __name__ == '__main__':
    main()
